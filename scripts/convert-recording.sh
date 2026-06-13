#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RECORDINGS_DIR="${RECORDINGS_DIR:-$ROOT_DIR/recordings}"
FPS="${FPS:-10}"
FONT_SIZE="${FONT_SIZE:-16}"
CELL_WIDTH="${CELL_WIDTH:-10}"
CELL_HEIGHT="${CELL_HEIGHT:-20}"
PADDING="${PADDING:-12}"
HOLD_SECONDS="${HOLD_SECONDS:-1.5}"
KEEP_FRAMES="${KEEP_FRAMES:-0}"
FRAME_DIR=""

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf '%s\n' \
    'Convert a util-linux script recording to MP4 without replaying it.' \
    '' \
    'Usage: scripts/convert-recording.sh [recording.typescript] [output.mp4]' \
    '' \
    'Defaults:' \
    '  input:  newest recordings/*.typescript' \
    '  output: same basename with .mp4' \
    '' \
    'Environment overrides:' \
    '  FPS=10 FONT_SIZE=16 CELL_WIDTH=10 CELL_HEIGHT=20 PADDING=12' \
    '  HOLD_SECONDS=1.5 KEEP_FRAMES=1'
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

latest_typescript() {
  local latest=""
  local candidate

  shopt -s nullglob
  for candidate in "$RECORDINGS_DIR"/*.typescript; do
    if [[ -z "$latest" || "$candidate" -nt "$latest" ]]; then
      latest="$candidate"
    fi
  done
  shopt -u nullglob
  [[ -n "$latest" ]] || die "no .typescript recordings found in $RECORDINGS_DIR"
  printf '%s\n' "$latest"
}

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi

  need_cmd python3
  need_cmd ffmpeg

  local input="${1:-}"
  if [[ -z "$input" ]]; then
    input="$(latest_typescript)"
  fi
  [[ -f "$input" ]] || die "missing input: $input"

  local timing="${input%.typescript}.timing"
  [[ -f "$timing" ]] || die "missing timing file: $timing"

  local output="${2:-${input%.typescript}.mp4}"
  FRAME_DIR="$(mktemp -d "${TMPDIR:-/tmp}/kli-recording-frames.XXXXXX")"
  trap 'if [[ -n "${FRAME_DIR:-}" ]]; then if [[ "${KEEP_FRAMES}" != "1" ]]; then rm -rf -- "${FRAME_DIR}"; else printf "frames kept: %s\n" "${FRAME_DIR}"; fi; fi' EXIT

  printf 'rendering frames from %s\n' "$input"
  python3 - "$input" "$timing" "$FRAME_DIR" "$FPS" "$FONT_SIZE" "$CELL_WIDTH" "$CELL_HEIGHT" "$PADDING" "$HOLD_SECONDS" <<'PY'
import codecs
import html
import math
import re
import sys
import unicodedata
from dataclasses import dataclass
from pathlib import Path

typescript = Path(sys.argv[1])
timing = Path(sys.argv[2])
frame_dir = Path(sys.argv[3])
fps = float(sys.argv[4])
font_size = int(sys.argv[5])
cell_w = int(sys.argv[6])
cell_h = int(sys.argv[7])
pad = int(sys.argv[8])
hold = float(sys.argv[9])

DEFAULT_FG = "#d7dae0"
DEFAULT_BG = "#0b1020"
FONT_FAMILY = "'JetBrainsMono Nerd Font', 'JetBrains Mono', monospace"


def parse_size(header: bytes) -> tuple[int, int]:
    text = header.decode("utf-8", "replace")
    cols = re.search(r"COLUMNS=\"?(\d+)", text)
    lines = re.search(r"LINES=\"?(\d+)", text)
    return (int(cols.group(1)) if cols else 120, int(lines.group(1)) if lines else 36)


raw = typescript.read_bytes()
header = b""
if raw.startswith(b"Script started"):
    first_nl = raw.find(b"\n")
    if first_nl != -1:
        header = raw[: first_nl + 1]
        raw = raw[first_nl + 1 :]
footer_at = raw.rfind(b"\nScript done on ")
if footer_at != -1:
    raw = raw[:footer_at]

cols, rows = parse_size(header)
events: list[tuple[float, int]] = []
total_bytes = 0
for line in timing.read_text(encoding="utf-8").splitlines():
    if not line.strip():
        continue
    parts = line.split()
    if len(parts) < 2:
        continue
    delay = float(parts[0])
    count = int(parts[1])
    events.append((delay, count))
    total_bytes += count

if len(raw) < total_bytes:
    raise SystemExit(f"timing references {total_bytes} bytes, but recording has {len(raw)} bytes")
if len(raw) > total_bytes:
    raw = raw[:total_bytes]


BASE16 = [
    "#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0",
    "#808080", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff",
]


def xterm256(n: int) -> str:
    n = max(0, min(255, n))
    if n < 16:
        return BASE16[n]
    if n < 232:
        n -= 16
        r, rem = divmod(n, 36)
        g, b = divmod(rem, 6)
        vals = [0, 95, 135, 175, 215, 255]
        return f"#{vals[r]:02x}{vals[g]:02x}{vals[b]:02x}"
    v = 8 + (n - 232) * 10
    return f"#{v:02x}{v:02x}{v:02x}"


def sgr_color(code: int, bright: bool = False) -> str:
    if 30 <= code <= 37:
        return xterm256(code - 30 + (8 if bright else 0))
    if 90 <= code <= 97:
        return xterm256(code - 90 + 8)
    if 40 <= code <= 47:
        return xterm256(code - 40 + (8 if bright else 0))
    if 100 <= code <= 107:
        return xterm256(code - 100 + 8)
    return DEFAULT_FG


def cell_width(ch: str) -> int:
    if unicodedata.combining(ch):
        return 0
    if unicodedata.east_asian_width(ch) in {"F", "W"}:
        return 2
    return 1


@dataclass(frozen=True)
class Attr:
    fg: str = DEFAULT_FG
    bg: str = DEFAULT_BG
    bold: bool = False
    italic: bool = False
    dim: bool = False


@dataclass
class Cell:
    ch: str = " "
    attr: Attr = Attr()


class Terminal:
    def __init__(self, width: int, height: int):
        self.width = width
        self.height = height
        self.cells = [[Cell() for _ in range(width)] for _ in range(height)]
        self.row = 0
        self.col = 0
        self.saved = (0, 0)
        self.attr = Attr()
        self.scroll_top = 0
        self.scroll_bottom = height - 1
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.state = "normal"
        self.esc = bytearray()
        self.osc = bytearray()
        self.charset = False

    def blank_cell(self) -> Cell:
        return Cell(" ", self.attr)

    def clear_line(self, row: int, start: int = 0, end: int | None = None) -> None:
        if not 0 <= row < self.height:
            return
        if end is None:
            end = self.width - 1
        start = max(0, min(self.width - 1, start))
        end = max(0, min(self.width - 1, end))
        for c in range(start, end + 1):
            self.cells[row][c] = self.blank_cell()

    def clear_screen(self) -> None:
        for r in range(self.height):
            self.clear_line(r)

    def scroll_up(self, count: int = 1) -> None:
        for _ in range(max(1, count)):
            del self.cells[self.scroll_top]
            self.cells.insert(self.scroll_bottom, [self.blank_cell() for _ in range(self.width)])

    def scroll_down(self, count: int = 1) -> None:
        for _ in range(max(1, count)):
            del self.cells[self.scroll_bottom]
            self.cells.insert(self.scroll_top, [self.blank_cell() for _ in range(self.width)])

    def newline(self) -> None:
        if self.row == self.scroll_bottom:
            self.scroll_up(1)
        else:
            self.row = min(self.height - 1, self.row + 1)

    def put_char(self, ch: str) -> None:
        if ch == "\r":
            self.col = 0
            return
        if ch == "\n":
            self.newline()
            return
        if ch == "\b":
            self.col = max(0, self.col - 1)
            return
        if ch == "\t":
            for _ in range(8 - (self.col % 8)):
                self.put_char(" ")
            return
        if ord(ch) < 32 or ord(ch) == 127:
            return

        w = cell_width(ch)
        if w == 0:
            if self.col > 0:
                self.cells[self.row][self.col - 1].ch += ch
            return
        if self.col >= self.width:
            self.col = 0
            self.newline()
        if w == 2 and self.col == self.width - 1:
            self.col = 0
            self.newline()
        self.cells[self.row][self.col] = Cell(ch, self.attr)
        if w == 2 and self.col + 1 < self.width:
            self.cells[self.row][self.col + 1] = Cell("", self.attr)
        self.col += w

    def text(self, data: bytes) -> None:
        out = self.decoder.decode(data, final=False)
        for ch in out:
            self.put_char(ch)

    def feed(self, data: bytes) -> None:
        for b in data:
            if self.state == "normal":
                if b == 0x1B:
                    self.text(b"")
                    self.state = "esc"
                    self.esc.clear()
                else:
                    self.text(bytes([b]))
            elif self.state == "esc":
                if b == ord("["):
                    self.state = "csi"
                    self.esc.clear()
                elif b == ord("]"):
                    self.state = "osc"
                    self.osc.clear()
                elif b in (ord("("), ord(")"), ord("*"), ord("+")):
                    self.state = "charset"
                elif b == ord("7"):
                    self.saved = (self.row, self.col)
                    self.state = "normal"
                elif b == ord("8"):
                    self.row, self.col = self.saved
                    self.state = "normal"
                elif b == ord("c"):
                    self.__init__(self.width, self.height)
                elif b in (ord("="), ord(">")):
                    self.state = "normal"
                elif b == ord("M"):
                    if self.row == self.scroll_top:
                        self.scroll_down(1)
                    else:
                        self.row = max(0, self.row - 1)
                    self.state = "normal"
                else:
                    self.state = "normal"
            elif self.state == "charset":
                self.state = "normal"
            elif self.state == "osc":
                if b == 0x07:
                    self.state = "normal"
                elif b == 0x1B:
                    self.state = "osc_esc"
                else:
                    self.osc.append(b)
            elif self.state == "osc_esc":
                if b == ord("\\"):
                    self.state = "normal"
                else:
                    self.state = "osc"
            elif self.state == "csi":
                self.esc.append(b)
                if 0x40 <= b <= 0x7E:
                    self.handle_csi(bytes(self.esc))
                    self.state = "normal"

    def params(self, body: str, default: int = 1) -> list[int]:
        body = body.lstrip("?=>")
        if body == "":
            return [default]
        vals = []
        for part in body.split(";"):
            if part == "":
                vals.append(default)
                continue
            part = part.split(":", 1)[0]
            try:
                vals.append(int(part))
            except ValueError:
                vals.append(default)
        return vals

    def handle_csi(self, seq: bytes) -> None:
        final = chr(seq[-1])
        body = seq[:-1].decode("ascii", "ignore")
        private = body.startswith("?")
        p = self.params(body)

        if final in "Hf":
            r = p[0] if len(p) >= 1 else 1
            c = p[1] if len(p) >= 2 else 1
            self.row = max(0, min(self.height - 1, r - 1))
            self.col = max(0, min(self.width - 1, c - 1))
        elif final == "A":
            self.row = max(0, self.row - p[0])
        elif final == "B":
            self.row = min(self.height - 1, self.row + p[0])
        elif final == "C":
            self.col = min(self.width - 1, self.col + p[0])
        elif final == "D":
            self.col = max(0, self.col - p[0])
        elif final == "G":
            self.col = max(0, min(self.width - 1, p[0] - 1))
        elif final == "d":
            self.row = max(0, min(self.height - 1, p[0] - 1))
        elif final == "J":
            mode = p[0] if p else 0
            if mode in (2, 3):
                self.clear_screen()
            elif mode == 0:
                self.clear_line(self.row, self.col, self.width - 1)
                for r in range(self.row + 1, self.height):
                    self.clear_line(r)
            elif mode == 1:
                for r in range(0, self.row):
                    self.clear_line(r)
                self.clear_line(self.row, 0, self.col)
        elif final == "K":
            mode = p[0] if p else 0
            if mode == 0:
                self.clear_line(self.row, self.col, self.width - 1)
            elif mode == 1:
                self.clear_line(self.row, 0, self.col)
            elif mode == 2:
                self.clear_line(self.row)
        elif final == "X":
            self.clear_line(self.row, self.col, min(self.width - 1, self.col + p[0] - 1))
        elif final == "@":
            n = min(p[0], self.width - self.col)
            line = self.cells[self.row]
            for _ in range(n):
                line.insert(self.col, self.blank_cell())
                line.pop()
        elif final == "P":
            n = min(p[0], self.width - self.col)
            line = self.cells[self.row]
            for _ in range(n):
                del line[self.col]
                line.append(self.blank_cell())
        elif final == "L":
            for _ in range(p[0]):
                self.cells.insert(self.row, [self.blank_cell() for _ in range(self.width)])
                del self.cells[self.scroll_bottom + 1]
        elif final == "M":
            for _ in range(p[0]):
                del self.cells[self.row]
                self.cells.insert(self.scroll_bottom, [self.blank_cell() for _ in range(self.width)])
        elif final == "S":
            self.scroll_up(p[0])
        elif final == "T":
            self.scroll_down(p[0])
        elif final == "r" and not private:
            top = (p[0] if len(p) >= 1 else 1) - 1
            bottom = (p[1] if len(p) >= 2 else self.height) - 1
            if 0 <= top < bottom < self.height:
                self.scroll_top, self.scroll_bottom = top, bottom
                self.row, self.col = 0, 0
        elif final == "s":
            self.saved = (self.row, self.col)
        elif final == "u":
            self.row, self.col = self.saved
        elif final == "m":
            self.handle_sgr(body)
        elif final in "hlcnq":
            pass

    def handle_sgr(self, body: str) -> None:
        raw = body.lstrip("?=>")
        codes = [0] if raw == "" else []
        for part in raw.split(";"):
            if part == "":
                codes.append(0)
            else:
                try:
                    codes.append(int(part.split(":", 1)[0]))
                except ValueError:
                    codes.append(0)
        i = 0
        attr = self.attr
        while i < len(codes):
            code = codes[i]
            if code == 0:
                attr = Attr()
            elif code == 1:
                attr = Attr(attr.fg, attr.bg, True, attr.italic, attr.dim)
            elif code == 2:
                attr = Attr(attr.fg, attr.bg, attr.bold, attr.italic, True)
            elif code == 3:
                attr = Attr(attr.fg, attr.bg, attr.bold, True, attr.dim)
            elif code in (22, 21):
                attr = Attr(attr.fg, attr.bg, False, attr.italic, False)
            elif code == 23:
                attr = Attr(attr.fg, attr.bg, attr.bold, False, attr.dim)
            elif code == 39:
                attr = Attr(DEFAULT_FG, attr.bg, attr.bold, attr.italic, attr.dim)
            elif code == 49:
                attr = Attr(attr.fg, DEFAULT_BG, attr.bold, attr.italic, attr.dim)
            elif 30 <= code <= 37 or 90 <= code <= 97:
                attr = Attr(sgr_color(code), attr.bg, attr.bold, attr.italic, attr.dim)
            elif 40 <= code <= 47 or 100 <= code <= 107:
                attr = Attr(attr.fg, sgr_color(code), attr.bold, attr.italic, attr.dim)
            elif code in (38, 48) and i + 2 < len(codes) and codes[i + 1] == 5:
                color = xterm256(codes[i + 2])
                if code == 38:
                    attr = Attr(color, attr.bg, attr.bold, attr.italic, attr.dim)
                else:
                    attr = Attr(attr.fg, color, attr.bold, attr.italic, attr.dim)
                i += 2
            elif code in (38, 48) and i + 4 < len(codes) and codes[i + 1] == 2:
                color = f"#{codes[i + 2] & 255:02x}{codes[i + 3] & 255:02x}{codes[i + 4] & 255:02x}"
                if code == 38:
                    attr = Attr(color, attr.bg, attr.bold, attr.italic, attr.dim)
                else:
                    attr = Attr(attr.fg, color, attr.bold, attr.italic, attr.dim)
                i += 4
            i += 1
        self.attr = attr


term = Terminal(cols, rows)
frame_dir.mkdir(parents=True, exist_ok=True)
frame = 0
next_frame_t = 0.0
now = 0.0
interval = 1.0 / fps
cursor = 0
svg_w = cols * cell_w + pad * 2
svg_h = rows * cell_h + pad * 2


def write_frame() -> None:
    global frame
    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{svg_w}" height="{svg_h}" viewBox="0 0 {svg_w} {svg_h}">',
        f'<rect width="100%" height="100%" fill="{DEFAULT_BG}"/>',
        f'<style>text{{font-family:{FONT_FAMILY};font-size:{font_size}px;dominant-baseline:text-before-edge;white-space:pre}}</style>',
    ]
    for r, line in enumerate(term.cells):
        c = 0
        while c < cols:
            bg = line[c].attr.bg
            start = c
            while c < cols and line[c].attr.bg == bg:
                c += 1
            if bg != DEFAULT_BG:
                parts.append(f'<rect x="{pad + start * cell_w}" y="{pad + r * cell_h}" width="{(c - start) * cell_w}" height="{cell_h}" fill="{bg}"/>')
    for r, line in enumerate(term.cells):
        c = 0
        while c < cols:
            while c < cols and line[c].ch in ("", " "):
                c += 1
            if c >= cols:
                break
            start = c
            attr = line[c].attr
            text = []
            while c < cols and line[c].ch not in ("", " ") and line[c].attr == attr:
                text.append(line[c].ch)
                c += 1
            weight = ' font-weight="700"' if attr.bold else ""
            style = ' font-style="italic"' if attr.italic else ""
            opacity = ' fill-opacity="0.65"' if attr.dim else ""
            parts.append(
                f'<text x="{pad + start * cell_w}" y="{pad + r * cell_h}" fill="{attr.fg}"{weight}{style}{opacity}>{html.escape("".join(text))}</text>'
            )
    parts.append("</svg>\n")
    (frame_dir / f"frame-{frame:05d}.svg").write_text("\n".join(parts), encoding="utf-8")
    frame += 1


for delay, count in events:
    end = now + delay
    while next_frame_t <= end + 1e-9:
        write_frame()
        next_frame_t += interval
    chunk = raw[cursor : cursor + count]
    cursor += count
    term.feed(chunk)
    now = end

end = now + hold
while next_frame_t <= end + 1e-9:
    write_frame()
    next_frame_t += interval

print(f"frames={frame} size={svg_w}x{svg_h} duration={end:.2f}s")
PY

  printf 'encoding %s\n' "$output"
  ffmpeg -hide_banner -y \
    -framerate "$FPS" \
    -i "$FRAME_DIR/frame-%05d.svg" \
    -vf 'format=yuv420p' \
    -c:v libx264 \
    -preset medium \
    -crf 20 \
    -movflags +faststart \
    "$output"

  printf 'wrote: %s\n' "$output"
}

main "$@"

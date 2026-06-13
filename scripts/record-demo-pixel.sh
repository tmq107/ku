#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/recordings}"
STAMP="${STAMP:-$(date +%Y%m%d-%H%M%S)}"
OUTPUT_FILE="${OUTPUT_FILE:-$OUTPUT_DIR/kli-demo-env-$STAMP.mp4}"
GEOMETRY="${GEOMETRY:-}"
FPS="${FPS:-30}"
RECORD_START_DELAY="${RECORD_START_DELAY:-0.75}"
RECORDER_PID=""

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf '%s\n' \
    'Record the visible KLI demo as real pixels.' \
    '' \
    'Usage: scripts/record-demo-pixel.sh' \
    '' \
    'This captures your actual terminal environment: font, theme, palette,' \
    'background, compositor effects, and any window styling in the selected area.' \
    '' \
    'Environment overrides:' \
    '  OUTPUT_FILE=recordings/demo.mp4' \
    '  GEOMETRY="x,y WxH"             skip slurp and record this region' \
    '  FPS=30' \
    '  THEME=tokyonight               forwarded to record-demo.sh' \
    '  KLI_BIN=./kli                  forwarded to record-demo.sh'
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

stop_recorder() {
  if [[ -n "$RECORDER_PID" ]] && kill -0 "$RECORDER_PID" 2>/dev/null; then
    kill -INT "$RECORDER_PID" 2>/dev/null || true
    wait "$RECORDER_PID" 2>/dev/null || true
  fi
  RECORDER_PID=""
}

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi

  need_cmd wf-recorder
  need_cmd slurp

  mkdir -p -- "$OUTPUT_DIR"

  if [[ -z "$GEOMETRY" ]]; then
    printf 'Select the terminal area to capture. Include window decorations if you want them in the MP4.\n'
    GEOMETRY="$(slurp)"
  fi
  [[ -n "$GEOMETRY" ]] || die 'no capture region selected'

  trap stop_recorder EXIT

  printf 'recording %s at %s fps -> %s\n' "$GEOMETRY" "$FPS" "$OUTPUT_FILE"
  wf-recorder -y -r "$FPS" -g "$GEOMETRY" -f "$OUTPUT_FILE" &
  RECORDER_PID="$!"
  sleep "$RECORD_START_DELAY"
  if ! kill -0 "$RECORDER_PID" 2>/dev/null; then
    wait "$RECORDER_PID" 2>/dev/null || true
    RECORDER_PID=""
    die 'wf-recorder exited before the demo started'
  fi

  BACKEND=terminal RENDER_GIF=0 "$ROOT_DIR/scripts/record-demo.sh"
  stop_recorder

  printf 'wrote: %s\n' "$OUTPUT_FILE"
}

main "$@"

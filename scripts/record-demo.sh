#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/recordings}"
KUBECONFIG_ARG="${KUBECONFIG_ARG:-demos/kubeconfig}"
KLI_BIN="${KLI_BIN:-}"
THEME="${THEME:-}"
BACKEND="${BACKEND:-auto}"
RENDER_GIF="${RENDER_GIF:-1}"

WIDTH="${WIDTH:-120}"
HEIGHT="${HEIGHT:-36}"
START_DELAY="${START_DELAY:-1.5}"
STEP_DELAY="${STEP_DELAY:-1.0}"
COMMENT_DELAY="${COMMENT_DELAY:-1.8}"
TYPE_DELAY="${TYPE_DELAY:-0.035}"

STAMP="${STAMP:-$(date +%Y%m%d-%H%M%S)}"
CAST_FILE="${CAST_FILE:-$OUTPUT_DIR/kli-demo-$STAMP.cast}"
GIF_FILE="${GIF_FILE:-$OUTPUT_DIR/kli-demo-$STAMP.gif}"
TYPESCRIPT_FILE="${TYPESCRIPT_FILE:-$OUTPUT_DIR/kli-demo-$STAMP.typescript}"
TIMING_FILE="${TIMING_FILE:-$OUTPUT_DIR/kli-demo-$STAMP.timing}"
SESSION="${SESSION:-kli-demo-record-$$}"
TARGET=""
DRIVER_PID=""
DEMO_HOME=""

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf '%s\n' \
    'Record an automated KLI demo with tmux caption overlays.' \
    '' \
    'Usage: scripts/record-demo.sh' \
    '       scripts/record-demo.sh --stop' \
    '' \
    'Environment overrides:' \
    '  KLI_BIN=./kli                 binary to run, auto-detected by default' \
    '  KUBECONFIG_ARG=demos/kubeconfig' \
    '  BACKEND=auto|asciinema|script|terminal capture backend' \
    '  OUTPUT_DIR=recordings         where recordings are written' \
    '  THEME=tokyonight              optional extra --theme value' \
    '  RENDER_GIF=0                  skip agg GIF rendering' \
    '  WIDTH=120 HEIGHT=36           tmux window size hint' \
    '' \
    'Outputs:' \
    '  asciinema backend: .cast, plus .gif when agg is installed' \
    '  script backend: .typescript and .timing for scriptreplay' \
    '  terminal backend: no file, plays the demo in the current terminal'
}

stop_recorders() {
  local sessions
  local stopped=0
  local s

  command -v tmux >/dev/null 2>&1 || return 0
  sessions="$(tmux list-sessions -F '#S' 2>/dev/null || true)"
  for s in $sessions; do
    case "$s" in
      "$SESSION"|kli-demo-record-*)
        tmux kill-session -t "$s" 2>/dev/null || true
        printf 'stopped: %s\n' "$s"
        stopped=1
        ;;
    esac
  done
  if [[ "$stopped" == "0" ]]; then
    printf 'no kli demo recorder tmux sessions found\n'
  fi
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

shell_quote() {
  printf '%q' "$1"
}

cleanup() {
  if [[ -n "$DRIVER_PID" ]] && kill -0 "$DRIVER_PID" 2>/dev/null; then
    kill "$DRIVER_PID" 2>/dev/null || true
  fi
  tmux kill-session -t "$SESSION" 2>/dev/null || true
  if [[ -n "$DEMO_HOME" && -d "$DEMO_HOME" ]]; then
    rm -rf -- "$DEMO_HOME"
  fi
}

resolve_kli() {
  if [[ -n "$KLI_BIN" ]]; then
    return
  fi
  if [[ -x "$ROOT_DIR/kli" ]]; then
    KLI_BIN="./kli"
    return
  fi
  if command -v kli >/dev/null 2>&1; then
    KLI_BIN="kli"
    return
  fi
  if command -v go >/dev/null 2>&1; then
    printf 'building local kli binary...\n'
    (cd "$ROOT_DIR" && go build -o "$ROOT_DIR/kli" .)
    KLI_BIN="./kli"
    return
  fi
  die 'could not find kli; run make build or set KLI_BIN=/path/to/kli'
}

pick_backend() {
  case "$BACKEND" in
    auto)
      if command -v asciinema >/dev/null 2>&1; then
        BACKEND="asciinema"
      elif command -v script >/dev/null 2>&1; then
        BACKEND="script"
      else
        die 'install asciinema or util-linux script to record the demo'
      fi
      ;;
    asciinema)
      need_cmd asciinema
      ;;
    script)
      need_cmd script
      ;;
    terminal)
      ;;
    *)
      die 'BACKEND must be auto, asciinema, script, or terminal'
      ;;
  esac
}

launch_command() {
  local cmd

  cmd="HOME=$(shell_quote "$DEMO_HOME") XDG_CONFIG_HOME=$(shell_quote "$DEMO_HOME/.config") TERM=xterm-256color $(shell_quote "$KLI_BIN") --kubeconfig $(shell_quote "$KUBECONFIG_ARG")"
  if [[ -n "$THEME" ]]; then
    cmd+=" --theme $(shell_quote "$THEME")"
  fi
  printf '%s' "$cmd"
}

set_caption() {
  local text="$1"

  tmux set-option -t "$SESSION" status-left " #[bold]$text" >/dev/null
}

comment() {
  set_caption "$1"
  sleep "$COMMENT_DELAY"
}

press() {
  tmux send-keys -t "$TARGET" "$@"
  sleep "$STEP_DELAY"
}

type_text() {
  local text="$1"
  local char
  local i

  for ((i = 0; i < ${#text}; i++)); do
    char="${text:i:1}"
    tmux send-keys -t "$TARGET" -l "$char"
    sleep "$TYPE_DELAY"
  done
  sleep "$STEP_DELAY"
}

start_tmux() {
  TARGET="$(tmux new-session -d -P -F '#{pane_id}' -s "$SESSION" -x "$WIDTH" -y "$HEIGHT" -c "$ROOT_DIR")"
  tmux set-option -t "$SESSION" status on >/dev/null
  tmux set-option -t "$SESSION" status-position top >/dev/null
  tmux set-option -t "$SESSION" status-interval 1 >/dev/null
  tmux set-option -t "$SESSION" status-left-length 1000 >/dev/null
  tmux set-option -t "$SESSION" status-right '' >/dev/null
  tmux set-option -t "$SESSION" status-style 'bg=colour236,fg=colour255' >/dev/null
  tmux set-option -t "$SESSION" status-left-style 'bg=colour31,fg=colour231,bold' >/dev/null
  tmux set-option -t "$SESSION" pane-border-status off >/dev/null
  set_caption 'Preparing KLI demo recording...'
}

run_walkthrough() {
  local cmd

  sleep "$START_DELAY"
  cmd="$(launch_command)"
  comment "Launch: $KLI_BIN --kubeconfig $KUBECONFIG_ARG"
  type_text "$cmd"
  press Enter
  sleep 3

  comment 'Cockpit overview: cluster health, node resources, workloads, and warnings.'
  sleep 1

  comment 'Switch to the disposable kli-demo namespace from inside the UI.'
  press n
  sleep 1
  type_text 'kli-demo'
  press Enter
  sleep 2

  comment "Jump to Pods with ':' and fuzzy resource search."
  press ':'
  type_text 'pods'
  press Enter
  sleep 2

  comment "Toggle wide columns with 'w' to expose kubectl-style details."
  press w
  sleep 1

  comment "Filter with '/' to narrow the table to frontend pods."
  press '/'
  type_text 'frontend'
  press Enter
  sleep 1

  comment "Open YAML for the selected pod with 'y'."
  press y
  sleep 2

  comment 'Scroll through the YAML, then return to the table.'
  press C-d
  sleep 1
  press Escape
  sleep 1

  comment "Stream logs for the selected pod with 'l'."
  press l
  sleep 3

  comment "Logs stay inside the TUI. Toggle follow with 'f', Esc returns."
  press f
  sleep 1
  press f
  sleep 1
  press Escape
  sleep 1

  comment 'Use Ctrl-K as a command palette for actions and resources.'
  press C-k
  type_text 'deploy'
  press Enter
  sleep 2

  comment 'Deployments show workload status with the same table controls.'
  press G
  sleep 1
  press g
  sleep 1

  comment "Toggle all namespaces from any table with 'a'."
  press a
  sleep 2

  comment "Open built-in help with '?'."
  press '?'
  sleep 3
  press Escape
  sleep 1

  comment "Demo complete. Quit with 'q'."
  press q
  sleep 1
  tmux detach-client -s "$SESSION" 2>/dev/null || true
}

record_session() {
  local attach_cmd="tmux attach-session -t $SESSION"

  case "$BACKEND" in
    asciinema)
      rm -f -- "$CAST_FILE"
      asciinema rec -t 'kli demo' -c "$attach_cmd" "$CAST_FILE"
      ;;
    script)
      rm -f -- "$TYPESCRIPT_FILE" "$TIMING_FILE"
      script -q --flush --timing="$TIMING_FILE" --command "$attach_cmd" "$TYPESCRIPT_FILE"
      ;;
    terminal)
      tmux attach-session -t "$SESSION"
      ;;
  esac
}

render_gif() {
  if [[ "$BACKEND" != "asciinema" || "$RENDER_GIF" == "0" ]]; then
    return
  fi
  if ! command -v agg >/dev/null 2>&1; then
    printf 'agg not found; kept asciinema recording only.\n'
    return
  fi
  rm -f -- "$GIF_FILE"
  agg "$CAST_FILE" "$GIF_FILE"
}

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi
  if [[ "${1:-}" == "--stop" ]]; then
    stop_recorders
    exit 0
  fi
  [[ "$SESSION" =~ ^[A-Za-z0-9_.-]+$ ]] || die 'SESSION contains unsupported characters'

  need_cmd tmux
  resolve_kli
  pick_backend
  [[ -f "$ROOT_DIR/$KUBECONFIG_ARG" ]] || die "missing $KUBECONFIG_ARG; run: cd demos && make start"

  mkdir -p -- "$OUTPUT_DIR"
  DEMO_HOME="$(mktemp -d "${TMPDIR:-/tmp}/kli-demo-home.XXXXXX")"
  trap cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  printf 'preflight: %s --kubeconfig %s --check\n' "$KLI_BIN" "$KUBECONFIG_ARG"
  (cd "$ROOT_DIR" && "$KLI_BIN" --kubeconfig "$KUBECONFIG_ARG" --check >/dev/null)

  start_tmux
  run_walkthrough &
  DRIVER_PID="$!"
  record_session
  wait "$DRIVER_PID" 2>/dev/null || true
  DRIVER_PID=""
  render_gif

  case "$BACKEND" in
    asciinema)
      printf 'recorded: %s\n' "$CAST_FILE"
      if [[ -f "$GIF_FILE" ]]; then
        printf 'rendered: %s\n' "$GIF_FILE"
      else
        printf 'play: asciinema play %s\n' "$CAST_FILE"
      fi
      ;;
    script)
      printf 'recorded: %s\n' "$TYPESCRIPT_FILE"
      printf 'timing:   %s\n' "$TIMING_FILE"
      printf 'play:     scriptreplay --timing %s %s\n' "$TIMING_FILE" "$TYPESCRIPT_FILE"
      ;;
    terminal)
      printf 'played demo in the current terminal; no terminal recording file was generated\n'
      ;;
  esac
}

main "$@"

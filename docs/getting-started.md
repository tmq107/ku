# Getting started

## Install

```
go build -o kli .       # local binary
make install            # builds and installs to ~/.local/bin
```

Requires Go 1.24+ and access to a cluster. kli reads your default kubeconfig.

## Run

```
kli                       # current context, remembered namespace
kli --context my-ctx      # a specific context
kli -n kube-system        # start in a namespace
kli --resource deploy     # start on a resource type
kli --theme tokyonight    # use the Tokyo Night theme
kli --check               # read-only connectivity check, no UI
kli --version
```

## Flags

| Flag | Description |
| --- | --- |
| `--context` | kubeconfig context to use (default: current-context) |
| `-n`, `--namespace` | initial namespace (empty means all namespaces) |
| `--resource` | initial resource, e.g. `pods`, `deploy`, `svc` |
| `--theme` | `ansi` (default) or `tokyonight` |
| `--check` | run a read-only connectivity check and exit |
| `--version` | print version and exit |

## Layout

The screen has three parts:

- A left nav with common resource kinds, grouped by category.
- The main table, server-rendered with the same columns as `kubectl get`.
- A status bar that always shows the keys available right now, with the creator
  handle in the bottom-right.

Press `Tab` to move focus between the nav and the table. The focused pane has a
highlighted border.

## Themes

The default theme uses your terminal's own ANSI palette and adapts to a light or
dark background, so it matches whatever scheme you already run. For a fixed,
high-contrast look, pass `--theme tokyonight` or set `KLI_THEME=tokyonight`.

## Session memory

kli remembers the last context and namespace you used and restores them on the
next launch. Flags override the remembered values. State is written to
`$XDG_CONFIG_HOME/kli/state.json` (or the OS-default config dir).

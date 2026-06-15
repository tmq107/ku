# kli

A fast, keyboard-driven Kubernetes TUI. Browse any resource, read and edit
objects, follow logs, and open a shell in a pod, without leaving the terminal.
Inspired by k9s, Lens, and lazygit.

https://github.com/user-attachments/assets/48756c6b-00ae-470d-8fb5-3f93ecbd46df

## Install

Install the latest release with the installer:

```bash
curl -fsSL https://raw.githubusercontent.com/bjarneo/kli/main/install.sh | sh
```

Or with Go:

```bash
go install github.com/bjarneo/kli@latest
```

Or from a clone:

```bash
make install   # builds and installs to ~/.local/bin, /usr/local/bin, or your last $PATH dir
go build -o kli .
```

Building from source requires Go 1.26.3+. Running kli requires a reachable
cluster.

## Quick start

```
kli                       # current context, remembered namespace
kli -n kube-system        # start in a namespace
kli --resource deploy     # start on a resource type
kli --theme tokyonight    # switch theme
kli upgrade               # replace the current binary with the latest release
```

Press `?` for help and `Ctrl+K` for the command palette.

## Configuration

`kli` reads an optional config file from `~/.config/kli/config.yaml` for sidebar
customization and stores session state in `~/.config/kli/state.json`.

### Add custom resources (CRDs) to the sidebar

The sidebar lists common built-in resources by default. CRDs are not shown until
you add them. Seed the config, then edit the `sidebar:` list:

```bash
kli config init     # write ~/.config/kli/config.yaml with the defaults
kli config path     # print the config file location
```

Add an item under any section (or a new one). The `resource` field accepts a
plural, singular, kind, short name, or group-qualified key. Use the
group-qualified form (`<plural>.<group>`) to avoid ambiguity:

```yaml
sidebar:
  - section: CRDs
    items:
      - { label: ScaledObjects, resource: scaledobjects.keda.sh }
      - { label: Certificates, resource: certificates.cert-manager.io }
      - { label: HPAs, resource: horizontalpodautoscalers }
```

Restart `kli` to apply the change. Resources your cluster does not expose are
dropped, and empty sections are hidden.

See [Configuration](docs/configuration.md) for the full reference.

## Highlights

- A cockpit overview on launch: cluster health, node CPU and memory gauges, pod and deployment status, and recent warnings.
- Server-rendered tables for any resource, the same columns as `kubectl get`, including CRDs.
- lazygit-style layout: a left resource nav, `Tab` between panes, and a status bar that always shows the keys that work right now.
- Config summaries, raw YAML, logs, edit-in-editor, shell into pods or nodes, delete, scale, restart, and CronJob trigger, all inside the TUI.
- ANSI colors that match your terminal in light or dark mode, with Tokyo Night as a fallback (`--theme tokyonight`).
- A customizable sidebar menu via an optional config file (`kli config init`): add CRDs like HPAs, KEDA ScaledObjects, or OpenTelemetry collectors.
- `C` shows the equivalent `kubectl` command, and `O` opens upstream Kubernetes docs for known resources.
- Remembers your last context and namespace.

## Docs

- [Getting started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [Keybindings](docs/keybindings.md)
- [Features](docs/features.md)

Full index: [docs/](docs/README.md).

## Created by

[x.com/iamdothash](https://x.com/iamdothash)

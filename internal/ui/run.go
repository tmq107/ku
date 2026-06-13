package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bjarneo/kli/internal/k8s"
)

// Options configures a kli session.
type Options struct {
	Context   string
	Namespace string
	Resource  string
	Theme     string
}

// Run connects to the cluster and starts the interactive TUI. Flags take
// precedence over the remembered context/namespace from the last session.
func Run(opts Options) error {
	th := PickTheme(opts.Theme)

	saved, hasSaved := loadState()

	ctxName := opts.Context
	if ctxName == "" && hasSaved {
		ctxName = saved.Context
	}
	cl, err := k8s.NewClient(ctxName)
	if err != nil && opts.Context == "" && ctxName != "" {
		// The remembered context may no longer exist; fall back to the default.
		cl, err = k8s.NewClient("")
	}
	if err != nil {
		return err
	}

	app := NewApp(cl, th)
	switch {
	case opts.Namespace != "":
		app.namespace = opts.Namespace
		app.lastNS = opts.Namespace
	case hasSaved && saved.Context == cl.ContextName:
		// Only restore the namespace if we actually connected to the saved
		// context, so it stays meaningful.
		app.namespace = saved.Namespace
		if saved.Namespace != "" {
			app.lastNS = saved.Namespace
		}
	}
	if opts.Resource != "" {
		if ri, ok := cl.Registry().Resolve(opts.Resource); ok {
			app.res = ri
			app.sidebar.syncTo(ri.Key())
		}
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

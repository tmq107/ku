// Command kli is a terminal UI for Kubernetes: browse any resource, view and
// edit objects, follow logs, and switch namespaces and contexts, all from the
// keyboard. It uses your default kubeconfig.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bjarneo/kli/internal/k8s"
	"github.com/bjarneo/kli/internal/ui"
)

const version = "0.1.0"

func main() {
	var (
		ctxFlag, nsFlag, resFlag, themeFlag string
		checkFlag, versionFlag              bool
	)
	flag.StringVar(&ctxFlag, "context", "", "kubeconfig context to use (default: current-context)")
	flag.StringVar(&nsFlag, "namespace", "", "initial namespace (empty = all namespaces)")
	flag.StringVar(&nsFlag, "n", "", "initial namespace (shorthand)")
	flag.StringVar(&resFlag, "resource", "", "initial resource, e.g. pods, deploy, svc")
	flag.StringVar(&themeFlag, "theme", "", "color theme: ansi (default) or tokyonight")
	flag.BoolVar(&checkFlag, "check", false, "run a read-only connectivity check and exit")
	flag.BoolVar(&versionFlag, "version", false, "print version and exit")
	flag.Parse()

	switch {
	case versionFlag:
		fmt.Println("kli", version)
		return
	case checkFlag:
		if err := check(ctxFlag, nsFlag, resFlag); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if err := ui.Run(ui.Options{
		Context:   ctxFlag,
		Namespace: nsFlag,
		Resource:  resFlag,
		Theme:     themeFlag,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// check performs a read-only listing to validate connectivity and discovery
// without starting the UI. Useful in non-interactive environments.
func check(ctxName, ns, resQuery string) error {
	if resQuery == "" {
		resQuery = "pods"
	}
	cl, err := k8s.NewClient(ctxName)
	if err != nil {
		return err
	}
	fmt.Printf("context:   %s\nhost:      %s\nnamespace: %q\nresources: %d discovered\n",
		cl.ContextName, cl.Host, cl.Namespace, len(cl.Registry().All()))

	ri, ok := cl.Registry().Resolve(resQuery)
	if !ok {
		return fmt.Errorf("unknown resource %q", resQuery)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tbl, err := cl.ListTable(ctx, ri, ns)
	if err != nil {
		return err
	}
	fmt.Printf("listed %s: %d columns, %d rows\n", ri.Key(), len(tbl.Columns), len(tbl.Rows))
	return nil
}

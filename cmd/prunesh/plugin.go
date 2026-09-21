package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/prunesh/prunesh/internal/plugininstall"
	"github.com/prunesh/prunesh/internal/pluginregistry"
)

func runPlugin(args []string) {
	if len(args) == 0 {
		printPluginUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "install":
		runPluginInstall(args[1:])
	case "uninstall":
		runPluginUninstall(args[1:])
	case "list":
		runPluginList()
	default:
		fmt.Fprintf(os.Stderr, "prunesh plugin: unknown subcommand %q\n", args[0])
		printPluginUsage()
		os.Exit(1)
	}
}

func printPluginUsage() {
	fmt.Fprintln(os.Stderr, "usage: prunesh plugin install github.com/<user>/<repo>@<version> [--replace] | uninstall <id> | list")
}

func runPluginInstall(args []string) {
	var replace bool
	var ref string
	for _, arg := range args {
		switch {
		case arg == "--replace":
			replace = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "prunesh plugin install: unknown flag %q\n", arg)
			os.Exit(1)
		default:
			if ref != "" {
				fmt.Fprintln(os.Stderr, "usage: prunesh plugin install <module@version> [--replace]")
				os.Exit(1)
			}
			ref = arg
		}
	}
	if ref == "" {
		fmt.Fprintln(os.Stderr, "usage: prunesh plugin install <module@version> [--replace]")
		os.Exit(1)
	}
	module, pluginVer, err := plugininstall.ParseRef(ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh plugin install: %v\n", err)
		os.Exit(1)
	}
	rec, err := plugininstall.Install(plugininstall.Options{
		Module:      module,
		Version:     pluginVer,
		CoreVersion: version,
		ReleaseRepo: releaseRepo(),
		Replace:     replace,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh plugin install: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("installed %s@%s -> %s (%s)\n", rec.Module, rec.Version, rec.ID, rec.BinaryPath)
}


func runPluginUninstall(args []string) {
	if len(args) != 1 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "usage: prunesh plugin uninstall <id>")
		os.Exit(1)
	}
	rec, err := plugininstall.Uninstall(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh plugin uninstall: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("uninstalled %s (argv0=%s)\n", rec.ID, rec.Argv0)
}

func runPluginList() {
	db, err := pluginregistry.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh plugin list: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	recs, err := db.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh plugin list: %v\n", err)
		os.Exit(1)
	}
	if len(recs) == 0 {
		fmt.Println("no plugins installed")
		return
	}
	activeByArgv0, err := activePluginIDs(db, recs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh plugin list: %v\n", err)
		os.Exit(1)
	}
	for _, rec := range recs {
		tag := ""
		if activeByArgv0[rec.Argv0] == rec.ID {
			tag = "  active"
		}
		fmt.Printf("%s  %s@%s  argv0=%s%s  %s\n", rec.ID, rec.Module, rec.Version, rec.Argv0, tag, rec.BinaryPath)
	}
}

func activePluginIDs(db *pluginregistry.DB, recs []pluginregistry.Record) (map[string]string, error) {
	seen := make(map[string]struct{})
	out := make(map[string]string)
	for _, rec := range recs {
		if _, ok := seen[rec.Argv0]; ok {
			continue
		}
		seen[rec.Argv0] = struct{}{}
		active, err := db.Active(rec.Argv0)
		if err != nil {
			return nil, err
		}
		if active != nil {
			out[rec.Argv0] = active.ID
		}
	}
	return out, nil
}

func releaseRepo() string {
	if v := os.Getenv("PRUNESH_RELEASE_REPO"); v != "" {
		return v
	}
	return "prunesh/prunesh"
}

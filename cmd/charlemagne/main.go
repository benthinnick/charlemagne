// Command charlemagne generates resttest-shaped Go services from templates.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benthinnick/charlemagne/internal/generate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "charlemagne: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	parsed, err := parseArgs(args)
	if err != nil {
		return err
	}

	path := generate.ResolveConfigPath(parsed.configPath)
	cat, err := generate.LoadCatalog(path)
	if err != nil {
		return err
	}
	svc, err := cat.ServiceByName(parsed.name)
	if err != nil {
		return err
	}
	proj, err := generate.ProjectFromService(svc)
	if err != nil {
		return err
	}

	dest := filepath.Join(".", proj.Name)
	switch parsed.cmd {
	case "new":
		if err := generate.Generate(dest, proj); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%s) from %s\n", dest, proj.Module, path)
	case "build":
		if err := generate.Build(dest, svc, path, generate.BuildOptions{
			Stage: parsed.stage,
			From:  parsed.from,
		}); err != nil {
			return err
		}
		fmt.Printf("built %s from %s\n", dest, path)
	default:
		return fmt.Errorf("%s", usageAll)
	}
	return nil
}

const usageAll = "usage: charlemagne <new|build> <name> [--config path] [--stage name] [--from name]"

type cliArgs struct {
	cmd, name, configPath, stage, from string
}

func parseArgs(args []string) (cliArgs, error) {
	if len(args) == 0 {
		return cliArgs{}, fmt.Errorf("%s", usageAll)
	}
	out := cliArgs{cmd: args[0]}
	if out.cmd != "new" && out.cmd != "build" {
		return cliArgs{}, fmt.Errorf("%s", usageAll)
	}
	usage := "usage: charlemagne new <name> [--config path]"
	if out.cmd == "build" {
		usage = "usage: charlemagne build <name> [--config path] [--stage name] [--from name]"
	}
	args = args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config":
			if i+1 >= len(args) {
				return cliArgs{}, fmt.Errorf("%s", usage)
			}
			out.configPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--config="):
			out.configPath = strings.TrimPrefix(args[i], "--config=")
		case args[i] == "--stage":
			if out.cmd != "build" {
				return cliArgs{}, fmt.Errorf("unknown flag %s\n%s", args[i], usage)
			}
			if i+1 >= len(args) {
				return cliArgs{}, fmt.Errorf("%s", usage)
			}
			out.stage = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--stage="):
			if out.cmd != "build" {
				return cliArgs{}, fmt.Errorf("unknown flag %s\n%s", args[i], usage)
			}
			out.stage = strings.TrimPrefix(args[i], "--stage=")
		case args[i] == "--from":
			if out.cmd != "build" {
				return cliArgs{}, fmt.Errorf("unknown flag %s\n%s", args[i], usage)
			}
			if i+1 >= len(args) {
				return cliArgs{}, fmt.Errorf("%s", usage)
			}
			out.from = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--from="):
			if out.cmd != "build" {
				return cliArgs{}, fmt.Errorf("unknown flag %s\n%s", args[i], usage)
			}
			out.from = strings.TrimPrefix(args[i], "--from=")
		case strings.HasPrefix(args[i], "-"):
			return cliArgs{}, fmt.Errorf("unknown flag %s\n%s", args[i], usage)
		case out.name == "":
			out.name = args[i]
		default:
			return cliArgs{}, fmt.Errorf("%s", usage)
		}
	}
	if out.name == "" {
		return cliArgs{}, fmt.Errorf("%s", usage)
	}
	if out.stage != "" && out.from != "" {
		return cliArgs{}, fmt.Errorf("use --stage or --from, not both")
	}
	return out, nil
}

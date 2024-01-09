package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jdkato/twine/nlp/tag"
	"github.com/pterm/pterm"

	"github.com/errata-ai/vale/v2/internal/check"
	"github.com/errata-ai/vale/v2/internal/core"
	"github.com/errata-ai/vale/v2/internal/lint"
	"github.com/errata-ai/vale/v2/internal/nlp"
)

// TaggedWord is a word with an NLP context.
type TaggedWord struct {
	Token tag.Token
	Line  int
	Span  []int
}

// CompiledRule is a rule's compiled regex.
type CompiledRule struct {
	Pattern string
}

var commandInfo = map[string]string{
	"ls-config":  "Print the current configuration to stdout.",
	"ls-metrics": "Print the given file's internal metrics to stdout.",
	"ls-dirs":    "Print the default configuration directories to stdout.",
	"sync":       "Download and install external configuration sources.",
	"fix":        "Attempt to automatically fix the given alert.",
}

// Actions are the available CLI commands.
var Actions = map[string]func(args []string, flags *core.CLIFlags) error{
	"ls-config":  printConfig,
	"ls-metrics": printMetrics,
	"ls-dirs":    printDirs,
	"dc":         printConfig,
	"tag":        runTag,
	"sync":       sync,
	"fix":        fix,

	// private
	"compile": compileRule,
	"run":     runRule,
}

func fix(args []string, flags *core.CLIFlags) error {
	if len(args) != 1 {
		return core.NewE100("fix", errors.New("one argument expected"))
	}

	alert := args[0]
	if core.FileExists(args[0]) {
		b, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		alert = string(b)
	}

	cfg, err := core.ReadPipeline(flags, false)
	if err != nil {
		return err
	}

	resp, err := lint.ParseAlert(alert, cfg)
	if err != nil {
		return err
	}

	return printJSON(resp)
}

func sync(_ []string, flags *core.CLIFlags) error {
	cfg, err := core.ReadPipeline(flags, true)
	if err != nil {
		return err
	} else if err = initPath(cfg); err != nil {
		return err
	}

	p, err := pterm.DefaultProgressbar.WithTotal(
		len(cfg.ConfigFiles)).WithTitle("Downloading packages for config files").Start()

	for _, cfgPath := range cfg.ConfigFiles {
		pkgs, err := core.GetPackages(cfgPath)
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}

		stylesPath, err := core.GetStylesPath(cfgPath)
		if err != nil {
			return err
		}

		for idx, pkg := range pkgs {
			if err = readPkg(pkg, stylesPath, idx); err != nil {
				return err
			}
			name := fileNameWithoutExt(pkg)
			pterm.Success.Println("Downloaded package '" + name + "'")
		}

		p.Increment()
	}

	return nil
}

func printConfig(_ []string, flags *core.CLIFlags) error {
	cfg, err := core.ReadPipeline(flags, false)
	if err != nil {
		return err
	}
	fmt.Println(cfg.String())
	return nil
}

func printMetrics(args []string, _ *core.CLIFlags) error {
	if len(args) != 1 {
		return core.NewE100("ls-metrics", errors.New("one argument expected"))
	} else if !core.FileExists(args[0]) {
		return errors.New("file not found")
	}

	cfg, err := core.NewConfig(&core.CLIFlags{})
	if err != nil {
		return err
	}

	cfg.MinAlertLevel = 0
	cfg.GBaseStyles = []string{"Vale"}
	cfg.Flags.InExt = ".txt" // default value

	linter, err := lint.NewLinter(cfg)
	if err != nil {
		return err
	}

	linted, err := linter.Lint([]string{args[0]}, "*")
	if err != nil {
		return err
	}

	computed, _ := linted[0].ComputeMetrics()
	return printJSON(computed)
}

func runTag(args []string, _ *core.CLIFlags) error {
	if len(args) != 3 {
		return core.NewE100("tag", errors.New("three arguments expected"))
	}

	text, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	out := core.TextToContext(
		string(text), &nlp.Info{Lang: args[1], Endpoint: args[2]})

	return printJSON(out)
}

func compileRule(args []string, _ *core.CLIFlags) error {
	if len(args) != 1 {
		return core.NewE100("compile", errors.New("one argument expected"))
	}

	cfg, err := core.NewConfig(&core.CLIFlags{})
	if err != nil {
		return err
	}

	path := args[0]
	name := filepath.Base(path)

	mgr, err := check.NewManager(cfg)
	if err != nil {
		return err
	}

	err = mgr.AddRuleFromFile(name, path)
	if err != nil {
		return err
	}

	rule := CompiledRule{Pattern: mgr.Rules()[name].Pattern()}
	return printJSON(rule)
}

func runRule(args []string, _ *core.CLIFlags) error {
	if len(args) != 2 {
		return core.NewE100("run", errors.New("two arguments expected"))
	}

	cfg, err := core.NewConfig(&core.CLIFlags{})
	if err != nil {
		return err
	}

	cfg.MinAlertLevel = 0
	cfg.GBaseStyles = []string{"Test"}
	cfg.Flags.InExt = ".txt" // default value

	linter, err := lint.NewLinter(cfg)
	if err != nil {
		return err
	}

	err = linter.Manager.AddRuleFromFile("Test.Rule", args[0])
	if err != nil {
		return err
	}

	linted, err := linter.Lint([]string{args[1]}, "*")
	if err != nil {
		return err
	}

	PrintJSONAlerts(linted)
	return nil
}

func printDirs(_ []string, flags *core.CLIFlags) error {
	styles, err := core.DefaultStylesPath()
	if err != nil {
		return err
	}

	cfg, err := core.DefaultConfig()
	if err != nil {
		return err
	}

	tableData := pterm.TableData{
		{"Asset", "Location"},
		{"StylesPath", styles},
		{".vale.ini", cfg},
	}

	return pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
}

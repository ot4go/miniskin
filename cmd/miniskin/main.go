package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ot4go/miniskin"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	cmd := os.Args[1]
	argsOffset := 2

	// Handle "mockup" subcommands
	if cmd == "mockup" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin mockup <update|clean|negative> [flags]\n")
			os.Exit(1)
		}
		cmd = "mockup " + os.Args[2]
		argsOffset = 3
	}

	// Validate command
	switch cmd {
	case "run", "generate", "generate-claude-skill", "mockup update", "mockup clean", "mockup negative", "deps", "combine", "split":
		// valid
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
	}

	// Parse flags after the command
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	contentPath := fs.String("content", ".", "path to content directory")
	modulesPath := fs.String("modules", ".", "path to modules directory")
	verbose := fs.Bool("v", false, "verbose output (dependency analysis, processing order)")
	debug := fs.Bool("vv", false, "debug output (all internal details)")
	silent := fs.Bool("silent", false, "suppress all output")

	// Flags exclusive to mockup negative
	var negativeSrc, negativeDst *string
	if cmd == "mockup negative" {
		negativeSrc = fs.String("src", "", "source mockup file (required)")
		negativeDst = fs.String("dst", "", "destination negative template file (required)")
	}

	// Flags exclusive to generate-claude-skill
	var skillDst *string
	var skillForce *bool
	if cmd == "generate-claude-skill" {
		skillDst = fs.String("dst", ".claude/skills/miniskin/SKILL.md", "destination path")
		skillForce = fs.Bool("force", false, "overwrite existing destination file")
	}

	fs.Parse(os.Args[argsOffset:])

	verbosity := miniskin.VerbosityNormal
	if *debug {
		verbosity = miniskin.VerbosityDebug
	} else if *verbose {
		verbosity = miniskin.VerbosityVerbose
	} else if *silent {
		verbosity = miniskin.VerbositySilent
	}

	var err error
	switch cmd {
	case "run":
		err = miniskin.MiniskinRun(*contentPath, *modulesPath, verbosity)
	case "generate":
		err = miniskin.MiniskinGenerate(*contentPath, *modulesPath, verbosity)
	case "mockup update":
		err = miniskin.MiniskinMockupUpdate(*contentPath, *modulesPath, verbosity)
	case "mockup clean":
		err = miniskin.MiniskinMockupClean(*contentPath, *modulesPath, verbosity)
	case "mockup negative":
		if *negativeSrc == "" || *negativeDst == "" {
			fmt.Fprintf(os.Stderr, "mockup negative requires -src and -dst flags\n")
			os.Exit(1)
		}
		absSrc, _ := filepath.Abs(*negativeSrc)
		absDst, _ := filepath.Abs(*negativeDst)
		var data []byte
		data, err = os.ReadFile(absSrc)
		if err != nil {
			break
		}
		result := miniskin.TransformNegative(string(data))
		err = os.WriteFile(absDst, []byte(result), 0644)
		if err == nil {
			fmt.Printf("negative: %s -> %s\n", *negativeSrc, *negativeDst)
		}
	case "generate-claude-skill":
		if !*skillForce {
			if _, statErr := os.Stat(*skillDst); statErr == nil {
				fmt.Fprintf(os.Stderr, "destination %s already exists (use -force to overwrite)\n", *skillDst)
				os.Exit(1)
			}
		}
		var content string
		content, err = miniskin.GenerateSkill()
		if err != nil {
			break
		}
		if mkErr := os.MkdirAll(filepath.Dir(*skillDst), 0755); mkErr != nil {
			err = mkErr
			break
		}
		err = os.WriteFile(*skillDst, []byte(content), 0644)
		if err == nil {
			fmt.Printf("skill generated: %s\n", *skillDst)
		}
	case "deps":
		ms := miniskin.MiniskinNew(*contentPath, *modulesPath).SetVerbosity(verbosity)
		var dm *miniskin.DepMap
		dm, err = ms.AnalyzeDeps()
		if err == nil {
			fmt.Print(dm.String())
			order, orderErr := dm.ProcessingOrder()
			if orderErr != nil {
				err = orderErr
			} else {
				fmt.Println("\n=== Processing Order ===")
				for i, src := range order {
					fmt.Printf("  %d. %s\n", i+1, src)
				}
			}
		}
	case "combine":
		args := fs.Args()
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin combine <directory>\n")
			os.Exit(1)
		}
		err = miniskin.CombineDir(args[0])
		if err == nil {
			fmt.Printf("combined: %s\n", args[0])
		}
	case "split":
		args := fs.Args()
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: miniskin split <file.miniskin.xml>\n")
			os.Exit(1)
		}
		err = miniskin.SplitXML(args[0])
		if err == nil {
			fmt.Printf("split: %s\n", args[0])
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: miniskin <command> [flags]\n\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  run                    Mockup update + Build + Generate code\n")
	fmt.Fprintf(os.Stderr, "  generate               Build embed assets + Generate Go code\n")
	fmt.Fprintf(os.Stderr, "  generate-claude-skill         Generate Claude Code SKILL.md\n")
	fmt.Fprintf(os.Stderr, "  mockup update          Export mockup pieces + Refresh imports\n")
	fmt.Fprintf(os.Stderr, "  mockup clean           Empty inline content of mockup-import blocks\n")
	fmt.Fprintf(os.Stderr, "  mockup negative        Transform a mockup file into a negative template\n")
	fmt.Fprintf(os.Stderr, "  deps                   Show dependency map and processing order\n")
	fmt.Fprintf(os.Stderr, "  combine <dir>          Combine subdirectory XMLs into one\n")
	fmt.Fprintf(os.Stderr, "  split <file>           Split nested resource-lists into separate XMLs\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  -content string        path to content directory (default \".\")\n")
	fmt.Fprintf(os.Stderr, "  -modules string        path to modules directory (default \".\")\n")
	fmt.Fprintf(os.Stderr, "  -v                     verbose output\n")
	fmt.Fprintf(os.Stderr, "  -vv                    debug output\n")
	fmt.Fprintf(os.Stderr, "  -silent                suppress all output\n")
	fmt.Fprintf(os.Stderr, "\nMockup negative flags:\n")
	fmt.Fprintf(os.Stderr, "  -src string            source mockup file (required)\n")
	fmt.Fprintf(os.Stderr, "  -dst string            destination negative template file (required)\n")
	fmt.Fprintf(os.Stderr, "\nGenerate-skill flags:\n")
	fmt.Fprintf(os.Stderr, "  -dst string            destination path (default: .claude/skills/miniskin/SKILL.md)\n")
	fmt.Fprintf(os.Stderr, "  -force                 overwrite existing destination file\n")
	os.Exit(1)
}

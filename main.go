package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "1.2.0"

type fileState struct {
	content []byte
	exists  bool
	deleted bool
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  patcher <patch-file.txt> [--dry-run]   Apply patch file")
	fmt.Println("  patcher version                        Show installed version")
	fmt.Println("  patcher help                           Show this help message")
	fmt.Println("\nOptions:")
	fmt.Println("  --dry-run, -n                          Preview changes without writing to disk")
	fmt.Println("  --version, -v                          Show version information")
	fmt.Println("  --help, -h                             Show this help message")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// First-class subcommands & version checks
	switch os.Args[1] {
	case "version", "--version", "-v", "-version":
		fmt.Printf("patcher v%s\n", version)
		os.Exit(0)
	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)
	}

	var patchFile string
	var dryRun bool

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		case "--version", "-v", "version":
			fmt.Printf("patcher v%s\n", version)
			os.Exit(0)
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n", arg)
				printUsage()
				os.Exit(1)
			}
			if patchFile == "" {
				patchFile = arg
			} else {
				fmt.Fprintf(os.Stderr, "Unexpected extra argument: %s\n", arg)
				printUsage()
				os.Exit(1)
			}
		}
	}

	if patchFile == "" {
		printUsage()
		os.Exit(1)
	}

	rawBytes, err := os.ReadFile(patchFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", patchFile, err)
		os.Exit(1)
	}

	content := cleanContent(string(rawBytes))
	actions, parseErrors := parsePatch(content)

	if len(parseErrors) > 0 {
		fmt.Fprintf(os.Stderr, "Patch syntax errors detected in %s:\n", patchFile)
		for _, pe := range parseErrors {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", pe)
		}
		fmt.Fprintln(os.Stderr, "\nAborted. No files were modified.")
		os.Exit(1)
	}

	if len(actions) == 0 {
		fmt.Printf("No actions matched in %s (file length: %d bytes).\n", patchFile, len(rawBytes))
		if strings.Contains(content, "\x3c"+"rep"+"lace") || strings.Contains(content, "\x3c"+"fi"+"le") {
			fmt.Println("Hint: Tags were found in the text, but the syntax didn't match the expected structure.")
		}
		os.Exit(1)
	}

	// PASS 1: In-Memory Simulation & Atomic Validation
	vfs := make(map[string]*fileState)
	getFile := func(path string) (*fileState, error) {
		if st, ok := vfs[path]; ok {
			return st, nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				st := &fileState{exists: false}
				vfs[path] = st
				return st, nil
			}
			return nil, err
		}
		st := &fileState{content: b, exists: true}
		vfs[path] = st
		return st, nil
	}

	var simErrors []string
	for _, act := range actions {
		switch act.Type {
		case ActionFile:
			st, err := getFile(act.Path)
			if err != nil {
				simErrors = append(simErrors, fmt.Sprintf("Line %d: Error reading %s: %v", act.Line, act.Path, err))
				continue
			}
			st.content = []byte(act.Body)
			st.exists = true
			st.deleted = false

		case ActionDelete:
			st, err := getFile(act.Path)
			if err != nil {
				simErrors = append(simErrors, fmt.Sprintf("Line %d: Error reading %s: %v", act.Line, act.Path, err))
				continue
			}
			st.deleted = true

		case ActionReplace:
			st, err := getFile(act.Path)
			if err != nil {
				simErrors = append(simErrors, fmt.Sprintf("Line %d: Error reading %s: %v", act.Line, act.Path, err))
				continue
			}
			if !st.exists || st.deleted {
				simErrors = append(simErrors, fmt.Sprintf("Line %d: Cannot replace in %s: file does not exist", act.Line, act.Path))
				continue
			}
			newStr, err := applyReplace(string(st.content), act)
			if err != nil {
				simErrors = append(simErrors, fmt.Sprintf("Line %d: ✗ %v in: %s", act.Line, err, act.Path))
				continue
			}
			st.content = []byte(newStr)
		}
	}

	if len(simErrors) > 0 {
		fmt.Fprintf(os.Stderr, "Patch verification failed (%d errors):\n", len(simErrors))
		for _, se := range simErrors {
			fmt.Fprintf(os.Stderr, "  %s\n", se)
		}
		fmt.Fprintln(os.Stderr, "\nWorking tree left clean. Zero files were modified.")
		os.Exit(1)
	}

	// PASS 2: Commit verified changes to disk or preview dry-run
	for _, act := range actions {
		prefix := "✓"
		if dryRun {
			prefix = "[dry-run]"
		}

		switch act.Type {
		case ActionFile:
			if dryRun {
				fmt.Printf("%s WRITE %s (%d bytes)\n", prefix, act.Path, len(act.Body))
			} else {
				fmt.Printf("%s Wrote: %s\n", prefix, act.Path)
			}
		case ActionReplace:
			desc := ""
			if act.From != "" {
				desc = fmt.Sprintf(" (from %q to %q)", act.From, act.To)
			} else if act.Symbol != "" {
				desc = fmt.Sprintf(" (symbol %q)", act.Symbol)
			} else if act.Lines != "" {
				desc = fmt.Sprintf(" (lines %s)", act.Lines)
			} else if act.After != "" {
				desc = fmt.Sprintf(" (after %q)", act.After)
			} else if act.Before != "" {
				desc = fmt.Sprintf(" (before %q)", act.Before)
			}
			if dryRun {
				fmt.Printf("%s REPLACE in %s%s\n", prefix, act.Path, desc)
			} else {
				fmt.Printf("%s Patched: %s%s\n", prefix, act.Path, desc)
			}
		case ActionDelete:
			if dryRun {
				fmt.Printf("%s DELETE %s\n", prefix, act.Path)
			} else {
				fmt.Printf("%s Deleted: %s\n", prefix, act.Path)
			}
		}
	}

	if !dryRun {
		for path, st := range vfs {
			if st.deleted {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", path, err)
				}
			} else if st.exists {
				if dir := filepath.Dir(path); dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
						continue
					}
				}
				if err := os.WriteFile(path, st.content, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
				}
			}
		}
	}

	if dryRun {
		fmt.Printf("\nDone. %d changes simulated successfully (dry-run).\n", len(actions))
	} else {
		fmt.Printf("\nDone. %d changes applied successfully.\n", len(actions))
	}
}

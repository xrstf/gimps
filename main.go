// SPDX-FileCopyrightText: 2023 Christoph Mewes
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/incu6us/goimports-reviser/v3/pkg/module"
	"github.com/spf13/pflag"

	"go.xrstf.de/gimps/pkg/gimps"
)

// These variables get set by ldflags during compilation.
var (
	BuildTag    string
	BuildCommit string
	BuildDate   string // RFC3339 format ("2006-01-02T15:04:05Z07:00")
)

func printVersion() {
	// handle empty values in case `go install` was used
	if BuildCommit == "" {
		fmt.Printf("gimps dev, built with %s\n",
			runtime.Version(),
		)
	} else {
		fmt.Printf("gimps %s (%s), built with %s on %s\n",
			BuildTag,
			BuildCommit[:10],
			runtime.Version(),
			BuildDate,
		)
	}
}

func main() {
	configFile := ""
	dryRun := false
	showVersion := false
	stdout := false
	verbose := false

	pflag.StringVarP(&configFile, "config", "c", configFile, "Path to the config file (mandatory).")
	pflag.BoolVarP(&stdout, "stdout", "s", stdout, "Print output to stdout instead of updating the source file(s).")
	pflag.BoolVarP(&dryRun, "dry-run", "d", dryRun, "Do not update files.")
	pflag.BoolVarP(&verbose, "verbose", "v", verbose, "List all instead of just changed files.")
	pflag.BoolVarP(&showVersion, "version", "V", showVersion, "Show version and exit.")
	pflag.Parse()

	if showVersion {
		printVersion()
		return
	}

	if pflag.NArg() == 0 {
		log.Fatal("Usage: gimps [--stdout] [--dry-run] [--config=(autodetect)] FILE_OR_DIRECTORY[, ...]")
	}

	inputs, err := cleanupArgs(pflag.Args())
	if err != nil {
		log.Fatalf("Invalid arguments: %v.", err)
	}

	// to auto-detect the .gimps.yaml, we need to find the go.mod; this can fail in
	// some special repos, so the "guess .gimps.yaml location" logic is best effort only
	modRoot, modRootErr := goModRootPath(inputs[0])

	config, err := loadConfiguration(configFile, modRoot)
	if err != nil {
		log.Fatalf("Failed to load -config file %q: %v", configFile, err)
	}

	if config.ProjectName == "" {
		if modRootErr != nil {
			log.Fatalf("Failed to auto-detect module root: %v", err)
		}

		modName, err := module.Name(modRoot)
		if err != nil {
			log.Fatalf("Failed to auto-detect project name based on the first given file (%q): %v", inputs[0], err)
		}

		config.ProjectName = modName
	}

	aliaser, err := gimps.NewAliaser(config.ProjectName, config.AliasRules)
	if err != nil {
		log.Fatalf("Failed to initialize aliaser: %v", err)
	}

	for _, input := range inputs {
		filenames, err := listFiles(input, modRoot, config.Exclude)
		if err != nil {
			log.Fatalf("Failed to process %q: %v", input, err)
		}

		for _, filename := range filenames {
			if *config.DetectGeneratedFiles {
				generated, err := isGeneratedFile(filename)
				if err != nil {
					log.Fatalf("Cannot check if file %q is generated: %v", filename, err)
				}

				if generated {
					continue
				}
			}

			relPath, err := filepath.Rel(modRoot, filename)
			if err != nil {
				log.Fatalf("This should never happen, could not determine relative path: %v", err)
			}

			if verbose {
				log.Printf("> %s", relPath)
			}

			formattedOutput, hasChange, err := gimps.Execute(&config.Config, filename, aliaser)
			if err != nil {
				log.Fatalf("Failed to process %q: %v", filename, err)
			}

			if stdout {
				fmt.Print(string(formattedOutput))
			} else if hasChange {
				if verbose {
					log.Printf("! %s", relPath)
				} else {
					log.Printf("Fixed %s", relPath)
				}

				if !dryRun {
					if err := os.WriteFile(filename, formattedOutput, 0644); err != nil {
						log.Fatalf("Failed to write fixed result to file %q: %v", filename, err)
					}
				}
			}
		}
	}
}

// cleanupArgs removes duplicates and turns every argument into an absolute
// filesystem path. The result is sorted alphabetically.
func cleanupArgs(args []string) ([]string, error) {
	unique := map[string]struct{}{}

	for _, arg := range args {
		if arg == "" {
			var err error

			arg, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("invalid path %q: %v", arg, err)
			}
		}

		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %v", arg, err)
		}

		unique[abs] = struct{}{}
	}

	result := []string{}
	for path := range unique {
		result = append(result, path)
	}

	sort.Strings(result)

	return result, nil
}

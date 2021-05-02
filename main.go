package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
	"github.com/incu6us/goimports-reviser/v2/pkg/module"

	"go.xrstf.de/gimps/pkg/gimps"
)

// Project build specific vars
var (
	Tag       string
	Commit    string
	SourceURL string
	GoVersion string
)

func printVersion() {
	fmt.Printf(
		"version: %s\nbuild with: %s\ntag: %s\ncommit: %s\nsource: %s\n",
		strings.TrimPrefix(Tag, "v"),
		GoVersion,
		Tag,
		Commit,
		SourceURL,
	)
}

func main() {
	configFile := ""
	showVersion := false
	stdout := false

	flag.StringVar(&configFile, "config", configFile, "Path to the config file (mandatory).")
	flag.BoolVar(&stdout, "stdout", showVersion, "Print output to stdout instead of updating the source file(s).")
	flag.BoolVar(&showVersion, "version", stdout, "Show version and exit.")
	flag.Parse()

	if showVersion {
		printVersion()
		return
	}

	if flag.NArg() == 0 {
		log.Fatal("Usage: gimps [-stdout] [-config=(autodetect)] FILE_OR_DIRECTORY[, ...]")
	}

	inputs, err := cleanupArgs(flag.Args())
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

	for _, input := range inputs {
		filenames, err := listFiles(input, modRoot, config.Exclude)
		if err != nil {
			log.Fatalf("Failed to list files in %q: %v", input, err)
		}

		for _, filename := range filenames {
			formattedOutput, hasChange, err := gimps.Execute(&config.Config, filename)
			if err != nil {
				log.Fatalf("Failed to process %q: %v", filename, err)
			}

			if stdout {
				fmt.Print(string(formattedOutput))
			} else if hasChange {
				relPath, err := filepath.Rel(modRoot, filename)
				if err != nil {
					log.Fatalf("This should never happen, could not determine relative path: %v", err)
				}

				log.Printf("Fixing %s", relPath)

				if err := ioutil.WriteFile(filename, formattedOutput, 0644); err != nil {
					log.Fatalf("Failed to write fixed result to file %q: %v", filename, err)
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

func listFiles(start string, moduleRoot string, skips []string) ([]string, error) {
	result := []string{}

	err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		relPath, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			for _, skip := range skips {
				if match, _ := doublestar.Match(skip, relPath); match {
					return filepath.SkipDir
				}
			}
		} else if strings.HasSuffix(path, ".go") {
			result = append(result, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func goModRootPath(path string) (string, error) {
	// turn path into directory, if it's a file
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}

	for {
		if fi, err := os.Stat(filepath.Join(path, "go.mod")); err == nil && !fi.IsDir() {
			return path, nil
		}

		d := filepath.Dir(path)
		if d == path {
			break
		}

		path = d
	}

	return "", errors.New("no go.mod found")
}

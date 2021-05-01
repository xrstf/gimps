package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/incu6us/goimports-reviser/v2/pkg/module"

	"go.xrstf.de/go-imports-sorter/reviser"
)

// Project build specific vars
var (
	Tag       string
	Commit    string
	SourceURL string
	GoVersion string

	configFile  string
	showVersion bool
	stdout      bool
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
	flag.StringVar(&configFile, "config", "", "Path to the config file (mandatory).")
	flag.BoolVar(&stdout, "stdout", false, "Print output to stdout instead of updating the source file(s).")
	flag.BoolVar(&showVersion, "version", false, "Show version and exit.")
	flag.Parse()

	if showVersion {
		printVersion()
		return
	}

	if flag.NArg() == 0 {
		log.Fatal("Usage: goimports-reviser [-stdout] -config CONFIG_FILE FILE[, ...]")
	}
	files := flag.Args()

	if len(configFile) == 0 {
		log.Fatal("Usage: goimports-reviser [-stdout] -config CONFIG_FILE FILE[, ...]")
	}

	config, err := loadConfiguration(configFile)
	if err != nil {
		log.Fatalf("Failed to load config file %q: %v", configFile, err)
	}

	if config.ProjectName == "" {
		projectName, err := determineProjectName(files[0])
		if err != nil {
			log.Fatalf("Failed to auto-detect project name based on the first given file (%q): %v", files[0], err)
		}
		config.ProjectName = projectName
	}

	for _, filename := range files {
		formattedOutput, hasChange, err := reviser.Execute(config, filename)
		if err != nil {
			log.Fatalf("Failed to process %q: %v", filename, err)
		}

		if stdout {
			fmt.Print(string(formattedOutput))
		} else if hasChange {
			if err := ioutil.WriteFile(filename, formattedOutput, 0644); err != nil {
				log.Fatalf("failed to write fixed result to file(%s): %v", filename, err)
			}
		}
	}
}

func determineProjectName(filePath string) (string, error) {
	projectRootPath, err := module.GoModRootPath(filePath)
	if err != nil {
		return "", err
	}

	moduleName, err := module.Name(projectRootPath)
	if err != nil {
		return "", err
	}

	return moduleName, nil
}

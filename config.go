package main

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"go.xrstf.de/gimps/pkg/gimps"
)

const (
	defaultConfigFile = ".gimps.yaml"
)

type Config struct {
	gimps.Config         `yaml:",inline"`
	Exclude              []string `yaml:"exclude"`
	DetectGeneratedFiles *bool    `yaml:"detectGeneratedFiles"`
}

func loadConfiguration(filename string, moduleRoot string) (*Config, error) {
	// user did not specify a config file
	if filename == "" {
		if moduleRoot == "" {
			return nil, errors.New("no -config specified and could not automatically find go module root")
		}

		filename = filepath.Join(moduleRoot, defaultConfigFile)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := &Config{}
	if err := yaml.NewDecoder(f).Decode(c); err != nil {
		return nil, err
	}

	if c.Exclude == nil || len(c.Exclude) == 0 {
		// vendor is because we never want to modify vendor, the
		// others are just to save time while scanning bigger
		// repositories that maybe also contain non-Go stuff
		c.Exclude = []string{
			// to not break 3rd party code
			"vendor/**",

			// to not muck with generated files
			"**/zz_generated.**",
			"**/zz_generated_**",
			"**/generated.pb.go",
			"**/*_generated.go",

			// for performance
			".git/**",
			"_build/**",
			"node_modules/**",
		}
	}

	if c.DetectGeneratedFiles == nil {
		yesPlease := true
		c.DetectGeneratedFiles = &yesPlease
	}

	return c, nil
}

// SPDX-FileCopyrightText: 2023 Christoph Mewes
// SPDX-License-Identifier: MIT

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

var (
	defaultExcludes = []string{
		// to not break 3rd party code
		"vendor/**",

		// to not muck with generated files
		"**/zz_generated.**",
		"**/zz_generated_**",
		"**/generated.pb.go",
		"**/generated.proto",
		"**/*_generated.go",

		// for performance
		".git/**",
		"_build/**",
		"node_modules/**",
	}
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

		// try if there is a .gimps.yaml in the module root
		filename = filepath.Join(moduleRoot, defaultConfigFile)

		if _, err := os.Stat(filename); err != nil {
			// file does not exist, so we just return the default config
			return defaultConfig(&Config{}), nil
		}

		// file exists, continue loading as normal
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

	return defaultConfig(c), nil
}

func defaultConfig(c *Config) *Config {
	if c.Exclude == nil {
		c.Exclude = defaultExcludes
	}

	if c.DetectGeneratedFiles == nil {
		yesPlease := true
		c.DetectGeneratedFiles = &yesPlease
	}

	return c
}

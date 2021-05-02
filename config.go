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

func loadConfiguration(filename string, moduleRoot string) (*gimps.Config, error) {
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

	c := &gimps.Config{}
	if err := yaml.NewDecoder(f).Decode(c); err != nil {
		return nil, err
	}

	return c, nil
}

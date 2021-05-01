package main

import (
	"os"

	"github.com/incu6us/goimports-reviser/v2/reviser"
	"gopkg.in/yaml.v3"
)

func loadConfiguration(filename string) (*reviser.Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := &reviser.Config{}
	if err := yaml.NewDecoder(f).Decode(c); err != nil {
		return nil, err
	}

	return c, nil
}
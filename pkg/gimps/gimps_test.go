// SPDX-FileCopyrightText: 2023 Christoph Mewes
// SPDX-License-Identifier: MIT

package gimps

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func newTestConfig(projectName string) *Config {
	return &Config{
		ProjectName: projectName,
	}
}

func TestExecute(t *testing.T) {
	testcases, err := filepath.Glob("testdata/*")
	require.Nil(t, err)

	for _, testcase := range testcases {
		stat, err := os.Stat(testcase)
		require.Nil(t, err)

		if !stat.IsDir() {
			continue
		}

		name := filepath.Base(testcase)
		testcase, _ := filepath.Abs(testcase)

		t.Run(name, func(t *testing.T) {
			config := loadTestConfig(t, testcase)

			inputFile := filepath.Join(testcase, "main.go.input")
			goFile := filepath.Join(testcase, "main.go")
			os.Rename(inputFile, goFile)
			defer os.Rename(goFile, inputFile)

			aliaser, err := NewAliaser(config.ProjectName, config.AliasRules)
			assertTestError(t, err, config.ExpectedAliaserError)
			if config.ExpectedAliaserError != nil {
				return
			}

			output, _, err := Execute(&config.Config, goFile, aliaser)
			assertTestError(t, err, config.ExpectedExecuteError)
			if config.ExpectedExecuteError != nil {
				return
			}

			expectedFile := filepath.Join(testcase, "main.go.expected")
			expected, err := ioutil.ReadFile(expectedFile)
			require.Nil(t, err)
			assert.Equal(t, string(expected), string(output))
		})
	}
}

func assertTestError(t *testing.T, actual error, expected *string) {
	if expected == nil {
		require.Nil(t, actual)
	} else {
		require.EqualError(t, actual, *expected)
	}
}

type testConfig struct {
	Config `yaml:",inline"`

	ExpectedExecuteError *string `yaml:"expectedExecuteError"`
	ExpectedAliaserError *string `yaml:"expectedAliaserError"`
}

func loadTestConfig(t *testing.T, testcase string) *testConfig {
	f, err := os.Open(filepath.Join(testcase, "config.yaml"))
	require.Nil(t, err)
	defer f.Close()

	c := &testConfig{}
	err = yaml.NewDecoder(f).Decode(c)
	require.Nil(t, err)

	setDefaults(&c.Config)

	return c
}

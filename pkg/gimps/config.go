package gimps

import (
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
	"github.com/incu6us/goimports-reviser/v2/pkg/std"
)

type Config struct {
	ProjectName         string   `yaml:"projectName"`
	ImportOrder         []string `yaml:"importOrder"`
	Sets                []Set    `yaml:"sets"`
	RemoveUnusedImports *bool    `yaml:"removeUnusedImports"`
	SetVersionAlias     *bool    `yaml:"setVersionAlias"`
}

type Set struct {
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"`
}

func setDefaults(c *Config) {
	// set polite defaults
	noThanks := false

	if c.RemoveUnusedImports == nil {
		c.RemoveUnusedImports = &noThanks // yes is buggy
	}

	if c.SetVersionAlias == nil {
		c.SetVersionAlias = &noThanks
	}

	if len(c.ImportOrder) == 0 {
		c.ImportOrder = []string{SetStd, SetProject, SetExternal}
	}
}

type importSet []string

const (
	SetStd      = "std"
	SetProject  = "project"
	SetExternal = "external"
)

func (c *Config) classifyImport(imprt string) string {
	pkgWithoutAlias := trimPackageAlias(imprt)

	if _, ok := std.StdPackages[pkgWithoutAlias]; ok {
		return SetStd
	}

	for _, set := range c.Sets {
		for _, pattern := range set.Patterns {
			if matches, _ := doublestar.Match(pattern, pkgWithoutAlias); matches {
				return set.Name
			}
		}
	}

	if c.isProjectImport(pkgWithoutAlias) {
		return SetProject
	}

	return SetExternal
}

func (c *Config) isProjectImport(pkgWithoutAlias string) bool {
	if pkgWithoutAlias == c.ProjectName {
		return true
	}

	prefix := c.ProjectName + "/"

	return strings.HasPrefix(pkgWithoutAlias, prefix)
}

func trimPackageAlias(pkg string) string {
	if values := strings.Split(pkg, " "); len(values) > 1 {
		pkg = values[1]
	}

	return strings.Trim(pkg, `"`)
}

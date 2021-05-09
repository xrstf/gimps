package gimps

import (
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
	"github.com/incu6us/goimports-reviser/v2/pkg/std"
)

type importSet []string

const (
	SetStd      = "std"
	SetProject  = "project"
	SetExternal = "external"
)

type Classifier struct {
	projectName string
	sets        []Set
}

type Set struct {
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"`
}

func NewClassifier(projectName string, sets []Set) *Classifier {
	return &Classifier{
		projectName: projectName,
		sets:        sets,
	}
}

func (c *Classifier) ClassifyImport(pkg string) string {
	if _, ok := std.StdPackages[pkg]; ok {
		return SetStd
	}

	for _, set := range c.sets {
		for _, pattern := range set.Patterns {
			if matches, _ := doublestar.Match(pattern, pkg); matches {
				return set.Name
			}
		}
	}

	if c.IsProjectImport(pkg) {
		return SetProject
	}

	return SetExternal
}

func (c *Classifier) IsProjectImport(pkg string) bool {
	return pkg == c.projectName || strings.HasPrefix(pkg, c.projectName+"/")
}

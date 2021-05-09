package gimps

import (
	"fmt"
	"path/filepath"

	"github.com/incu6us/goimports-reviser/v2/pkg/astutil"
)

// DependencyCache holds a list of package names per
// build tags (i.e. the key is the build tags string,
// like "windows prod").
type DependencyCache map[string]astutil.PackageImports

func NewDependencyCache() DependencyCache {
	return make(DependencyCache)
}

func (c DependencyCache) GetPackageName(filePath string, buildTags string, packagePath string) (string, error) {
	if _, ok := c[buildTags]; !ok {
		c[buildTags] = astutil.PackageImports{}
	}

	packageName, ok := c[buildTags][packagePath]
	if !ok {
		packageNames, err := astutil.LoadPackageDependencies(filepath.Dir(filePath), buildTags)
		if err != nil {
			return "", fmt.Errorf("failed to load package dependencies: %v", err)
		}

		c[buildTags] = packageNames
		packageName, ok = c[buildTags][packagePath]
	}

	if !ok {
		return "", fmt.Errorf("package %q is not a dependency of %q", packagePath, filepath.Dir(filePath))
	}

	return packageName, nil
}

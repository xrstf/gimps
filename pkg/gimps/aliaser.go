// SPDX-FileCopyrightText: 2023 Christoph Mewes
// SPDX-License-Identifier: MIT

package gimps

import (
	"fmt"
	"go/ast"
	"regexp"

	"github.com/incu6us/goimports-reviser/v3/pkg/astutil"
)

type Aliaser struct {
	projectName string
	rules       []AliasRule
	deps        DependencyCache
}

type AliasRule struct {
	Name       string         `yaml:"name"`
	Expression string         `yaml:"expr"`
	regexp     *regexp.Regexp `yaml:"-"`
	Alias      string         `yaml:"alias"`
}

func NewAliaser(projectName string, rules []AliasRule) (*Aliaser, error) {
	for i, rule := range rules {
		expr, err := regexp.Compile(rule.Expression)
		if err != nil {
			return nil, fmt.Errorf("invalid expression in rule %d: %v", i+1, err)
		}

		rules[i].regexp = expr
	}

	return &Aliaser{
		projectName: projectName,
		rules:       rules,
		deps:        NewDependencyCache(),
	}, nil
}

var validAlias = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func (a *Aliaser) RewriteFile(file *ast.File, filePath string, imports map[string]*importMetadata) error {
	// do not waste time loading package dependencies
	if len(a.rules) == 0 {
		return nil
	}

	// map of old package name (can be alias) to new alias
	aliasRenames := map[string]string{}

	buildTags := astutil.ParseBuildTag(file)

	// process each of the file's imports
	for imprt, metadata := range imports {
		// find the first rule that applies
		var rule *AliasRule
		for i, r := range a.rules {
			if r.regexp.MatchString(metadata.Package) {
				rule = &a.rules[i]
				break
			}
		}

		// no rule matched
		if rule == nil {
			continue
		}

		// cannot rewrite dot imports, because without parsing the whole
		// package, we cannot know what identifier resolves to the dot-imported
		// package
		oldAlias := metadata.Alias
		if oldAlias == "." {
			return fmt.Errorf("cannot rewrite, import %q matches rule %s but is a dot-import; dot-imports cannot be rewritten", imprt, rule.Name)
		}

		// if there was no alias, the package was properly referred to by its name
		if oldAlias == "" {
			// determine the actual names of packages, e.g. resolve
			// "github.com/bmatcuk/doublestar/v4" to "doublestar";
			// packageNames is a map of [path]=>[name]

			var err error
			oldAlias, err = a.deps.GetPackageName(filePath, buildTags, metadata.Package)
			if err != nil {
				return fmt.Errorf("invalid state: file imports %q, but: %v", metadata.Package, err)
			}
		}

		// generate new alias
		newAlias := rule.regexp.ReplaceAllString(metadata.Package, rule.Alias)
		if newAlias == "" {
			return fmt.Errorf("applying rule %s to %q leads to an empty alias", rule.Name, metadata.Package)
		}

		if !validAlias.MatchString(newAlias) {
			return fmt.Errorf("rule %s generated an invalid alias %q for package %q", rule.Name, newAlias, metadata.Package)
		}

		if oldAlias == newAlias {
			continue
		}

		aliasRenames[oldAlias] = newAlias

		// make sure whoever uses the import metadata from now on has the new aliases
		imports[imprt].Alias = newAlias
	}

	// nothing to do, the ideal situation
	if len(aliasRenames) == 0 {
		return nil
	}

	/*
		We can thankfully rely on package names being unique, i.e.
		it's impossible to import both core/v1 and networking/v1
		without already having aliased one of the two. This guarantees
		that there are no conflicts for the left side of aliasRenames.

		packageNames = {
			fmt: fmt,
			log: log,
			k8s.io/api/core/v1: v1,
			k8s.io/api/networking/v1beta1: v1beta1,
		}

		Note that the left side is either the alias (if already set)
		or the original package name.

		aliasRenames = {
			v1: corev1,
			v1beta1: networkingv1beta1,
		}

		Final set of names that would be in use:

		finalNames = [fmt, log, v1, v1beta1]
	*/

	// ensure renames are unique
	values := map[string]string{}
	for from, to := range aliasRenames {
		if oldFrom, exists := values[to]; exists {
			return aliasError(oldFrom, from, to)
		}

		values[to] = from
	}

	// ensure introducing new aliases doesn't lead to naming conflicts with
	// existing, un-aliased imports
	values = map[string]string{}
	for _, metadata := range imports {
		effectiveName := metadata.Alias
		if effectiveName == "." {
			continue
		}

		if effectiveName == "" {
			effectiveName, _ = a.deps.GetPackageName(filePath, buildTags, metadata.Package)
		}

		if otherPackage, exists := values[effectiveName]; exists {
			return aliasError(metadata.Package, otherPackage, effectiveName)
		}

		values[effectiveName] = metadata.Package
	}

	// update keys in metadata map
	newMap := map[string]*importMetadata{}
	for oldKey, metadata := range imports {
		newMap[metadata.Statement()] = metadata
		delete(imports, oldKey)
	}
	for k, v := range newMap {
		imports[k] = v
	}

	// with the rename map ready, it's now time to walk the file and replace old aliases
	ast.Walk(visitFn(func(node ast.Node) {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}

		localIdentifier := ident.Name

		newName, ok := aliasRenames[localIdentifier]
		if !ok {
			return
		}

		ident.Name = newName
	}), file)

	return nil
}

// aliasError is just to make sure the order of both mentioned packages
// is stable, so that tests can rely on it
func aliasError(pkga string, pkgb string, alias string) error {
	if pkgb < pkga {
		pkga, pkgb = pkgb, pkga
	}

	return fmt.Errorf("two or more packages (at least %q and %q) would be aliased to %q", pkga, pkgb, alias)
}

type visitFn func(node ast.Node)

func (f visitFn) Visit(node ast.Node) ast.Visitor {
	f(node)
	return f
}

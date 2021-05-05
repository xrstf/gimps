package main

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
)

// listFiles takes a filename or directory as its start argument and returns
// a list of absolute file paths. If a filename is given, the list contains
// exactly one element, otherwise the directory is scanned recursively.
// Note that if start is a file, the skip rules are not evaluated. This allows
// users to force-format an otherwise skipped file.
func listFiles(start string, moduleRoot string, skips []string) ([]string, error) {
	result := []string{}

	info, err := os.Stat(start)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{start}, nil
	}

	err = filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		relPath, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}

		if isSkipped(relPath, skips) {
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			result = append(result, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func isSkipped(relPath string, skips []string) bool {
	for _, skip := range skips {
		if match, _ := doublestar.Match(skip, relPath); match {
			return true
		}
	}

	return false
}

func goModRootPath(path string) (string, error) {
	// turn path into directory, if it's a file
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}

	for {
		if fi, err := os.Stat(filepath.Join(path, "go.mod")); err == nil && !fi.IsDir() {
			return path, nil
		}

		d := filepath.Dir(path)
		if d == path {
			break
		}

		path = d
	}

	return "", errors.New("no go.mod found")
}

var (
	// detect generated files by presence if this string in the first non-stripped line
	generatedRe = regexp.MustCompile("(been generated|generated by|do not edit)")
)

func isGeneratedFile(filename string) (bool, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}

	return isGeneratedCode(content)
}

func isGeneratedCode(sourceCode []byte) (bool, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "", sourceCode, parser.ParseComments)
	if err != nil {
		return false, err
	}

	// go through all comments until we reach the package declaration
outer:
	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			// found the package declaration
			if comment.Slash > file.Package {
				break outer
			}

			text := []byte(strings.ToLower(comment.Text))
			if generatedRe.Match(text) {
				return true, nil
			}
		}
	}

	return false, nil
}

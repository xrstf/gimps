package gimps

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"github.com/incu6us/goimports-reviser/v2/pkg/astutil"
)

// Execute is for revise imports and format the code
func Execute(config *Config, filePath string) ([]byte, bool, error) {
	setDefaults(config)

	originalContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, false, err
	}

	fset := token.NewFileSet()

	pf, err := parser.ParseFile(fset, "", originalContent, parser.ParseComments)
	if err != nil {
		return nil, false, err
	}

	importsWithMetadata, err := parseImports(config, pf, filePath)
	if err != nil {
		return nil, false, err
	}

	importSets := groupImports(
		config,
		importsWithMetadata,
	)

	if decls, hadMultipleDecls := mergeMultipleImportDecls(pf); hadMultipleDecls {
		pf.Decls = decls
	}

	fixImports(pf, importSets, importsWithMetadata)
	formatDecls(config, pf)

	fixedImportsContent, err := generateFile(fset, pf)
	if err != nil {
		return nil, false, err
	}

	formattedContent, err := format.Source(fixedImportsContent)
	if err != nil {
		return nil, false, err
	}

	return formattedContent, !bytes.Equal(originalContent, formattedContent), nil
}

func formatDecls(config *Config, f *ast.File) {
	for _, decl := range f.Decls {
		if dd, ok := decl.(*ast.FuncDecl); ok {
			var formattedComments []*ast.Comment
			if dd.Doc != nil {
				formattedComments = make([]*ast.Comment, len(dd.Doc.List))
			}

			formattedDoc := &ast.CommentGroup{
				List: formattedComments,
			}

			if dd.Doc != nil {
				for i, comment := range dd.Doc.List {
					formattedDoc.List[i] = comment
				}
			}

			dd.Doc = formattedDoc
		}
	}
}

// groupImports takes all the imports of a file and matches them against
// the configured classification rules. It then returns a list of import
// sets.
func groupImports(
	config *Config,
	importsWithMetadata map[string]*commentsMetadata,
) []importSet {
	sets := map[string]importSet{}

	for imprt := range importsWithMetadata {
		setName := config.classifyImport(imprt)

		if _, ok := sets[setName]; !ok {
			sets[setName] = importSet{}
		}
		sets[setName] = append(sets[setName], imprt)
	}

	result := []importSet{}

	for _, setName := range config.ImportOrder {
		if set, ok := sets[setName]; ok {
			sort.Strings(set)
			result = append(result, set)
		}
	}

	return result
}

func generateFile(fset *token.FileSet, f *ast.File) ([]byte, error) {
	var output []byte
	buffer := bytes.NewBuffer(output)
	if err := printer.Fprint(buffer, fset, f); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func fixImports(
	f *ast.File,
	importSets []importSet,
	commentsMetadata map[string]*commentsMetadata,
) {
	var importsPositions []*importPosition
	for _, decl := range f.Decls {
		dd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		if dd.Tok != token.IMPORT {
			continue
		}

		importsPositions = append(
			importsPositions, &importPosition{
				Start: dd.Pos(),
				End:   dd.End(),
			},
		)

		dd.Specs = rebuildImports(dd.Tok, commentsMetadata, importSets)
	}

	clearImportDocs(f, importsPositions)
	removeEmptyImportNode(f)
}

// mergeMultipleImportDecls will return combined import declarations to single declaration
//
// Ex.:
// import "fmt"
// import "io"
// -----
// to
// -----
// import (
// 	"fmt"
//	"io"
// )
func mergeMultipleImportDecls(f *ast.File) ([]ast.Decl, bool) {
	importSpecs := make([]ast.Spec, 0, len(f.Imports))
	for _, importSpec := range f.Imports {
		importSpecs = append(importSpecs, importSpec)
	}

	var (
		mergedMultipleImportDecls bool
		isFirstImportDeclDefined  bool
	)

	decls := make([]ast.Decl, 0, len(f.Decls))
	for _, decl := range f.Decls {
		dd, ok := decl.(*ast.GenDecl)
		if !ok {
			decls = append(decls, decl)
			continue
		}

		if dd.Tok != token.IMPORT {
			decls = append(decls, dd)
			continue
		}

		if isFirstImportDeclDefined {
			mergedMultipleImportDecls = true
			storedGenDecl := decls[len(decls)-1].(*ast.GenDecl)
			if storedGenDecl.Tok == token.IMPORT {
				storedGenDecl.Rparen = dd.End()
			}
			continue
		}

		dd.Specs = importSpecs
		decls = append(decls, dd)
		isFirstImportDeclDefined = true
	}

	return decls, mergedMultipleImportDecls
}

func removeEmptyImportNode(f *ast.File) {
	var (
		decls      []ast.Decl
		hasImports bool
	)

	for _, decl := range f.Decls {
		dd, ok := decl.(*ast.GenDecl)
		if !ok {
			decls = append(decls, decl)

			continue
		}

		if dd.Tok == token.IMPORT && len(dd.Specs) > 0 {
			hasImports = true

			break
		}

		if dd.Tok != token.IMPORT {
			decls = append(decls, decl)
		}
	}

	if !hasImports {
		f.Decls = decls
	}
}

func rebuildImports(
	tok token.Token,
	commentsMetadata map[string]*commentsMetadata,
	importSets []importSet,
) []ast.Spec {
	// convert each set into a list of ImportSpec declarations
	importSpecSets := [][]ast.Spec{}
	for _, set := range importSets {
		importSpecs := []ast.Spec{}

		for _, imprt := range set {
			importSpecs = append(importSpecs, &ast.ImportSpec{
				Path: &ast.BasicLit{Value: importWithComment(imprt, commentsMetadata), Kind: tok},
			})
		}

		importSpecSets = append(importSpecSets, importSpecs)
	}

	// merge them all together while inserting empty lines between the sets
	specs := []ast.Spec{}
	for i, importSpecs := range importSpecSets {
		specs = append(specs, importSpecs...)

		if i < len(importSpecSets)-1 {
			specs = append(specs, &ast.ImportSpec{Path: &ast.BasicLit{Value: "", Kind: token.STRING}})
		}
	}

	return specs
}

func clearImportDocs(f *ast.File, importsPositions []*importPosition) {
	importsComments := make([]*ast.CommentGroup, 0, len(f.Comments))

	for _, comment := range f.Comments {
		for _, importPosition := range importsPositions {
			if importPosition.IsInRange(comment) {
				continue
			}
			importsComments = append(importsComments, comment)
		}
	}

	if len(f.Imports) > 0 {
		f.Comments = importsComments
	}
}

func importWithComment(imprt string, commentsMetadata map[string]*commentsMetadata) string {
	var comment string
	commentGroup, ok := commentsMetadata[imprt]
	if ok {
		if commentGroup != nil && commentGroup.Comment != nil && len(commentGroup.Comment.List) > 0 {
			comment = fmt.Sprintf("// %s", strings.ReplaceAll(commentGroup.Comment.Text(), "\n", ""))
		}
	}

	return fmt.Sprintf("%s %s", imprt, comment)
}

func parseImports(config *Config, f *ast.File, filePath string) (map[string]*commentsMetadata, error) {
	importsWithMetadata := map[string]*commentsMetadata{}

	var packageImports map[string]string
	var err error

	if *config.RemoveUnusedImports || *config.SetVersionAlias {
		packageImports, err = astutil.LoadPackageDependencies(path.Dir(filePath), astutil.ParseBuildTag(f))
		if err != nil {
			return nil, err
		}
	}

	for _, decl := range f.Decls {
		switch decl.(type) {
		case *ast.GenDecl:
			dd := decl.(*ast.GenDecl)
			if dd.Tok == token.IMPORT {
				for _, spec := range dd.Specs {
					var importSpecStr string
					importSpec := spec.(*ast.ImportSpec)

					if *config.RemoveUnusedImports && !astutil.UsesImport(
						f, packageImports, strings.Trim(importSpec.Path.Value, `"`),
					) {
						continue
					}

					if importSpec.Name != nil {
						importSpecStr = strings.Join([]string{importSpec.Name.String(), importSpec.Path.Value}, " ")
					} else {
						if *config.SetVersionAlias {
							importSpecStr = setAliasForVersionedImportSpec(importSpec, packageImports)
						} else {
							importSpecStr = importSpec.Path.Value
						}
					}

					importsWithMetadata[importSpecStr] = &commentsMetadata{
						Doc:     importSpec.Doc,
						Comment: importSpec.Comment,
					}
				}
			}
		}
	}

	return importsWithMetadata, nil
}

func setAliasForVersionedImportSpec(importSpec *ast.ImportSpec, packageImports map[string]string) string {
	var importSpecStr string

	imprt := strings.Trim(importSpec.Path.Value, `"`)
	aliasName := packageImports[imprt]

	importSuffix := path.Base(imprt)
	if importSuffix != aliasName {
		importSpecStr = fmt.Sprintf("%s %s", aliasName, importSpec.Path.Value)
	} else {
		importSpecStr = importSpec.Path.Value
	}

	return importSpecStr
}

type commentsMetadata struct {
	Doc     *ast.CommentGroup
	Comment *ast.CommentGroup
}

type importPosition struct {
	Start token.Pos
	End   token.Pos
}

func (p *importPosition) IsInRange(comment *ast.CommentGroup) bool {
	if p.Start <= comment.Pos() && comment.Pos() <= p.End {
		return true
	}

	return false
}

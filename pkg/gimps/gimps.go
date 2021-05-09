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
	"sort"
	"strings"
)

type GimpsString string

type importMetadata struct {
	Doc     *ast.CommentGroup
	Comment *ast.CommentGroup
	Alias   string
	Package string
}

func (m *importMetadata) Statement() string {
	statement := fmt.Sprintf(`"%s"`, m.Package)
	if m.Alias != "" {
		statement = fmt.Sprintf("%s %s", m.Alias, statement)
	}

	return statement
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

// Execute is for revise imports and format the code
func Execute(config *Config, filePath string, aliaser *Aliaser) ([]byte, bool, error) {
	setDefaults(config)

	originalContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, false, err
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "", originalContent, parser.ParseComments)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse file: %v", err)
	}

	// determine the imports used in the file
	imports, err := parseImports(file, filePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse imports: %v", err)
	}

	// re-calculate aliases early, but only spend the effort if some rules
	// are configured
	if aliaser != nil {
		err = aliaser.RewriteFile(file, filePath, imports)
		if err != nil {
			return nil, false, fmt.Errorf("failed to rewrite import aliases: %v", err)
		}
	}

	// apply classification rules to group the imports into sets
	importSets := groupImports(config, imports)

	// merge import statements into a single one
	combineImportDecls(file)

	// rebuild/regroup imported packages
	fixImports(file, importSets, imports)

	// in case the source file actually had a single empty import statement
	removeEmptyImportNode(file)

	fixedImportsContent, err := generateFile(fset, file)
	if err != nil {
		return nil, false, fmt.Errorf("failed to generate code: %v", err)
	}

	formattedContent, err := format.Source(fixedImportsContent)
	if err != nil {
		return nil, false, fmt.Errorf("failed to format code: %v", err)
	}

	return formattedContent, !bytes.Equal(originalContent, formattedContent), nil
}

func parseImports(file *ast.File, filePath string) (map[string]*importMetadata, error) {
	metadata := map[string]*importMetadata{}

	for _, importDecl := range getImportDecls(file) {
		for _, spec := range importDecl.Specs {
			importSpec := spec.(*ast.ImportSpec)
			key := importSpec.Path.Value
			pkg := strings.Trim(key, `"`)
			alias := ""

			// prepend alias if set
			if importSpec.Name != nil {
				alias = importSpec.Name.String()
				key = fmt.Sprintf("%s %s", alias, importSpec.Path.Value)
			}

			// key is a quoted string, like `"fmt"` or `yaml "gopkg.in/yaml.v3"`
			metadata[key] = &importMetadata{
				Doc:     importSpec.Doc,
				Comment: importSpec.Comment,
				Package: pkg,
				Alias:   alias,
			}
		}
	}

	return metadata, nil
}

// groupImports takes all the imports of a file and matches them against
// the configured classification rules. It then returns a list of import
// sets.
func groupImports(config *Config, imports map[string]*importMetadata) []importSet {
	sets := map[string]importSet{}
	classifier := NewClassifier(config.ProjectName, config.Sets)

	for imprt, metadata := range imports {
		setName := classifier.ClassifyImport(metadata.Package)

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

// combineImportDecls will return combined import declarations to single declaration
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
func combineImportDecls(file *ast.File) []ast.Decl {
	// convert _all_ imports into a set of ast.Spec
	importSpecs := make([]ast.Spec, len(file.Imports))
	for i, importSpec := range file.Imports {
		importSpecs[i] = importSpec
	}

	var combinedImportDecl *ast.GenDecl

	// walk through all declarations
	decls := make([]ast.Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		// keep non-generic / non-import declarations as-is
		genericDecl, ok := decl.(*ast.GenDecl)
		if !ok || genericDecl.Tok != token.IMPORT {
			decls = append(decls, decl)
			continue
		}

		// we already have an import statement, add the current
		// one to it
		if combinedImportDecl != nil {
			combinedImportDecl.Rparen = genericDecl.End()
			continue
		}

		// we found the first import statement
		combinedImportDecl = genericDecl
		combinedImportDecl.Specs = importSpecs
		decls = append(decls, combinedImportDecl)
	}

	// update file
	file.Decls = decls

	return decls
}

// fixImports rebuilds the import statement and fixes the associated comments.
func fixImports(file *ast.File, importSets []importSet, imports map[string]*importMetadata) {
	var importPositions []*importPosition

	// there should only ever be a single import statement at this point,
	// because we combined them earlier
	for _, importDecl := range getImportDecls(file) {
		importPositions = append(
			importPositions, &importPosition{
				Start: importDecl.Pos(),
				End:   importDecl.End(),
			},
		)

		importDecl.Specs = rebuildImports(importDecl.Tok, imports, importSets)
	}

	clearImportDocs(file, importPositions)
}

// rebuildImports takes the already grouped importSets plus the metadata and
// generates a list of ast.Spec statements, representing the contents (Specs)
// of the file's new import statement.
func rebuildImports(tok token.Token, imports map[string]*importMetadata, importSets []importSet) []ast.Spec {
	// convert each set into a list of ImportSpec declarations
	importSpecSets := [][]ast.Spec{}
	for _, set := range importSets {
		importSpecs := []ast.Spec{}

		for _, imprt := range set {
			importSpecs = append(importSpecs, &ast.ImportSpec{
				Path: &ast.BasicLit{Value: importWithComment(imprt, imports), Kind: tok},
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

// ???
func clearImportDocs(file *ast.File, importPositions []*importPosition) {
	importsComments := make([]*ast.CommentGroup, 0, len(file.Comments))

	for _, comment := range file.Comments {
		for _, importPosition := range importPositions {
			if importPosition.IsInRange(comment) {
				continue
			}
			importsComments = append(importsComments, comment)
		}
	}

	if len(file.Imports) > 0 {
		file.Comments = importsComments
	}
}

// removeEmptyImportNode removes a single empty import node. This
// occurs if the original input file already had no imports, but
// a "import ()" statement. This function relies on all imports
// already being combined into a single statement.
func removeEmptyImportNode(file *ast.File) {
	var decls []ast.Decl

	for _, decl := range file.Decls {
		// collect all non-generic and non-import declarations
		genericDecl, ok := decl.(*ast.GenDecl)
		if !ok || genericDecl.Tok != token.IMPORT {
			decls = append(decls, decl)
			continue
		}

		// the import declaration is not empty, nothing to do
		if len(genericDecl.Specs) > 0 {
			return // exit early
		}
	}

	// there was no non-empty import statement, but possibly
	// an empty one; the empty one is not part of `decls` and
	// so we now override the file's content
	file.Decls = decls
}

// importWithComment appends a possible comment to the import statement,
// i.e. turning `foo "gopkg.in/foo/v2"` into `foo "gopkg.in/foo/v2" // my comment`
func importWithComment(imprt string, imports map[string]*importMetadata) string {
	var comment string
	if metadata, ok := imports[imprt]; ok && metadata.Comment != nil && len(metadata.Comment.List) > 0 {
		// TODO: use TrimSpace() ? Can this be a multiline comment?
		comment = fmt.Sprintf("// %s", strings.ReplaceAll(metadata.Comment.Text(), "\n", ""))
	}

	return strings.TrimSpace(fmt.Sprintf("%s %s", imprt, comment))
}

// generateFile creates Go source code for the given token set and file.
func generateFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var output []byte
	buffer := bytes.NewBuffer(output)
	if err := printer.Fprint(buffer, fset, file); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// getImportDecls returns all generic declarations with Tok==token.IMPORT
func getImportDecls(file *ast.File) []*ast.GenDecl {
	var result []*ast.GenDecl
	for _, decl := range file.Decls {
		genericDecl, ok := decl.(*ast.GenDecl)
		if !ok || genericDecl.Tok != token.IMPORT {
			continue
		}

		result = append(result, genericDecl)
	}

	return result
}

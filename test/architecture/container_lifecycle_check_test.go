package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Symbols use import paths, not source aliases. This is a convention check, not
// a control-flow proof: it follows named function references, but not interface
// dispatch, reflection, or arbitrary higher-order cleanup construction.
type symbol struct {
	pkg  string
	name string
}

type sourceFile struct {
	file *ast.File
	name string
	pkg  string

	uses map[*ast.Ident]types.Object
}

type declaration struct {
	file *sourceFile
	body ast.Node
}

type fixtureRule struct {
	name    string
	entries []symbol
	cleanup symbol
	display string
}

func lifecycleFixtureRules(modulePath string) []fixtureRule {
	return []fixtureRule{
		{
			name: "PostgreSQL",
			entries: []symbol{
				{modulePath + "/internal/testutil", "NewTestDB"},
				{modulePath + "/internal/testutil", "NewPostgresTestDB"},
				{modulePath + "/internal/testutil", "PostgresTestDSN"},
			},
			cleanup: symbol{modulePath + "/internal/testutil", "TerminateSharedPostgres"},
			display: "testutil.TerminateSharedPostgres",
		},
		{
			name:    "SpiceDB",
			entries: []symbol{{modulePath + "/internal/testutil/relations", "New"}},
			cleanup: symbol{modulePath + "/internal/testutil/relations", "TerminateSharedSpiceDB"},
			display: "relationstest.TerminateSharedSpiceDB",
		},
	}
}

type lifecycleException struct {
	file, declaration, fixture, reason string
}

// These declarations deliberately own cleanup or reject setup before any
// container starts. Match exact declarations, never an entire package.
var lifecycleExceptions = []lifecycleException{
	{
		file: "internal/testutil/postgres_testdb_test.go", declaration: "TestPostgresDebugLifecycle", fixture: "PostgreSQL",
		reason: "launches the retention subprocess and removes its retained container explicitly",
	},
	{
		file: "internal/testutil/postgres_testdb_test.go", declaration: "runPostgresDebugChild", fixture: "PostgreSQL",
		reason: "subprocess registers TerminateSharedPostgres with t.Cleanup to test retention",
	},
	{
		file: "internal/testutil/testdb_test.go", declaration: "TestTestDBBackend_Invalid", fixture: "PostgreSQL",
		reason: "invalid TEST_DB must panic before fixture setup, so no container is started",
	},
}

func checkContainerLifecycle(tree fs.FS, exceptions ...lifecycleException) ([]string, error) {
	modulePath, err := lifecycleRepositoryModule(tree)
	if err != nil {
		return nil, err
	}
	fixtureRules := lifecycleFixtureRules(modulePath)
	runner := symbol{modulePath + "/internal/testutil/testlifecycle", "Run"}
	usedExceptions := make([]bool, len(exceptions))
	for _, exception := range exceptions {
		if strings.TrimSpace(exception.reason) == "" {
			return nil, fmt.Errorf("lifecycle exception %s:%s requires a reason", exception.file, exception.declaration)
		}
	}
	fset := token.NewFileSet()
	var files []*sourceFile
	packageNames := map[string]string{}
	err = fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata" || entry.Name() == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		content, err := fs.ReadFile(tree, name)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(fset, name, content, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		pkg := modulePath
		if directory := path.Dir(name); directory != "." {
			pkg += "/" + directory
		}
		if strings.HasSuffix(parsed.Name.Name, "_test") {
			pkg += "_test"
		} else {
			packageNames[pkg] = parsed.Name.Name
		}
		files = append(files, &sourceFile{file: parsed, name: name, pkg: pkg})
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The fixture package's declared name differs from its path basename, even
	// in synthetic regression inputs that do not include its implementation.
	packageNames[modulePath+"/internal/testutil/relations"] = "relationstest"
	// Resolve lexical bindings without loading/building external dependencies.
	// Imported packages need only names and paths: selector targets are matched
	// against the source graph below, not type-checked as external API calls.
	// Incomplete imported types cause expected type errors, which are ignored;
	// normal compilation remains responsible for validating the Go code.
	packageFiles := map[string][]*sourceFile{}
	for _, file := range files {
		packageFiles[file.pkg] = append(packageFiles[file.pkg], file)
	}
	for pkg, sources := range packageFiles {
		info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
		var syntax []*ast.File
		for _, source := range sources {
			syntax = append(syntax, source.file)
			source.uses = info.Uses
		}
		config := types.Config{Importer: nameImporter{packageNames}, Error: func(error) {}}
		_, _ = config.Check(pkg, fset, syntax, info)
	}
	declarations := map[symbol]declaration{}
	var testRoots []symbol
	mains := map[string][]symbol{}
	for _, file := range files {
		for _, spec := range file.file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, err
			}
			if spec.Name != nil && spec.Name.Name == "." {
				if importPath == modulePath+"/internal/testutil" || importPath == modulePath+"/internal/testutil/relations" || importPath == modulePath+"/internal/testutil/testlifecycle" {
					return nil, fmt.Errorf("%s: use a named import for %s so the lifecycle check can resolve fixtures and cleanups; see %s", fset.Position(spec.Pos()), importPath, lifecycleDocLink)
				}
				continue
			}
		}
		add := func(name string, body ast.Node) {
			id := symbol{file.pkg, name}
			declarations[id] = declaration{file, body}
			if strings.HasSuffix(file.name, "_test.go") {
				testRoots = append(testRoots, id)
			}
		}
		for _, decl := range file.file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				name := decl.Name.Name
				if decl.Recv != nil || name == "init" {
					// Methods are roots too; their bodies may directly acquire fixtures.
					// Position-qualified identities also keep multiple init functions distinct.
					name = fmt.Sprintf("%s@%d", name, decl.Pos())
				}
				if decl.Body != nil {
					add(name, decl.Body)
				}
				if decl.Recv == nil && name == "TestMain" {
					mains[path.Dir(file.name)] = append(mains[path.Dir(file.name)], symbol{file.pkg, name})
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if value, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range value.Names {
							add(name.Name, value)
						}
					}
				}
			}
		}
	}

	// Gather fixture requirements across local and imported helper functions.
	// Inspecting function references also catches passing a fixture as a value.
	references := map[symbol][]symbol{}
	for id, decl := range declarations {
		ast.Inspect(decl.body, func(node ast.Node) bool {
			if ref, ok := resolveSymbol(node, decl.file); ok {
				references[id] = append(references[id], ref)
			}
			return true
		})
	}
	type requirement struct {
		position token.Position
		rules    map[int]bool
	}
	required := map[string]*requirement{}
	for _, root := range testRoots {
		decl := declarations[root]
		directory := path.Dir(decl.file.name)

		visitSymbols(root, references, func(id symbol) {
			for i, rule := range fixtureRules {
				for _, entry := range rule.entries {
					if id == entry {
						exempt := false
						for j, exception := range exceptions {
							if decl.file.name == exception.file && root.name == exception.declaration && rule.name == exception.fixture {
								usedExceptions[j] = true
								exempt = true
							}
						}
						if exempt {
							continue
						}
						if required[directory] == nil {
							required[directory] = &requirement{fset.Position(decl.body.Pos()), map[int]bool{}}
						}
						required[directory].rules[i] = true
					}
				}
			}
		})
	}
	var problems []string
	for i, used := range usedExceptions {
		if !used {
			exception := exceptions[i]
			problems = append(problems, fmt.Sprintf("%s:%s: stale lifecycle exception for %s (%s); remove or update the exception because the declaration no longer requires it. See %s", exception.file, exception.declaration, exception.fixture, exception.reason, lifecycleDocLink))
		}
	}
	for directory, needed := range required {
		registered := map[symbol]bool{}
		for _, main := range mains[directory] {
			collectCleanups(main, runner, declarations, registered, map[symbol]bool{})
		}
		for i, rule := range fixtureRules {
			if needed.rules[i] && !registered[rule.cleanup] {
				var cleanups []string
				for j, requiredRule := range fixtureRules {
					if needed.rules[j] {
						cleanups = append(cleanups, requiredRule.display)
					}
				}
				problems = append(problems, fmt.Sprintf("%s: package %s uses shared %s fixtures but TestMain does not register %s with testlifecycle.Run.\nAdd or update main_test.go (import os, testing, and the helper packages):\n\nfunc TestMain(m *testing.M) {\n    os.Exit(testlifecycle.Run(m, %s))\n}\n\nSee %s for imports, examples, and cleanup requirements.", needed.position, directory, rule.name, rule.display, strings.Join(cleanups, ", "), lifecycleDocLink))
			}
		}
	}
	sort.Strings(problems)
	return problems, nil
}

func resolveSymbol(node ast.Node, file *sourceFile) (symbol, bool) {
	switch node := node.(type) {
	case *ast.SelectorExpr:
		if ident, ok := node.X.(*ast.Ident); ok {
			if pkg, ok := file.uses[ident].(*types.PkgName); ok {
				return symbol{pkg.Imported().Path(), node.Sel.Name}, true
			}
		}
	case *ast.Ident:
		object := file.uses[node]
		if object != nil && object.Pkg() != nil && object.Parent() == object.Pkg().Scope() {
			return symbol{object.Pkg().Path(), node.Name}, true
		}
	}
	return symbol{}, false
}

type nameImporter struct {
	names map[string]string
}

func (importer nameImporter) Import(importPath string) (*types.Package, error) {
	name := importer.names[importPath]
	if name == "" {
		name = path.Base(importPath)
	}
	pkg := types.NewPackage(importPath, name)
	pkg.MarkComplete()
	return pkg, nil
}

func visitSymbols(root symbol, references map[symbol][]symbol, visit func(symbol)) {
	seen := map[symbol]bool{}
	var walk func(symbol)
	walk = func(id symbol) {
		if seen[id] {
			return
		}
		seen[id] = true
		visit(id)
		for _, ref := range references[id] {
			walk(ref)
		}
	}
	walk(root)
}

func collectCleanups(id, runner symbol, declarations map[symbol]declaration, registered, seen map[symbol]bool) {
	if seen[id] {
		return
	}
	seen[id] = true
	decl, ok := declarations[id]
	if !ok {
		return
	}
	ast.Inspect(decl.body, func(node ast.Node) bool {
		// An unused closure containing Run is not a lifecycle registration.
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := resolveSymbol(call.Fun, decl.file)
		if !ok {
			return true
		}
		if callee == runner && len(call.Args) > 0 {
			for _, argument := range call.Args[1:] {
				if cleanup, ok := resolveSymbol(argument, decl.file); ok {
					registered[cleanup] = true
				}
			}
		} else {
			collectCleanups(callee, runner, declarations, registered, seen)
		}
		return true
	})
}

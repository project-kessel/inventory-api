package architecture

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

const testModulePath = "example.com/inventory"

func lifecycleTestFiles(sources map[string]string) fstest.MapFS {
	files := fstest.MapFS{
		"go.mod":         {Data: []byte("module " + testModulePath + "\n")},
		lifecycleDocPath: {Data: []byte(lifecycleDocHeading + "\n")},
	}
	for name, source := range sources {
		files[name] = &fstest.MapFile{Data: []byte(source)}
	}
	return files
}

// TestLifecycleExceptions verifies that exceptions cannot exempt new tests or callers.
func TestLifecycleExceptions(t *testing.T) {
	const filename = "internal/testutil/lifecycle_test.go"
	const fixtureSource = `package testutil_test
import db "example.com/inventory/internal/testutil"
func approved() { db.NewTestDB(nil, nil, nil) }
`
	exception := lifecycleException{
		file: filename, declaration: "approved", fixture: "PostgreSQL",
		reason: "isolated lifecycle regression owns its cleanup",
	}
	for _, test := range []struct {
		name, source, filename, want string
		useException                 bool
	}{
		{name: "exact declaration", source: fixtureSource, filename: filename, useException: true},
		{name: "package no longer exempt", source: fixtureSource, filename: filename, want: "uses shared PostgreSQL fixtures"},
		{name: "new declaration in same file", source: fixtureSource + "\nfunc TestNew() { db.NewTestDB(nil, nil, nil) }", filename: filename, useException: true, want: "uses shared PostgreSQL fixtures"},
		{name: "caller does not inherit exception", source: fixtureSource + "\nfunc TestCaller() { approved() }", filename: filename, useException: true, want: "uses shared PostgreSQL fixtures"},
		{name: "another fixture not exempt", source: strings.ReplaceAll(strings.ReplaceAll(fixtureSource, "import db", "import authz \"example.com/inventory/internal/testutil/relations\"\nimport db"), "db.NewTestDB(nil, nil, nil)", "db.NewTestDB(nil, nil, nil); authz.New(nil)"), filename: filename, useException: true, want: "uses shared SpiceDB fixtures"},
		{name: "renamed declaration", source: strings.ReplaceAll(fixtureSource, "approved", "renamed"), filename: filename, useException: true, want: "stale lifecycle exception"},
		{name: "moved file", source: fixtureSource, filename: "internal/testutil/moved_test.go", useException: true, want: "stale lifecycle exception"},
		{name: "fixture removed", source: strings.ReplaceAll(fixtureSource, "db.NewTestDB(nil, nil, nil)", ""), filename: filename, useException: true, want: "stale lifecycle exception"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var exceptions []lifecycleException
			if test.useException {
				exceptions = append(exceptions, exception)
			}
			problems, err := checkContainerLifecycle(lifecycleTestFiles(map[string]string{test.filename: test.source}), exceptions...)
			require.NoError(t, err)
			if test.want == "" {
				require.Empty(t, problems)
				return
			}
			require.Contains(t, strings.Join(problems, "\n"), test.want)
		})
	}
	t.Run("reason required", func(t *testing.T) {
		exception.reason = ""
		_, err := checkContainerLifecycle(lifecycleTestFiles(map[string]string{filename: fixtureSource}), exception)
		require.ErrorContains(t, err, "requires a reason")
	})
}

// TestLifecycleModuleRename ensures a changed module cannot silently disable matching.
func TestLifecycleModuleRename(t *testing.T) {
	files := lifecycleTestFiles(map[string]string{
		"example/example_test.go": `package example
import db "example.com/inventory/internal/testutil"
func TestExample() { db.NewTestDB(nil, nil, nil) }`,
	})
	for _, file := range files {
		file.Data = []byte(strings.ReplaceAll(string(file.Data), testModulePath, "example.org/renamed"))
	}
	problems, err := checkContainerLifecycle(files)
	require.NoError(t, err)
	require.Len(t, problems, 1)
	require.Contains(t, problems[0], "uses shared PostgreSQL fixtures")

	files["example/main_test.go"] = &fstest.MapFile{Data: []byte(`package example
import (
    "os"
    "testing"
    db "example.org/renamed/internal/testutil"
    lifecycle "example.org/renamed/internal/testutil/testlifecycle"
)
func TestMain(m *testing.M) { os.Exit(lifecycle.Run(m, db.TerminateSharedPostgres)) }
`)}
	problems, err = checkContainerLifecycle(files)
	require.NoError(t, err)
	require.Empty(t, problems)
}

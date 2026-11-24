package architecture

import (
	"os"

	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSharedContainerLifecycle enforces cleanup ownership for shared fixtures.
func TestSharedContainerLifecycle(t *testing.T) {
	root, err := findLifecycleRepositoryRoot(".")
	require.NoError(t, err)
	problems, err := checkContainerLifecycle(os.DirFS(root), lifecycleExceptions...)
	require.NoError(t, err)
	for _, problem := range problems {
		t.Error(problem)
	}
}

// TestContainerLifecycleCheck exercises the guard without starting containers.
func TestContainerLifecycleCheck(t *testing.T) {
	postgresTest := `package example_test
import (
    db "example.com/inventory/internal/testutil"
    "testing"
)
func fixture(t *testing.T) { db.NewTestDB(t, nil, nil) }
func TestExample(t *testing.T) { fixture(t) }
`
	postgresMain := `package example_test
import (
    "os"
    "testing"
    db "example.com/inventory/internal/testutil"
    lifecycle "example.com/inventory/internal/testutil/testlifecycle"
)
func TestMain(m *testing.M) { os.Exit(lifecycle.Run(m, db.TerminateSharedPostgres)) }
`
	spiceTest := `package example_test
import authz "example.com/inventory/internal/testutil/relations"
func spiceFixture() { authz.New(nil) }
`
	for _, test := range []struct {
		name    string
		files   map[string]string
		missing []string
	}{
		{
			name:    "local helper requires TestMain",
			files:   map[string]string{"example/example_test.go": postgresTest},
			missing: []string{"PostgreSQL"},
		},
		{
			name: "multiple init functions in one file retain fixture requirements",
			files: map[string]string{"example/example_test.go": `package example
import db "example.com/inventory/internal/testutil"
func init() { db.NewTestDB(nil, nil, nil) }
func init() {}`},
			missing: []string{"PostgreSQL"},
		},
		{
			name: "multiple init functions across files retain fixture requirements",
			files: map[string]string{
				"example/a_test.go": `package example
import db "example.com/inventory/internal/testutil"
func init() { db.NewTestDB(nil, nil, nil) }`,
				"example/z_test.go": `package example
func init() {}`,
			},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "both fixtures without TestMain",
			files:   map[string]string{"example/db_test.go": postgresTest, "example/spice_test.go": spiceTest},
			missing: []string{"PostgreSQL", "SpiceDB"},
		},
		{
			name:  "aliased imports and external test package",
			files: map[string]string{"example/example_test.go": postgresTest, "example/main_test.go": postgresMain},
		},
		{
			name: "TestMain in a non-test file does not count",
			files: map[string]string{
				"example/example_test.go": postgresTest,
				"example/lifecycle.go":    strings.ReplaceAll(postgresMain, "package example_test", "package example"),
			},
			missing: []string{"PostgreSQL"},
		},
		{
			name:  "TestMain can be in the internal test package",
			files: map[string]string{"example/example_test.go": postgresTest, "example/main_test.go": strings.ReplaceAll(postgresMain, "package example_test", "package example")},
		},
		{
			name: "unaliased relations package uses its declared name",
			files: map[string]string{"example/example_test.go": `package example
import "example.com/inventory/internal/testutil/relations"
func fixture() { relationstest.New(nil) }`},
			missing: []string{"SpiceDB"},
		},
		{
			name:    "wrong cleanup",
			files:   map[string]string{"example/example_test.go": spiceTest, "example/main_test.go": postgresMain},
			missing: []string{"SpiceDB"},
		},
		{
			name:    "both fixtures need both cleanups",
			files:   map[string]string{"example/db_test.go": postgresTest, "example/spice_test.go": spiceTest, "example/main_test.go": postgresMain},
			missing: []string{"SpiceDB"},
		},
		{
			name:  "both cleanups registered",
			files: map[string]string{"example/db_test.go": postgresTest, "example/spice_test.go": spiceTest, "example/main_test.go": strings.ReplaceAll(strings.ReplaceAll(postgresMain, `"os"`, `"os"; authz "example.com/inventory/internal/testutil/relations"`), "db.TerminateSharedPostgres)", "db.TerminateSharedPostgres, authz.TerminateSharedSpiceDB)")},
		},
		{
			name:    "unrelated lifecycle function does not count",
			files:   map[string]string{"example/example_test.go": postgresTest, "example/main_test.go": strings.ReplaceAll(postgresMain, "func TestMain", "func unused")},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "cleanup reference alone does not count",
			files:   map[string]string{"example/example_test.go": postgresTest, "example/main_test.go": strings.ReplaceAll(postgresMain, "os.Exit(lifecycle.Run(m, db.TerminateSharedPostgres))", "_ = db.TerminateSharedPostgres; os.Exit(m.Run())")},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "dormant closure does not count",
			files:   map[string]string{"example/example_test.go": postgresTest, "example/main_test.go": strings.ReplaceAll(postgresMain, "os.Exit(lifecycle.Run(m, db.TerminateSharedPostgres))", "unused := func() { lifecycle.Run(m, db.TerminateSharedPostgres) }; _ = unused; os.Exit(m.Run())")},
			missing: []string{"PostgreSQL"},
		},
		{
			name:  "package-local TestMain wrapper",
			files: map[string]string{"example/example_test.go": postgresTest, "example/main_test.go": strings.ReplaceAll(postgresMain, "func TestMain(m *testing.M) {", "func TestMain(m *testing.M) { run(m) }; func run(m *testing.M) {")},
		},
		{
			name: "shared imported fixture helper",
			files: map[string]string{
				"helper/fixture.go": strings.ReplaceAll(postgresTest, "package example_test", "package helper"),
				"example/example_test.go": `package example
import "example.com/inventory/helper"
func TestExample() { helper.TestExample(nil) }`,
			},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "fixture function value also requires cleanup",
			files:   map[string]string{"example/example_test.go": strings.ReplaceAll(postgresTest, "db.NewTestDB(t, nil, nil)", "newDB := db.NewTestDB; newDB(t, nil, nil)")},
			missing: []string{"PostgreSQL"},
		},
		{
			name: "package-level fixture function value",
			files: map[string]string{"example/example_test.go": `package example
import db "example.com/inventory/internal/testutil"
var newDB = db.NewTestDB
func fixture() { newDB(nil, nil, nil) }`},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "recursive helpers terminate",
			files:   map[string]string{"example/example_test.go": postgresTest + "\nfunc cycle() { cycle(); fixture(nil) }\n"},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "testutil convenience constructors",
			files:   map[string]string{"example/example_test.go": strings.ReplaceAll(postgresTest, "NewTestDB", "NewPostgresTestDB")},
			missing: []string{"PostgreSQL"},
		},
		{
			name:    "raw DSN fixture",
			files:   map[string]string{"example/example_test.go": strings.ReplaceAll(postgresTest, "NewTestDB", "PostgresTestDSN")},
			missing: []string{"PostgreSQL"},
		},
		{
			name: "no fixture no TestMain needed",
			files: map[string]string{"example/example_test.go": `package example
func TestExample() { s := "relationstest.New(t)"; _ = s }`},
		},
		{
			name: "unused imported helper does not require cleanup in caller",
			files: map[string]string{
				"helper/fixture.go": strings.ReplaceAll(postgresTest, "package example_test", "package helper") + "\nfunc Pure() {}\n",
				"example/example_test.go": `package example
import "example.com/inventory/helper"
func TestExample() { helper.Pure() }`,
			},
		},
		{
			name: "local function shadowing does not invoke the fixture helper",
			files: map[string]string{
				"helper/fixture.go": strings.ReplaceAll(postgresTest, "package example_test", "package helper") + "\nfunc Pure() { fixture := func(interface{}) {}; fixture(nil) }\n",
				"example/example_test.go": `package example
import "example.com/inventory/helper"
func TestExample() { helper.Pure() }`,
			},
		},
		{
			name:  "same named method is not an imported fixture",
			files: map[string]string{"example/example_test.go": strings.ReplaceAll(postgresTest, "db.NewTestDB(t, nil, nil)", "db := localClient{}; db.NewTestDB(t, nil, nil)")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := lifecycleTestFiles(test.files)
			problems, err := checkContainerLifecycle(files)
			require.NoError(t, err)
			require.Len(t, problems, len(test.missing))
			for i, missing := range test.missing {
				require.Contains(t, problems[i], "uses shared "+missing+" fixtures")
				require.Contains(t, problems[i], "func TestMain(m *testing.M)")
				require.Contains(t, problems[i], "os.Exit(testlifecycle.Run(m,")
				require.Contains(t, problems[i], "test/GUIDELINES.md#shared-container-lifecycle-required")
			}
		})
	}
}

// TestContainerLifecycleCheckErrors prevents unsupported input from silently passing.
func TestContainerLifecycleCheckErrors(t *testing.T) {
	for _, test := range []struct {
		name, source, want string
	}{
		{"syntax error", "package example; func {", "expected"},
		{"fixture dot import", `package example
import . "example.com/inventory/internal/testutil"
func TestExample() { NewTestDB(nil, nil, nil) }`, "use a named import"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := checkContainerLifecycle(lifecycleTestFiles(map[string]string{
				"example/example_test.go": test.source,
			}))
			require.ErrorContains(t, err, test.want)
		})
	}
}

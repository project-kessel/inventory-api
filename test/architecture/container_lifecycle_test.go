package architecture

import (
	"go/build"
	"os"

	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSharedContainerLifecycle enforces cleanup ownership for shared fixtures.
func TestSharedContainerLifecycle(t *testing.T) {
	root, err := findLifecycleRepositoryRoot(".")
	require.NoError(t, err)
	// build.Default does not inherit go test's -tags or -race flags. Check the
	// supported suites explicitly, without merging mutually exclusive files.
	var contexts []build.Context
	for _, tags := range [][]string{nil, {"integration"}, {"test"}, {"sanity"}, {"performance"}} {
		for _, race := range []bool{false, true} {
			context := build.Default
			context.BuildTags = append([]string(nil), tags...)
			if race {
				context.BuildTags = append(context.BuildTags, "race")
			}
			contexts = append(contexts, context)
		}
	}
	problems, err := checkContainerLifecycleBuilds(os.DirFS(root), contexts, lifecycleExceptions...)
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

// TestContainerLifecycleBuildConstraints keeps fixture and cleanup graphs isolated
// across build tags, platform suffixes, and excluded helper implementations.
func TestContainerLifecycleBuildConstraints(t *testing.T) {
	const fixture = `package example
import db "example.com/inventory/internal/testutil"
func TestExample() { db.NewTestDB(nil, nil, nil) }
`
	const main = `package example
import (
    "os"
    "testing"
    db "example.com/inventory/internal/testutil"
    lifecycle "example.com/inventory/internal/testutil/testlifecycle"
)
func TestMain(m *testing.M) { os.Exit(lifecycle.Run(m, db.TerminateSharedPostgres)) }
`
	for _, test := range []struct {
		name    string
		files   map[string]string
		tags    []string
		missing bool
	}{
		{
			name: "tagged main cannot clean up untagged fixture",
			files: map[string]string{
				"example/example_test.go": fixture,
				"example/main_test.go":    "//go:build integration\n\n" + main,
			},
			missing: true,
		},
		{
			name: "tagged main cleans up in matching context",
			files: map[string]string{
				"example/example_test.go": fixture,
				"example/main_test.go":    "//go:build integration\n\n" + main,
			},
			tags: []string{"integration"},
		},
		{
			name: "legacy constraint excludes main",
			files: map[string]string{
				"example/example_test.go": fixture,
				"example/main_test.go":    "// +build integration\n\n" + main,
			},
			missing: true,
		},
		{
			name: "inactive fixture does not require cleanup",
			files: map[string]string{
				"example/example_test.go": "//go:build integration\n\n" + fixture,
			},
		},
		{
			name: "active tagged fixture requires cleanup",
			files: map[string]string{
				"example/example_test.go": "//go:build integration\n\n" + fixture,
			},
			tags: []string{"integration"}, missing: true,
		},
		{
			name: "foreign OS main cannot supply cleanup",
			files: map[string]string{
				"example/example_test.go":      fixture,
				"example/main_windows_test.go": main,
			},
			missing: true,
		},
		{
			name: "foreign architecture main cannot supply cleanup",
			files: map[string]string{
				"example/example_test.go":    fixture,
				"example/main_arm64_test.go": main,
			},
			missing: true,
		},
		{
			name: "matching platform main supplies cleanup",
			files: map[string]string{
				"example/example_test.go":          fixture,
				"example/main_linux_amd64_test.go": main,
			},
		},
		{
			name: "excluded cleanup helper cannot overwrite active helper",
			files: map[string]string{
				"example/example_test.go": fixture,
				"example/main_test.go": strings.ReplaceAll(main,
					"os.Exit(lifecycle.Run(m, db.TerminateSharedPostgres))", "os.Exit(run(m))"),
				"example/a.go": "package example; func run(m *testing.M) int { return m.Run() }",
				"example/z.go": "//go:build integration\n\npackage example\n" +
					`import lifecycle "example.com/inventory/internal/testutil/testlifecycle"
import db "example.com/inventory/internal/testutil"
func run(m *testing.M) int { return lifecycle.Run(m, db.TerminateSharedPostgres) }`,
			},
			missing: true,
		},
		{
			name: "excluded fixture helper does not require cleanup",
			files: map[string]string{
				"example/example_test.go": "package example; func TestExample() { fixture() }",
				"example/a.go":            "//go:build !integration\n\npackage example; func fixture() {}",
				"example/z.go":            "//go:build integration\n\n" + strings.ReplaceAll(fixture, "TestExample", "fixture"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := build.Default
			context.GOOS, context.GOARCH = "linux", "amd64"
			context.BuildTags = test.tags
			problems, err := checkContainerLifecycleBuilds(lifecycleTestFiles(test.files), []build.Context{context})
			require.NoError(t, err)
			if test.missing {
				require.Len(t, problems, 1)
				require.Contains(t, problems[0], "uses shared PostgreSQL fixtures")
			} else {
				require.Empty(t, problems)
			}
		})
	}

	baseline := build.Default
	baseline.BuildTags = nil
	integration := baseline
	integration.BuildTags = []string{"integration"}
	contexts := []build.Context{baseline, integration}
	t.Run("cleanup registrations are not combined across contexts", func(t *testing.T) {
		problems, err := checkContainerLifecycleBuilds(lifecycleTestFiles(map[string]string{
			"example/example_test.go": fixture,
			"example/main_test.go":    "//go:build integration\n\n" + main,
		}), contexts)
		require.NoError(t, err)
		require.Len(t, problems, 1)
		require.Contains(t, problems[0], "tags=[]")
		require.Contains(t, problems[0], "uses shared PostgreSQL fixtures")
	})
	t.Run("exception usage is combined across contexts", func(t *testing.T) {
		files := lifecycleTestFiles(map[string]string{
			"example/example_test.go": "//go:build integration\n\n" + fixture,
		})
		exception := lifecycleException{
			file: "example/example_test.go", declaration: "TestExample", fixture: "PostgreSQL",
			reason: "isolated regression owns its cleanup",
		}
		problems, err := checkContainerLifecycleBuilds(files, contexts, exception)
		require.NoError(t, err)
		require.Empty(t, problems)

		files["example/example_test.go"].Data = []byte("//go:build integration\n\npackage example\nfunc TestExample() {}")
		problems, err = checkContainerLifecycleBuilds(files, contexts, exception)
		require.NoError(t, err)
		require.Len(t, problems, 1)
		require.Contains(t, problems[0], "stale lifecycle exception")
	})
}

// TestContainerLifecycleCheckErrors prevents unsupported input from silently passing.
func TestContainerLifecycleCheckErrors(t *testing.T) {
	for _, test := range []struct {
		name, source, want string
	}{
		{"syntax error", "package example; func {", "expected"},
		{"invalid build constraint", "//go:build integration &&\n\npackage example", "match build constraints"},
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

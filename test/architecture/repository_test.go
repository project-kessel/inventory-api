package architecture

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// TestLifecycleRepositoryMetadata verifies discovery fails closed on stale paths.
func TestLifecycleRepositoryMetadata(t *testing.T) {
	for _, test := range []struct {
		name, goMod, documentation, wantModule, wantError string
	}{
		{name: "module directive", goMod: "module example.com/renamed\n\ngo 1.26\n", documentation: lifecycleDocHeading, wantModule: "example.com/renamed"},
		{name: "quoted module with comment", goMod: "// module misleading.example/path\nmodule \"example.com/quoted\" // comment\n", documentation: lifecycleDocHeading, wantModule: "example.com/quoted"},
		{name: "missing go.mod", documentation: lifecycleDocHeading, wantError: "read repository go.mod"},
		{name: "missing module directive", goMod: "go 1.26\n", documentation: lifecycleDocHeading, wantError: "exactly one module directive"},
		{name: "duplicate module directive", goMod: "module example.com/one\nmodule example.com/two\n", documentation: lifecycleDocHeading, wantError: "exactly one module directive"},
		{name: "malformed module directive", goMod: "module\n", documentation: lifecycleDocHeading, wantError: "invalid module directive"},
		{name: "invalid module path", goMod: "module \"example.com/with space\"\n", documentation: lifecycleDocHeading, wantError: "invalid module path"},
		{name: "missing documentation", goMod: "module example.com/repo\n", wantError: "read lifecycle documentation"},
		{name: "missing section", goMod: "module example.com/repo\n", documentation: "### A renamed section\n", wantError: "missing heading"},
		{name: "heading inside example is insufficient", goMod: "module example.com/repo\n", documentation: "```markdown\n" + lifecycleDocHeading + "\n```\n", wantError: "missing heading"},
		{name: "indented code is not a heading", goMod: "module example.com/repo\n", documentation: "    " + lifecycleDocHeading + "\n", wantError: "missing heading"},
		{name: "heading after fenced example", goMod: "module example.com/repo\n", documentation: "~~~~markdown\n```\n~~~~\n" + lifecycleDocHeading + "\n", wantModule: "example.com/repo"},
		{name: "mention outside heading is insufficient", goMod: "module example.com/repo\n", documentation: "See " + lifecycleDocHeading + " for setup.\n", wantError: "missing heading"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := fstest.MapFS{}
			if test.goMod != "" {
				files["go.mod"] = &fstest.MapFile{Data: []byte(test.goMod)}
			}
			if test.documentation != "" {
				files[lifecycleDocPath] = &fstest.MapFile{Data: []byte(test.documentation)}
			}
			module, err := lifecycleRepositoryModule(files)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantModule, module)
		})
	}
}

// TestLifecycleDocumentationLink keeps the generated diagnostic anchor accurate.
func TestLifecycleDocumentationLink(t *testing.T) {
	require.Equal(t, "test/GUIDELINES.md#shared-container-lifecycle-required", lifecycleDocLink)
}

// TestFindLifecycleRepositoryRoot does not depend on the checker's directory depth.
func TestFindLifecycleRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "arbitrary", "deep", "directory")
	require.NoError(t, os.MkdirAll(nested, 0755))
	// A nearest-but-invalid module must fail, not fall back to an ancestor.
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/repo\n"), 0600))
	_, err := findLifecycleRepositoryRoot(nested)
	require.ErrorContains(t, err, "read lifecycle documentation")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "test"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(lifecycleDocPath)), []byte(lifecycleDocHeading+"\n"), 0600))
	actual, err := findLifecycleRepositoryRoot(nested)
	require.NoError(t, err)
	require.Equal(t, root, actual)

	require.NoError(t, os.WriteFile(filepath.Join(nested, "go.mod"), []byte("go 1.26\n"), 0600))
	_, err = findLifecycleRepositoryRoot(nested)
	require.ErrorContains(t, err, "exactly one module directive")
}

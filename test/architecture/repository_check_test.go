package architecture

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	lifecycleDocPath    = "test/GUIDELINES.md"
	lifecycleDocHeading = "### Shared Container Lifecycle (Required)"
)

var lifecycleDocLink = lifecycleDocPath + "#" + lifecycleHeadingAnchor(lifecycleDocHeading)

func lifecycleHeadingAnchor(heading string) string {
	return strings.Map(func(character rune) rune {
		switch {
		case character == ' ':
			return '-'
		case unicode.IsLetter(character), unicode.IsDigit(character), character == '-', character == '_':
			return unicode.ToLower(character)
		default:
			return -1
		}
	}, strings.TrimSpace(strings.TrimLeft(heading, "#")))
}

var moduleDirective = regexp.MustCompile(`^module\s+("(?:[^"\\]|\\.)*"|[^\s"]+)\s*(?://.*)?$`)

// Find the nearest module, then validate its metadata rather than assuming this
// check always lives exactly two directories below the repository root.
func findLifecycleRepositoryRoot(start string) (string, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		_, err := os.Stat(filepath.Join(directory, "go.mod"))
		if err == nil {
			if _, err := lifecycleRepositoryModule(os.DirFS(directory)); err != nil {
				return "", fmt.Errorf("validate repository root %s: %w", directory, err)
			}
			return directory, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("find repository go.mod: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("no repository go.mod found from %s", start)
		}
		directory = parent
	}
}

func lifecycleRepositoryModule(tree fs.FS) (string, error) {
	content, err := fs.ReadFile(tree, "go.mod")
	if err != nil {
		return "", fmt.Errorf("read repository go.mod: %w", err)
	}
	var module string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "module" {
			continue
		}
		if module != "" {
			return "", fmt.Errorf("repository go.mod must contain exactly one module directive")
		}
		match := moduleDirective.FindStringSubmatch(line)
		if match == nil {
			return "", fmt.Errorf("invalid module directive in repository go.mod: %q", line)
		}
		module = match[1]
		if strings.HasPrefix(module, `"`) {
			module, err = strconv.Unquote(module)
			if err != nil {
				return "", fmt.Errorf("invalid quoted module path in repository go.mod: %w", err)
			}
		}
		if module == "" || module == "." || path.IsAbs(module) || path.Clean(module) != module || strings.ContainsAny(module, " \t\r\n\\") {
			return "", fmt.Errorf("invalid module path in repository go.mod: %q", module)
		}
	}
	if module == "" {
		return "", fmt.Errorf("repository go.mod must contain exactly one module directive")
	}
	documentation, err := fs.ReadFile(tree, lifecycleDocPath)
	if err != nil {
		return "", fmt.Errorf("read lifecycle documentation %s: %w", lifecycleDocPath, err)
	}
	var fence string
	for _, line := range strings.Split(string(documentation), "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, string(fence[0])) == "" {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, string(trimmed[0])))]
			continue
		}
		if line == lifecycleDocHeading {
			return module, nil
		}
	}
	return "", fmt.Errorf("lifecycle documentation %s is missing heading %q; update the section and diagnostic link together", lifecycleDocPath, lifecycleDocHeading)
}

// Package skill provides SKILL.md loading, frontmatter parsing, skill
// invocation, and output validation.
package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pithecene-io/bonsai/internal/assets"
)

// Definition holds a parsed skill definition.
type Definition struct {
	Name         string // From frontmatter or registry
	Description  string // From frontmatter
	Body         string // SKILL.md body (frontmatter stripped)
	OutputSchema string // output.schema.json content
	InputSchema  string // input.schema.json content
	Source       string // "repo-local", "user", or "embedded"
}

// Load loads a skill definition from the resolver.
func Load(resolver *assets.Resolver, name, version string) (*Definition, error) {
	fsPath, embedPath, err := resolver.ResolveSkillDir(name, version)
	if err != nil {
		return nil, fmt.Errorf("resolve skill %s/%s: %w", name, version, err)
	}

	var source string
	var readFile func(string) ([]byte, error)

	if fsPath != "" {
		source = "filesystem"
		readFile = func(name string) ([]byte, error) {
			return os.ReadFile(filepath.Join(fsPath, name))
		}
	} else {
		source = "embedded"
		efs := assets.EmbeddedFS()
		readFile = func(name string) ([]byte, error) {
			return fs.ReadFile(efs, filepath.Join(embedPath, name))
		}
	}

	// Validate required files
	for _, required := range []string{"SKILL.md", "input.schema.json", "output.schema.json"} {
		if _, err := readFile(required); err != nil {
			return nil, fmt.Errorf("missing required file: %s/%s/%s", name, version, required)
		}
	}

	skillMD, err := readFile("SKILL.md")
	if err != nil {
		return nil, err
	}

	outputSchema, err := readFile("output.schema.json")
	if err != nil {
		return nil, err
	}

	inputSchema, err := readFile("input.schema.json")
	if err != nil {
		return nil, err
	}

	body, frontmatter := parseFrontmatter(string(skillMD))

	def := &Definition{
		Name:         name,
		Body:         body,
		OutputSchema: string(outputSchema),
		InputSchema:  string(inputSchema),
		Source:       source,
	}

	// Extract description from frontmatter if present
	if desc, ok := frontmatter["description"]; ok {
		def.Description = desc
	}

	return def, nil
}

// parseFrontmatter strips YAML frontmatter from SKILL.md content.
// Returns the body (after frontmatter) and a map of frontmatter key-value pairs.
// Matches shell: sed '1{/^---$/!q}; 1,/^---$/d'
func parseFrontmatter(content string) (body string, fm map[string]string) {
	fm = make(map[string]string)
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, fm
	}

	// Find closing ---
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}

	if end == -1 {
		return content, fm
	}

	// Parse simple key: value pairs from frontmatter
	for _, line := range lines[1:end] {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			fm[key] = value
		}
	}

	// Body is everything after the closing ---
	if end+1 < len(lines) {
		body = strings.Join(lines[end+1:], "\n")
		// Trim leading newline if present
		body = strings.TrimPrefix(body, "\n")
	}

	return body, fm
}

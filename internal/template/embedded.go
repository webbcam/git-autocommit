package tmpl

import (
	"embed"
	"sort"
	"strings"
)

//go:embed templates/*.tmpl
var embeddedFS embed.FS

// embeddedTemplates maps template name to raw file content.
// Populated at init time from the embedded filesystem.
var embeddedTemplates map[string]string

func init() {
	entries, _ := embeddedFS.ReadDir("templates")
	embeddedTemplates = make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tmpl") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".tmpl")
		content, _ := embeddedFS.ReadFile("templates/" + e.Name())
		embeddedTemplates[name] = string(content)
	}
}

// EmbeddedNames returns the names of all built-in templates in stable order.
func EmbeddedNames() []string {
	names := make([]string, 0, len(embeddedTemplates))
	for name := range embeddedTemplates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AllEmbedded returns all built-in templates, sorted by name.
func AllEmbedded() ([]*Template, error) {
	names := EmbeddedNames()
	result := make([]*Template, 0, len(names))
	for _, name := range names {
		t, err := Parse(embeddedTemplates[name], name)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

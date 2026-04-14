// Package tmpl handles commit message template parsing and rendering.
package tmpl

import (
	"bytes"
	"fmt"
	"strings"
	gotmpl "text/template"

	"gopkg.in/yaml.v3"
)

const defaultMaxSubjectLength = 80

// frontmatter holds the optional YAML header from a template file.
type frontmatter struct {
	Name             string `yaml:"name"`
	Description      string `yaml:"description"`
	MaxSubjectLength int    `yaml:"max_subject_length"`
}

// RenderContext is the data passed to the template body during rendering.
// Additional fields can be added in future versions without breaking existing templates.
type RenderContext struct {
	MaxSubjectLength int
}

// Template is a parsed and ready-to-render commit message template.
type Template struct {
	Name             string
	Description      string
	MaxSubjectLength int
	RawBody          string // unexpanded body text, as written by the author
	body             *gotmpl.Template
}

// Parse parses a template from raw content. sourceName is used as the default
// template name when the frontmatter has no name field (e.g. filename without
// the .tmpl extension).
func Parse(content, sourceName string) (*Template, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	name := fm.Name
	if name == "" {
		name = sourceName
	}

	maxLen := fm.MaxSubjectLength
	if maxLen == 0 {
		maxLen = defaultMaxSubjectLength
	}

	t, err := gotmpl.New(name).Parse(body)
	if err != nil {
		return nil, fmt.Errorf("template %q: parse body: %w", name, err)
	}

	return &Template{
		Name:             name,
		Description:      fm.Description,
		MaxSubjectLength: maxLen,
		RawBody:          body,
		body:             t,
	}, nil
}

// Render executes the template body, substituting variables such as
// {{ .MaxSubjectLength }}.
func (t *Template) Render() (string, error) {
	var buf bytes.Buffer
	ctx := RenderContext{MaxSubjectLength: t.MaxSubjectLength}
	if err := t.body.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("template %q: render: %w", t.Name, err)
	}
	return buf.String(), nil
}

// splitFrontmatter separates optional YAML frontmatter from the template body.
// Frontmatter is detected when the content begins with "---\n". Unknown YAML
// fields are silently ignored for forward-compatibility.
func splitFrontmatter(content string) (frontmatter, string, error) {
	var fm frontmatter

	if !strings.HasPrefix(content, "---\n") {
		return fm, content, nil
	}

	rest := content[4:] // skip leading "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// No closing delimiter — treat entire content as body.
		return fm, content, nil
	}

	yamlSrc := rest[:idx]
	body := rest[idx+5:] // skip "\n---\n"

	if err := yaml.Unmarshal([]byte(yamlSrc), &fm); err != nil {
		return fm, "", fmt.Errorf("parsing frontmatter: %w", err)
	}

	return fm, body, nil
}

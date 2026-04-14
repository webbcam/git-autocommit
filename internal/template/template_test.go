package tmpl

import (
	"strings"
	"testing"
)

func TestParse_WithFrontmatter(t *testing.T) {
	content := "---\nname: mytemplate\ndescription: A test template\nmax_subject_length: 72\n---\n<subject, {{ .MaxSubjectLength }} chars>\n"
	tmpl, err := Parse(content, "fallback")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tmpl.Name != "mytemplate" {
		t.Errorf("Name = %q, want mytemplate", tmpl.Name)
	}
	if tmpl.Description != "A test template" {
		t.Errorf("Description = %q", tmpl.Description)
	}
	if tmpl.MaxSubjectLength != 72 {
		t.Errorf("MaxSubjectLength = %d, want 72", tmpl.MaxSubjectLength)
	}
}

func TestParse_WithoutFrontmatter(t *testing.T) {
	content := "<single line commit message>"
	tmpl, err := Parse(content, "bare")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tmpl.Name != "bare" {
		t.Errorf("Name = %q, want bare (from sourceName)", tmpl.Name)
	}
	if tmpl.MaxSubjectLength != defaultMaxSubjectLength {
		t.Errorf("MaxSubjectLength = %d, want %d (default)", tmpl.MaxSubjectLength, defaultMaxSubjectLength)
	}
	if tmpl.RawBody != content {
		t.Errorf("RawBody = %q, want %q", tmpl.RawBody, content)
	}
}

func TestParse_MissingNameDerivesFromSourceName(t *testing.T) {
	content := "---\ndescription: no name field\n---\nbody text\n"
	tmpl, err := Parse(content, "derived-name")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tmpl.Name != "derived-name" {
		t.Errorf("Name = %q, want derived-name", tmpl.Name)
	}
}

func TestParse_UnknownFrontmatterFieldsIgnored(t *testing.T) {
	content := "---\nname: test\nunknown_field: value\nanother_unknown: 42\n---\nbody\n"
	_, err := Parse(content, "test")
	if err != nil {
		t.Errorf("expected no error for unknown frontmatter fields, got: %v", err)
	}
}

func TestParse_MalformedFrontmatterError(t *testing.T) {
	content := "---\nname: [invalid yaml\n---\nbody\n"
	_, err := Parse(content, "test")
	if err == nil {
		t.Error("expected error for malformed frontmatter YAML")
	}
	if !strings.Contains(err.Error(), "parsing frontmatter") {
		t.Errorf("error message = %q, want it to mention 'parsing frontmatter'", err.Error())
	}
}

func TestParse_DefaultMaxSubjectLength(t *testing.T) {
	content := "---\nname: nomax\n---\nbody\n"
	tmpl, err := Parse(content, "nomax")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tmpl.MaxSubjectLength != defaultMaxSubjectLength {
		t.Errorf("MaxSubjectLength = %d, want %d (default)", tmpl.MaxSubjectLength, defaultMaxSubjectLength)
	}
}

func TestRender_SubstitutesMaxSubjectLength(t *testing.T) {
	content := "---\nname: t\nmax_subject_length: 60\n---\n<subject, {{ .MaxSubjectLength }} chars max>\n"
	tmpl, err := Parse(content, "t")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := tmpl.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "60") {
		t.Errorf("rendered output %q should contain 60", out)
	}
	if strings.Contains(out, "{{ .MaxSubjectLength }}") {
		t.Error("rendered output should not contain unexpanded template variable")
	}
}

func TestRender_UsesDefaultWhenNoFrontmatter(t *testing.T) {
	content := "{{ .MaxSubjectLength }}"
	tmpl, err := Parse(content, "t")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := tmpl.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	expected := "80" // defaultMaxSubjectLength
	if strings.TrimSpace(out) != expected {
		t.Errorf("Render() = %q, want %q", out, expected)
	}
}

func TestParse_BodyAfterFrontmatter(t *testing.T) {
	content := "---\nname: t\n---\nline1\nline2\n"
	tmpl, err := Parse(content, "t")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tmpl.RawBody != "line1\nline2\n" {
		t.Errorf("RawBody = %q, want %q", tmpl.RawBody, "line1\nline2\n")
	}
}

func TestParse_NoClosingFrontmatterDelimiter(t *testing.T) {
	// No closing "---" → treat entire content as body (no frontmatter).
	content := "---\nname: test\nbody text without closing delimiter"
	tmpl, err := Parse(content, "fallback")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Should use sourceName as fallback since frontmatter wasn't parsed.
	if tmpl.Name != "fallback" {
		t.Errorf("Name = %q, want fallback", tmpl.Name)
	}
}

// TestBuiltinTemplates verifies that the embedded full and short templates
// parse and render correctly.
func TestBuiltinTemplates(t *testing.T) {
	tests := []struct {
		name             string
		wantMaxSubject   int
		wantBodyContains string
	}{
		{"full", 80, "[Problem]"},
		{"short", 72, "imperative mood"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, ok := embeddedTemplates[tt.name]
			if !ok {
				t.Fatalf("built-in template %q not found in embedded templates", tt.name)
			}
			tmpl, err := Parse(content, tt.name)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if tmpl.MaxSubjectLength != tt.wantMaxSubject {
				t.Errorf("MaxSubjectLength = %d, want %d", tmpl.MaxSubjectLength, tt.wantMaxSubject)
			}
			out, err := tmpl.Render()
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(out, tt.wantBodyContains) {
				t.Errorf("rendered body missing %q", tt.wantBodyContains)
			}
			// Verify MaxSubjectLength was substituted
			if strings.Contains(out, "{{ .MaxSubjectLength }}") {
				t.Error("rendered output should not contain unexpanded template variable")
			}
		})
	}
}

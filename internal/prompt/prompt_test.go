package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectContextType(t *testing.T) {
	t.Run("https URL", func(t *testing.T) {
		ctx := DetectContextType("https://example.com/ticket/123")
		if ctx.Type != ContextURL {
			t.Errorf("Type = %v, want ContextURL", ctx.Type)
		}
		if ctx.Value != "https://example.com/ticket/123" {
			t.Errorf("Value = %q", ctx.Value)
		}
	})

	t.Run("http URL", func(t *testing.T) {
		ctx := DetectContextType("http://example.com")
		if ctx.Type != ContextURL {
			t.Errorf("Type = %v, want ContextURL", ctx.Type)
		}
	})

	t.Run("existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ticket.md")
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
		ctx := DetectContextType(path)
		if ctx.Type != ContextFile {
			t.Errorf("Type = %v, want ContextFile", ctx.Type)
		}
		if ctx.Value != path {
			t.Errorf("Value = %q, want %q", ctx.Value, path)
		}
	})

	t.Run("plain string", func(t *testing.T) {
		ctx := DetectContextType("part of the auth refactor")
		if ctx.Type != ContextString {
			t.Errorf("Type = %v, want ContextString", ctx.Type)
		}
		if ctx.Content != "part of the auth refactor" {
			t.Errorf("Content = %q", ctx.Content)
		}
	})

	t.Run("non-existent path treated as string", func(t *testing.T) {
		ctx := DetectContextType("/no/such/file.md")
		if ctx.Type != ContextString {
			t.Errorf("Type = %v, want ContextString", ctx.Type)
		}
	})

	t.Run("empty value", func(t *testing.T) {
		ctx := DetectContextType("")
		if ctx.Type != ContextNone {
			t.Errorf("Type = %v, want ContextNone", ctx.Type)
		}
	})
}

func TestBuild_AlwaysContainsDelimiters(t *testing.T) {
	p := Build("some diff", "template body", Context{})
	if !strings.Contains(p, "===COMMIT_MSG_START===") {
		t.Error("prompt missing start delimiter")
	}
	if !strings.Contains(p, "===COMMIT_MSG_END===") {
		t.Error("prompt missing end delimiter")
	}
}

func TestBuild_ContainsDiff(t *testing.T) {
	diff := "--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n+hello"
	p := Build(diff, "body", Context{})
	if !strings.Contains(p, diff) {
		t.Error("prompt should contain the diff verbatim")
	}
}

func TestBuild_ContainsTemplateBody(t *testing.T) {
	body := "<subject line, 80 chars max>\n\n[Problem]\n<why>"
	p := Build("", body, Context{})
	if !strings.Contains(p, body) {
		t.Error("prompt should contain the template body verbatim")
	}
}

func TestBuild_NoContext(t *testing.T) {
	p := Build("", "body", Context{Type: ContextNone})
	if strings.Contains(p, "Additional context") {
		t.Error("prompt with no context should not mention 'Additional context'")
	}
}

func TestBuild_StringContext(t *testing.T) {
	ctx := Context{Type: ContextString, Value: "the auth refactor", Content: "the auth refactor"}
	p := Build("", "body", ctx)
	if !strings.Contains(p, "the auth refactor") {
		t.Error("prompt should contain string context")
	}
}

func TestBuild_FileContext(t *testing.T) {
	ctx := Context{Type: ContextFile, Value: "/path/to/ticket.md", Content: "ticket body text"}
	p := Build("", "body", ctx)
	if !strings.Contains(p, "ticket.md") {
		t.Error("prompt should reference the file name")
	}
	if !strings.Contains(p, "ticket body text") {
		t.Error("prompt should contain resolved file content")
	}
}

func TestBuild_URLContext(t *testing.T) {
	ctx := Context{Type: ContextURL, Value: "https://example.com/123", Content: "page body"}
	p := Build("", "body", ctx)
	if !strings.Contains(p, "https://example.com/123") {
		t.Error("prompt should reference the URL")
	}
	if !strings.Contains(p, "page body") {
		t.Error("prompt should contain resolved URL content")
	}
}

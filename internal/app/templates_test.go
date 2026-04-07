package app

import (
	"strings"
	"testing"
)

func TestShouldTruncatePostContent(t *testing.T) {
	if shouldTruncatePostContent("short post") {
		t.Fatal("short post should not truncate")
	}
	if shouldTruncatePostContent(strings.Repeat("a", 700)) {
		t.Fatal("medium post should not truncate")
	}
	if !shouldTruncatePostContent("```go\nfmt.Println(\"hi\")\n```") {
		t.Fatal("code block should truncate")
	}
	if !shouldTruncatePostContent("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14\nline15") {
		t.Fatal("many-line post should truncate")
	}
	if !shouldTruncatePostContent(strings.Repeat("a", 1000)) {
		t.Fatal("long post should truncate")
	}
}

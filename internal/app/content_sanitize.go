package app

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	errInvalidUTF8       = errors.New("post contains invalid UTF-8")
	errInvisibleSpoofing = errors.New("post contains unsupported invisible or bidi control characters")
)

func sanitizePostContent(content string) (string, error) {
	if !utf8.ValidString(content) {
		return "", errInvalidUTF8
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	var b strings.Builder
	b.Grow(len(content))

	for _, r := range content {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// Strip control characters except newline and tab.
			continue
		case isBlockedInvisibleRune(r):
			return "", errInvisibleSpoofing
		default:
			b.WriteRune(r)
		}
	}

	return b.String(), nil
}

func isBlockedInvisibleRune(r rune) bool {
	switch r {
	case '\u200b', '\u200c', '\u200d', '\ufeff':
		return true
	}
	return (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

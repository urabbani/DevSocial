package app

import "testing"

func TestSanitizePostContentNormalizesAndStripsControls(t *testing.T) {
	got, err := sanitizePostContent("hello\r\nworld\x00\tok\rdone")
	if err != nil {
		t.Fatalf("sanitizePostContent returned error: %v", err)
	}
	want := "hello\nworld\tok\ndone"
	if got != want {
		t.Fatalf("sanitizePostContent = %q, want %q", got, want)
	}
}

func TestSanitizePostContentRejectsInvalidUTF8(t *testing.T) {
	if _, err := sanitizePostContent(string([]byte{0xff, 0xfe, 'a'})); err != errInvalidUTF8 {
		t.Fatalf("error = %v, want %v", err, errInvalidUTF8)
	}
}

func TestSanitizePostContentRejectsInvisibleSpoofingChars(t *testing.T) {
	cases := []string{
		"bad\u202econtent",
		"bad\u2066content",
		"bad\u200bcontent",
		"bad\ufeffcontent",
	}
	for _, input := range cases {
		if _, err := sanitizePostContent(input); err != errInvisibleSpoofing {
			t.Fatalf("sanitizePostContent(%q) error = %v, want %v", input, err, errInvisibleSpoofing)
		}
	}
}

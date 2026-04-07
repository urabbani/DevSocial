package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleUserRSS(t *testing.T) {
	app := newTestApp(t)
	app.BaseURL = "https://karpathytalk.com"

	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}

	postID, err := app.CreatePost(author.ID, "Top level post", markdownHTML(t, "Top level post"), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePost(top-level): %v", err)
	}
	if _, err := app.CreatePost(author.ID, "Reply post", markdownHTML(t, "Reply post"), &postID, nil, nil, nil); err != nil {
		t.Fatalf("CreatePost(reply): %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/user/author/feed.xml", nil)
	req.SetPathValue("username", "author")
	rr := httptest.NewRecorder()

	app.handleUserRSS(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "application/rss+xml") {
		t.Fatalf("Content-Type = %q, want RSS XML", got)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "<title>@author on KarpathyTalk</title>") {
		t.Fatalf("feed body missing channel title: %q", body)
	}
	if !strings.Contains(body, "<link>https://karpathytalk.com/user/author</link>") {
		t.Fatalf("feed body missing profile link: %q", body)
	}
	if !strings.Contains(body, `href="https://karpathytalk.com/user/author/feed.xml"`) {
		t.Fatalf("feed body missing self link: %q", body)
	}
	if strings.Count(body, "<item>") != 1 {
		t.Fatalf("item count = %d, want 1", strings.Count(body, "<item>"))
	}
	if !strings.Contains(body, "<title>Top level post</title>") {
		t.Fatalf("feed body missing item title: %q", body)
	}
	if !strings.Contains(body, fmt.Sprintf("<link>https://karpathytalk.com/posts/%d</link>", postID)) {
		t.Fatalf("feed body missing post link: %q", body)
	}
	if !strings.Contains(body, "<description><![CDATA[<p>Top level post</p>\n]]></description>") {
		t.Fatalf("feed body missing rendered HTML description: %q", body)
	}
}

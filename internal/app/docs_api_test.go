package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHandleDocsRaw(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/docs.md", nil)
	rr := httptest.NewRecorder()

	app.handleDocsRaw(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/markdown") {
		t.Fatalf("Content-Type = %q, want markdown", got)
	}
	if !strings.Contains(rr.Body.String(), "# Docs") {
		t.Fatalf("docs body missing heading: %q", rr.Body.String())
	}
}

func TestHandlePostMarkdown(t *testing.T) {
	app := newTestApp(t)
	app.BaseURL = "https://devsocial.app"

	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}

	content := "Hello\n\n![image](/uploads/example.png)"
	postID, err := app.CreatePost(author.ID, content, markdownHTML(t, content), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/posts/1/md", nil)
	req.SetPathValue("id", strconv.FormatInt(postID, 10))
	rr := httptest.NewRecorder()

	app.handlePostMarkdown(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/markdown") {
		t.Fatalf("Content-Type = %q, want markdown", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "kind: \"post\"") {
		t.Fatalf("post markdown missing kind frontmatter: %q", body)
	}
	if !strings.Contains(body, "id: "+strconv.FormatInt(postID, 10)) {
		t.Fatalf("post markdown missing post id: %q", body)
	}
	if !strings.Contains(body, "author_username: \"author\"") {
		t.Fatalf("post markdown missing author: %q", body)
	}
	if !strings.Contains(body, "![image](https://devsocial.app/uploads/example.png)") {
		t.Fatalf("post markdown missing absolutized body url: %q", body)
	}
}

func TestHandleUserMarkdown(t *testing.T) {
	app := newTestApp(t)
	app.BaseURL = "https://devsocial.app"

	user, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}
	if _, err := app.CreatePost(user.ID, "Top level", markdownHTML(t, "Top level"), nil, nil, nil, nil); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/user/author/md", nil)
	req.SetPathValue("username", "author")
	rr := httptest.NewRecorder()

	app.handleUserMarkdown(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/markdown") {
		t.Fatalf("Content-Type = %q, want markdown", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "kind: \"user\"") {
		t.Fatalf("user markdown missing kind frontmatter: %q", body)
	}
	if !strings.Contains(body, "username: \"author\"") {
		t.Fatalf("user markdown missing username: %q", body)
	}
	if !strings.Contains(body, "# @author") {
		t.Fatalf("user markdown missing heading: %q", body)
	}
	if !strings.Contains(body, "- GitHub: https://github.com/author") {
		t.Fatalf("user markdown missing github line: %q", body)
	}
}

func TestAPIPostsQuery(t *testing.T) {
	app := newTestApp(t)
	app.BaseURL = "https://devsocial.app"

	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}
	other, err := app.UpsertUser(2, "other", "Other", "https://example.com/o.png")
	if err != nil {
		t.Fatalf("UpsertUser(other): %v", err)
	}

	topID, err := app.CreatePost(author.ID, "Top level", markdownHTML(t, "Top level"), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePost(top-level): %v", err)
	}
	if _, err := app.CreatePost(author.ID, "Reply", markdownHTML(t, "Reply"), &topID, nil, nil, nil); err != nil {
		t.Fatalf("CreatePost(reply): %v", err)
	}
	if _, err := app.CreatePost(other.ID, "Other top level", markdownHTML(t, "Other top level"), nil, nil, nil, nil); err != nil {
		t.Fatalf("CreatePost(other): %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts?author=author&has_parent=false", nil)
	rr := httptest.NewRecorder()
	app.handleAPIPosts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("posts query status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp apiPostsQueryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(posts query): %v", err)
	}
	if len(resp.Posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(resp.Posts))
	}
	if resp.Posts[0].Author.Username != "author" {
		t.Fatalf("author username = %q, want author", resp.Posts[0].Author.Username)
	}
	if resp.Posts[0].ParentPostID != nil {
		t.Fatal("top-level query returned child post")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/posts?author=author&has_parent=true", nil)
	rr = httptest.NewRecorder()
	app.handleAPIPosts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("child-post query status = %d, want %d", rr.Code, http.StatusOK)
	}

	resp = apiPostsQueryResponse{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(child-post query): %v", err)
	}
	if len(resp.Posts) != 1 {
		t.Fatalf("len(child posts) = %d, want 1", len(resp.Posts))
	}
	if resp.Posts[0].ParentPostID == nil || *resp.Posts[0].ParentPostID != topID {
		t.Fatalf("parent_post_id = %v, want %d", resp.Posts[0].ParentPostID, topID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/posts?parent_post_id="+strconv.FormatInt(topID, 10), nil)
	rr = httptest.NewRecorder()
	app.handleAPIPosts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("parent filter status = %d, want %d", rr.Code, http.StatusOK)
	}

	resp = apiPostsQueryResponse{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(parent filter): %v", err)
	}
	if len(resp.Posts) != 1 {
		t.Fatalf("len(parent filtered posts) = %d, want 1", len(resp.Posts))
	}
	if resp.Posts[0].ParentPostID == nil || *resp.Posts[0].ParentPostID != topID {
		t.Fatalf("parent filtered parent_post_id = %v, want %d", resp.Posts[0].ParentPostID, topID)
	}
}

func TestAPIPostAbsolutizesRelativeMarkdownURLs(t *testing.T) {
	app := newTestApp(t)
	app.BaseURL = "https://devsocial.app"

	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}

	content := "![image](/uploads/example.png)\n\n[ref]: /uploads/other.png"
	postID, err := app.CreatePost(author.ID, content, markdownHTML(t, content), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	app.handleAPIPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp apiPostResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp.Post.ID != postID {
		t.Fatalf("post id = %d, want %d", resp.Post.ID, postID)
	}
	if strings.Contains(resp.Post.ContentMarkdown, "](/uploads/") {
		t.Fatalf("content_markdown still contains relative inline url: %q", resp.Post.ContentMarkdown)
	}
	if strings.Contains(resp.Post.ContentMarkdown, "]: /uploads/") {
		t.Fatalf("content_markdown still contains relative reference url: %q", resp.Post.ContentMarkdown)
	}
	if !strings.Contains(resp.Post.ContentMarkdown, "](https://devsocial.app/uploads/example.png)") {
		t.Fatalf("content_markdown missing absolutized inline url: %q", resp.Post.ContentMarkdown)
	}
	if !strings.Contains(resp.Post.ContentMarkdown, "[ref]: https://devsocial.app/uploads/other.png") {
		t.Fatalf("content_markdown missing absolutized reference url: %q", resp.Post.ContentMarkdown)
	}
}

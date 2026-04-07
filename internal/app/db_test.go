package app

import (
	"path/filepath"
	"testing"
)

func newTestApp(t *testing.T) *App {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "karpathytalk-test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return &App{DB: db}
}

func markdownHTML(t *testing.T, content string) string {
	t.Helper()

	html, err := RenderMarkdown(content)
	if err != nil {
		t.Fatalf("RenderMarkdown(%q): %v", content, err)
	}
	return html
}

func TestEditPostCreatesRevisionHistory(t *testing.T) {
	app := newTestApp(t)

	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}

	postID, err := app.CreatePost(author.ID, "first", markdownHTML(t, "first"), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	post, err := app.GetPost(postID)
	if err != nil {
		t.Fatalf("GetPost initial: %v", err)
	}
	if post.RevisionCount != 1 || post.RevisionNumber != 1 {
		t.Fatalf("initial revision state = count %d number %d, want 1/1", post.RevisionCount, post.RevisionNumber)
	}

	revision, err := app.EditPost(postID, author.ID, "second", markdownHTML(t, "second"))
	if err != nil {
		t.Fatalf("EditPost: %v", err)
	}
	if revision.RevisionNumber != 2 {
		t.Fatalf("new revision number = %d, want 2", revision.RevisionNumber)
	}

	post, err = app.GetPost(postID)
	if err != nil {
		t.Fatalf("GetPost after edit: %v", err)
	}
	if post.Content != "second" {
		t.Fatalf("latest content = %q, want %q", post.Content, "second")
	}
	if post.RevisionCount != 2 || post.RevisionNumber != 2 {
		t.Fatalf("latest revision state = count %d number %d, want 2/2", post.RevisionCount, post.RevisionNumber)
	}
	if post.EditedAt == nil {
		t.Fatal("EditedAt is nil after edit")
	}

	firstRevision, err := app.GetPostRevision(postID, 1)
	if err != nil {
		t.Fatalf("GetPostRevision(1): %v", err)
	}
	if firstRevision.Content != "first" {
		t.Fatalf("revision 1 content = %q, want %q", firstRevision.Content, "first")
	}

	revisions, err := app.GetPostRevisions(postID)
	if err != nil {
		t.Fatalf("GetPostRevisions: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("len(revisions) = %d, want 2", len(revisions))
	}
	if revisions[0].RevisionNumber != 2 || revisions[1].RevisionNumber != 1 {
		t.Fatalf("revision order = [%d, %d], want [2, 1]", revisions[0].RevisionNumber, revisions[1].RevisionNumber)
	}
}

func TestRevisionAwareQuoteReplyAndRepost(t *testing.T) {
	app := newTestApp(t)

	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser(author): %v", err)
	}
	reader, err := app.UpsertUser(2, "reader", "Reader", "https://example.com/r.png")
	if err != nil {
		t.Fatalf("UpsertUser(reader): %v", err)
	}

	parentID, err := app.CreatePost(author.ID, "rev1", markdownHTML(t, "rev1"), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreatePost parent: %v", err)
	}
	parent, err := app.GetPost(parentID)
	if err != nil {
		t.Fatalf("GetPost parent rev1: %v", err)
	}

	quoteID, err := app.CreatePost(reader.ID, "quoting rev1", markdownHTML(t, "quoting rev1"), nil, nil, &parentID, &parent.RevisionID)
	if err != nil {
		t.Fatalf("CreatePost quote: %v", err)
	}
	if _, err := app.CreatePost(reader.ID, "reply on rev1", markdownHTML(t, "reply on rev1"), &parentID, &parent.RevisionID, nil, nil); err != nil {
		t.Fatalf("CreatePost reply: %v", err)
	}
	if _, err := app.ToggleRepost(reader.ID, parentID, parent.RevisionID); err != nil {
		t.Fatalf("ToggleRepost: %v", err)
	}

	if _, err := app.EditPost(parentID, author.ID, "rev2", markdownHTML(t, "rev2")); err != nil {
		t.Fatalf("EditPost parent: %v", err)
	}

	quotedPost, err := app.GetPostWithAuthor(quoteID)
	if err != nil {
		t.Fatalf("GetPostWithAuthor quote: %v", err)
	}
	if quotedPost.QuotedPost == nil {
		t.Fatal("QuotedPost is nil")
	}
	if quotedPost.QuotedPost.RevisionNumber != 1 {
		t.Fatalf("quoted revision number = %d, want 1", quotedPost.QuotedPost.RevisionNumber)
	}
	if quotedPost.QuotedPost.RevisionCount != 2 {
		t.Fatalf("quoted current revision count = %d, want 2", quotedPost.QuotedPost.RevisionCount)
	}

	replies, err := app.GetReplies(parentID)
	if err != nil {
		t.Fatalf("GetReplies: %v", err)
	}
	if err := app.HydratePosts(replies, reader.ID); err != nil {
		t.Fatalf("HydratePosts(replies): %v", err)
	}
	if len(replies) != 1 {
		t.Fatalf("len(replies) = %d, want 1", len(replies))
	}
	if replies[0].ParentPost == nil {
		t.Fatal("reply parent post is nil")
	}
	if replies[0].ParentPost.RevisionNumber != 1 {
		t.Fatalf("reply parent revision number = %d, want 1", replies[0].ParentPost.RevisionNumber)
	}

	repostRevisionID, err := app.GetRepostRevisionID(reader.ID, parentID)
	if err != nil {
		t.Fatalf("GetRepostRevisionID: %v", err)
	}
	repostRevision, err := app.GetPostRevisionByID(parentID, repostRevisionID)
	if err != nil {
		t.Fatalf("GetPostRevisionByID repost revision: %v", err)
	}
	if repostRevision.RevisionNumber != 1 {
		t.Fatalf("repost revision number = %d, want 1", repostRevision.RevisionNumber)
	}
}

func TestLoadTemplates(t *testing.T) {
	LoadTemplates()
}

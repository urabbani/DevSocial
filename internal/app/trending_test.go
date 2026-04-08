package app

import (
	"testing"
	"time"
)

// trendingTestSetup creates an author and n posts via CreatePost.
// Returns the post IDs in creation order.
func trendingTestSetup(t *testing.T, app *App, n int) []int64 {
	t.Helper()
	author, err := app.UpsertUser(1, "author", "Author", "https://example.com/a.png")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	ids := make([]int64, n)
	for i := 0; i < n; i++ {
		id, err := app.CreatePost(author.ID, "p", "<p>p</p>", nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreatePost[%d]: %v", i, err)
		}
		ids[i] = id
	}
	return ids
}

func setEngagement(t *testing.T, app *App, postID int64, likes, reposts, comments int) {
	t.Helper()
	if _, err := app.DB.Exec(
		`UPDATE posts SET like_count = ?, repost_count = ?, comment_count = ? WHERE id = ?`,
		likes, reposts, comments, postID,
	); err != nil {
		t.Fatalf("setEngagement(%d): %v", postID, err)
	}
}

func setCreatedAt(t *testing.T, app *App, postID int64, when time.Time) {
	t.Helper()
	if _, err := app.DB.Exec(
		`UPDATE posts SET created_at = ? WHERE id = ?`,
		when.UTC(), postID,
	); err != nil {
		t.Fatalf("setCreatedAt(%d): %v", postID, err)
	}
}

func TestRebuildTrendingExcludesZeroEngagement(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 3)

	// Only the middle post has engagement; the other two should be excluded.
	setEngagement(t, app, ids[1], 5, 0, 0)

	if err := app.RebuildTrending(); err != nil {
		t.Fatalf("RebuildTrending: %v", err)
	}
	posts, _, err := app.GetTrendingPosts(50, 0)
	if err != nil {
		t.Fatalf("GetTrendingPosts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(posts))
	}
	if posts[0].ID != ids[1] {
		t.Fatalf("trending post = %d, want %d", posts[0].ID, ids[1])
	}
}

func TestRebuildTrendingOrdersByEngagementWhenAgesEqual(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 3)

	// Identical age means age cancels out and the score reduces to engagement.
	pinned := time.Now().UTC().Add(-1 * time.Hour)
	for _, id := range ids {
		setCreatedAt(t, app, id, pinned)
	}
	setEngagement(t, app, ids[0], 1, 0, 0)
	setEngagement(t, app, ids[1], 10, 0, 0)
	setEngagement(t, app, ids[2], 5, 0, 0)

	if err := app.RebuildTrending(); err != nil {
		t.Fatalf("RebuildTrending: %v", err)
	}
	posts, _, err := app.GetTrendingPosts(50, 0)
	if err != nil {
		t.Fatalf("GetTrendingPosts: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("len(posts) = %d, want 3", len(posts))
	}
	want := []int64{ids[1], ids[2], ids[0]} // 10, 5, 1
	for i, w := range want {
		if posts[i].ID != w {
			t.Fatalf("posts[%d].ID = %d, want %d", i, posts[i].ID, w)
		}
	}
}

func TestRebuildTrendingAgeDecayBeatsRawEngagement(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 2)

	// Old high-engagement post:  10 / (102)^1.8 ≈ 0.0024
	// Fresh low-engagement post:  2 / (2)^1.8   ≈ 0.5743
	setCreatedAt(t, app, ids[0], time.Now().UTC().Add(-100*time.Hour))
	setEngagement(t, app, ids[0], 10, 0, 0)
	setCreatedAt(t, app, ids[1], time.Now().UTC().Add(-1*time.Minute))
	setEngagement(t, app, ids[1], 2, 0, 0)

	if err := app.RebuildTrending(); err != nil {
		t.Fatalf("RebuildTrending: %v", err)
	}
	posts, _, err := app.GetTrendingPosts(50, 0)
	if err != nil {
		t.Fatalf("GetTrendingPosts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}
	if posts[0].ID != ids[1] {
		t.Fatalf("posts[0].ID = %d, want %d (fresh post should rank first)", posts[0].ID, ids[1])
	}
}

func TestRebuildTrendingExcludesPostsOutsideWindow(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 2)

	// One post just inside the 7-day window, one outside, with the OUTSIDE
	// post having way more engagement — it should still be excluded.
	setCreatedAt(t, app, ids[0], time.Now().UTC().Add(-6*24*time.Hour))
	setEngagement(t, app, ids[0], 5, 0, 0)
	setCreatedAt(t, app, ids[1], time.Now().UTC().Add(-8*24*time.Hour))
	setEngagement(t, app, ids[1], 1000, 0, 0)

	if err := app.RebuildTrending(); err != nil {
		t.Fatalf("RebuildTrending: %v", err)
	}
	posts, _, err := app.GetTrendingPosts(50, 0)
	if err != nil {
		t.Fatalf("GetTrendingPosts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(posts))
	}
	if posts[0].ID != ids[0] {
		t.Fatalf("posts[0].ID = %d, want %d", posts[0].ID, ids[0])
	}
}

func TestRebuildTrendingExcludesReplies(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 1)

	// Manually create a reply with engagement and verify it does not appear
	// in trending — only top-level posts should.
	parent := ids[0]
	res, err := app.DB.Exec(
		`INSERT INTO posts (author_id, content, content_html, parent_post_id, like_count, revision_count) VALUES (1, 'r', '<p>r</p>', ?, 100, 1)`,
		parent,
	)
	if err != nil {
		t.Fatalf("insert reply: %v", err)
	}
	replyID, _ := res.LastInsertId()
	setEngagement(t, app, parent, 1, 0, 0)

	if err := app.RebuildTrending(); err != nil {
		t.Fatalf("RebuildTrending: %v", err)
	}
	posts, _, err := app.GetTrendingPosts(50, 0)
	if err != nil {
		t.Fatalf("GetTrendingPosts: %v", err)
	}
	for _, p := range posts {
		if p.ID == replyID {
			t.Fatalf("reply %d unexpectedly present in trending", replyID)
		}
	}
	if len(posts) != 1 || posts[0].ID != parent {
		t.Fatalf("trending = %v, want exactly [%d]", postIDs(posts), parent)
	}
}

func TestGetTrendingPostsCursorPagination(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 5)

	// Pin all posts to the same age so order is engagement-only.
	pinned := time.Now().UTC().Add(-1 * time.Hour)
	for _, id := range ids {
		setCreatedAt(t, app, id, pinned)
	}
	setEngagement(t, app, ids[0], 100, 0, 0)
	setEngagement(t, app, ids[1], 80, 0, 0)
	setEngagement(t, app, ids[2], 60, 0, 0)
	setEngagement(t, app, ids[3], 40, 0, 0)
	setEngagement(t, app, ids[4], 20, 0, 0)

	if err := app.RebuildTrending(); err != nil {
		t.Fatalf("RebuildTrending: %v", err)
	}

	want := []int64{ids[0], ids[1], ids[2], ids[3], ids[4]}
	var got []int64
	var afterRank int64
	for i := 0; i < 10; i++ { // safety bound
		posts, ranks, err := app.GetTrendingPosts(2, afterRank)
		if err != nil {
			t.Fatalf("GetTrendingPosts: %v", err)
		}
		if len(posts) == 0 {
			break
		}
		for _, p := range posts {
			got = append(got, p.ID)
		}
		afterRank = ranks[len(ranks)-1]
	}
	if len(got) != len(want) {
		t.Fatalf("got %d posts, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d (full got=%v)", i, got[i], want[i], got)
		}
	}
}

func TestRebuildTrendingIsIdempotent(t *testing.T) {
	app := newTestApp(t)
	ids := trendingTestSetup(t, app, 2)
	setEngagement(t, app, ids[0], 5, 0, 0)
	setEngagement(t, app, ids[1], 3, 0, 0)

	for i := 0; i < 3; i++ {
		if err := app.RebuildTrending(); err != nil {
			t.Fatalf("RebuildTrending #%d: %v", i, err)
		}
	}
	posts, ranks, err := app.GetTrendingPosts(50, 0)
	if err != nil {
		t.Fatalf("GetTrendingPosts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}
	// Ranks should always be 1 and 2 — never accumulate across rebuilds.
	if ranks[0] != 1 || ranks[1] != 2 {
		t.Fatalf("ranks = %v, want [1 2]", ranks)
	}
}

func postIDs(posts []*Post) []int64 {
	out := make([]int64, len(posts))
	for i, p := range posts {
		out[i] = p.ID
	}
	return out
}

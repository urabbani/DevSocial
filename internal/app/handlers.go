package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxPostLength     = 10000
	maxReplyLength    = 5000
	maxImageSize      = 5 << 20 // 5MB
	maxImagesPerPost  = 4
	postsPerPage      = 20
	postsPerHourMax   = 30
	repliesPerHour    = 60
	uploadsPerHourMax = 30
)

var allowedImageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// --- Timeline ---

func (app *App) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		app.renderNotFound(w, r)
		return
	}

	user := GetCurrentUser(r)
	activeTab := r.URL.Query().Get("tab")
	if activeTab != "following" && activeTab != "activity" && activeTab != "trending" {
		activeTab = "latest"
	}
	if user == nil && (activeTab == "following" || activeTab == "activity") {
		activeTab = "latest"
	}

	beforeCursor := r.URL.Query().Get("before")
	beforeID := int64(0)
	if activeTab != "activity" && beforeCursor != "" {
		beforeID, _ = strconv.ParseInt(beforeCursor, 10, 64)
	}

	data := &PageData{
		CurrentUser: user,
		Title:       "KarpathyTalk",
		ActiveTab:   activeTab,
	}

	if activeTab == "activity" {
		items, nextCursor, err := app.GetActivityItems(user.ID, postsPerPage+1, beforeCursor)
		if err != nil {
			app.renderStatus(w, r, http.StatusInternalServerError, "Activity Error", "Failed to load activity.")
			return
		}
		hasMore := len(items) > postsPerPage
		if hasMore {
			items = items[:postsPerPage]
		}
		data.Activity = items
		data.HasMore = hasMore
		if hasMore && len(items) > 0 {
			data.LoadMoreURL = fmt.Sprintf("/?tab=activity&before=%s", url.QueryEscape(nextCursor))
		}
		if r.Header.Get("HX-Request") == "true" && beforeCursor != "" {
			app.renderActivityFragment(w, data)
			return
		}
		app.render(w, "timeline.html", data)
		return
	}

	if activeTab == "trending" {
		afterRank, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		posts, ranks, err := app.GetTrendingPosts(postsPerPage+1, afterRank)
		if err != nil {
			app.renderStatus(w, r, http.StatusInternalServerError, "Timeline Error", "Failed to load trending.")
			return
		}
		hasMore := len(posts) > postsPerPage
		if hasMore {
			posts = posts[:postsPerPage]
			ranks = ranks[:postsPerPage]
		}
		var uid int64
		if user != nil {
			uid = user.ID
		}
		app.HydratePosts(posts, uid)
		data.Posts = posts
		data.HasMore = hasMore
		if hasMore && len(ranks) > 0 {
			data.LoadMoreURL = fmt.Sprintf("/?tab=trending&after=%d", ranks[len(ranks)-1])
		}
		if r.Header.Get("HX-Request") == "true" && afterRank > 0 {
			app.renderFragment(w, data)
			return
		}
		app.render(w, "timeline.html", data)
		return
	}

	if v := beforeCursor; v != "" {
		beforeID, _ = strconv.ParseInt(v, 10, 64)
	}

	var posts []*Post
	var err error
	if activeTab == "following" {
		posts, err = app.GetFollowingTimelinePosts(user.ID, postsPerPage+1, beforeID)
	} else {
		posts, err = app.GetTimelinePosts(postsPerPage+1, beforeID)
	}
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Timeline Error", "Failed to load timeline.")
		return
	}

	hasMore := len(posts) > postsPerPage
	if hasMore {
		posts = posts[:postsPerPage]
	}

	var uid int64
	if user != nil {
		uid = user.ID
	}
	app.HydratePosts(posts, uid)

	data.Posts = posts
	data.HasMore = hasMore
	if hasMore && len(posts) > 0 {
		data.NextCursor = posts[len(posts)-1].ID
		if activeTab == "following" {
			data.LoadMoreURL = fmt.Sprintf("/?tab=following&before=%d", data.NextCursor)
		} else {
			data.LoadMoreURL = fmt.Sprintf("/?before=%d", data.NextCursor)
		}
	}

	if r.Header.Get("HX-Request") == "true" && beforeID > 0 {
		app.renderFragment(w, data)
		return
	}

	app.render(w, "timeline.html", data)
}

// --- Single Post ---

func (app *App) handleViewPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	post, err := app.GetPostWithAuthor(id)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	user := GetCurrentUser(r)
	var uid int64
	if user != nil {
		uid = user.ID
	}
	app.HydratePosts([]*Post{post}, uid)

	replies, err := app.GetReplies(id)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Post Error", "Failed to load replies.")
		return
	}
	app.HydratePosts(replies, uid)
	if user != nil && post.IsReposted {
		if revisionID, err := app.GetRepostRevisionID(user.ID, post.ID); err == nil {
			post.ViewerRepostRevisionID = revisionID
			if repostRevision, err := app.GetPostRevisionByID(post.ID, revisionID); err == nil {
				post.ViewerRepostRevisionNumber = repostRevision.RevisionNumber
			}
		}
	}

	app.render(w, "post.html", &PageData{
		CurrentUser: user,
		Title:       fmt.Sprintf("Post by %s", post.Author.Username),
		Post:        post,
		Replies:     replies,
	})
}

func (app *App) handleViewPostRevision(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}
	revisionNumber, err := strconv.Atoi(r.PathValue("revision"))
	if err != nil || revisionNumber < 1 {
		app.renderNotFound(w, r)
		return
	}

	post, err := app.GetPostRevision(id, revisionNumber)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}
	post.Author, err = app.getUserByID(post.AuthorID)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}
	if post.QuoteOfID != nil && post.QuoteOfRevisionID != nil {
		post.QuotedPost, _ = app.GetPostRevisionWithAuthor(*post.QuoteOfID, *post.QuoteOfRevisionID)
	}

	user := GetCurrentUser(r)
	var uid int64
	if user != nil {
		uid = user.ID
	}
	app.HydratePosts([]*Post{post}, uid)

	replies, err := app.GetReplies(id)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Post Error", "Failed to load replies.")
		return
	}
	app.HydratePosts(replies, uid)
	if user != nil && post.IsReposted {
		if revisionID, err := app.GetRepostRevisionID(user.ID, post.ID); err == nil {
			post.ViewerRepostRevisionID = revisionID
			if repostRevision, err := app.GetPostRevisionByID(post.ID, revisionID); err == nil {
				post.ViewerRepostRevisionNumber = repostRevision.RevisionNumber
			}
		}
	}

	app.render(w, "post.html", &PageData{
		CurrentUser: user,
		Title:       fmt.Sprintf("Revision %d of post by %s", post.RevisionNumber, post.Author.Username),
		Post:        post,
		Replies:     replies,
	})
}

func (app *App) handlePostRevisions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	post, err := app.GetPostWithAuthor(id)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}
	revisions, err := app.GetPostRevisions(id)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Revision Error", "Failed to load revisions.")
		return
	}

	user := GetCurrentUser(r)
	var uid int64
	if user != nil {
		uid = user.ID
	}
	app.HydratePosts([]*Post{post}, uid)

	app.render(w, "revisions.html", &PageData{
		CurrentUser: user,
		Title:       fmt.Sprintf("Revisions for post by %s", post.Author.Username),
		Post:        post,
		Revisions:   revisions,
	})
}

// --- Compose ---

func (app *App) handleCompose(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	data := &PageData{
		CurrentUser: user,
		Title:       "New Post",
		FormAction:  "/posts",
		SubmitLabel: "Post",
		CancelURL:   "/",
	}

	// Quote post
	if parentID := r.URL.Query().Get("reply_to"); parentID != "" {
		id, err := strconv.ParseInt(parentID, 10, 64)
		if err == nil {
			data.ParentPost, _ = app.GetPostWithAuthor(id)
			if data.ParentPost != nil {
				data.Title = "Reply"
				data.SubmitLabel = "Reply"
				data.CancelURL = fmt.Sprintf("/posts/%d", data.ParentPost.ID)
			}
		}
	}

	// Quote post
	if qid := r.URL.Query().Get("quote"); qid != "" {
		id, err := strconv.ParseInt(qid, 10, 64)
		if err == nil {
			if revision := r.URL.Query().Get("revision"); revision != "" {
				if revisionNumber, err := strconv.Atoi(revision); err == nil && revisionNumber > 0 {
					postRevision, err := app.GetPostRevision(id, revisionNumber)
					if err == nil {
						data.QuotePost, _ = app.GetPostRevisionWithAuthor(id, postRevision.RevisionID)
					}
				}
			}
			if data.QuotePost == nil {
				data.QuotePost, _ = app.GetPostWithAuthor(id)
			}
		}
	}

	app.render(w, "compose.html", data)
}

func (app *App) handleEditPostForm(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	post, err := app.GetPostWithAuthor(id)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}
	if post.AuthorID != user.ID {
		app.renderStatus(w, r, http.StatusForbidden, "Forbidden", "You cannot edit this post.")
		return
	}

	app.render(w, "compose.html", &PageData{
		CurrentUser: user,
		Title:       "Edit Post",
		Post:        post,
		FormAction:  fmt.Sprintf("/posts/%d/edit", post.ID),
		SubmitLabel: "Save revision",
		CancelURL:   fmt.Sprintf("/posts/%d", post.ID),
		IsEditing:   true,
	})
}

// --- Create Post ---

func (app *App) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)

	rawContent := r.FormValue("content")
	content, sanitizeErr := sanitizePostContent(rawContent)
	if sanitizeErr != nil {
		content = rawContent
	}
	content = strings.TrimSpace(content)
	data := &PageData{
		CurrentUser: user,
		Title:       "New Post",
		FormAction:  "/posts",
		SubmitLabel: "Post",
		CancelURL:   "/",
		Post:        &Post{Content: content},
	}

	var parentPostID *int64
	var parentPostRevisionID *int64
	var quoteOfID *int64
	var quoteOfRevisionID *int64
	if pid := r.FormValue("parent_post_id"); pid != "" {
		if id, err := strconv.ParseInt(pid, 10, 64); err == nil {
			parentPostID = &id
		}
	}
	if prid := r.FormValue("parent_post_revision_id"); prid != "" {
		if id, err := strconv.ParseInt(prid, 10, 64); err == nil {
			parentPostRevisionID = &id
		}
	}
	if qid := r.FormValue("quote_of_id"); qid != "" {
		if id, err := strconv.ParseInt(qid, 10, 64); err == nil {
			quoteOfID = &id
		}
	}
	if qrid := r.FormValue("quote_of_revision_id"); qrid != "" {
		if id, err := strconv.ParseInt(qrid, 10, 64); err == nil {
			quoteOfRevisionID = &id
		}
	}
	if parentPostID != nil {
		data.ParentPost, _ = app.GetPostWithAuthor(*parentPostID)
		data.Title = "Reply"
		data.SubmitLabel = "Reply"
		data.CancelURL = fmt.Sprintf("/posts/%d", *parentPostID)
	}
	if quoteOfID != nil && quoteOfRevisionID != nil {
		data.QuotePost, _ = app.GetPostRevisionWithAuthor(*quoteOfID, *quoteOfRevisionID)
	} else if quoteOfID != nil {
		data.QuotePost, _ = app.GetPostWithAuthor(*quoteOfID)
	}

	if sanitizeErr != nil {
		data.Error = "Post contains unsupported control or invisible characters."
		app.render(w, "compose.html", data)
		return
	}

	if content == "" {
		data.Error = "Post content cannot be empty."
		app.render(w, "compose.html", data)
		return
	}
	if len(content) > maxPostLength {
		data.Error = fmt.Sprintf("Post content too long (max %d characters).", maxPostLength)
		app.render(w, "compose.html", data)
		return
	}

	// Rate limit
	count, err := app.CountRecentPosts(user.ID, time.Now().Add(-1*time.Hour))
	if err == nil && count >= postsPerHourMax {
		data.Error = "You're posting too frequently. Please wait a bit."
		app.render(w, "compose.html", data)
		return
	}

	contentHTML, err := RenderMarkdown(content)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Compose Error", "Failed to render markdown.")
		return
	}

	postID, err := app.CreatePost(user.ID, content, contentHTML, parentPostID, parentPostRevisionID, quoteOfID, quoteOfRevisionID)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Compose Error", "Failed to create post.")
		return
	}

	if parentPostID != nil {
		http.Redirect(w, r, fmt.Sprintf("/posts/%d", *parentPostID), http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/posts/%d", postID), http.StatusFound)
}

func (app *App) handleEditPost(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	post, err := app.GetPostWithAuthor(id)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}
	if post.AuthorID != user.ID {
		app.renderStatus(w, r, http.StatusForbidden, "Forbidden", "You cannot edit this post.")
		return
	}

	rawContent := r.FormValue("content")
	content, sanitizeErr := sanitizePostContent(rawContent)
	if sanitizeErr != nil {
		content = rawContent
	}
	content = strings.TrimSpace(content)
	post.Content = content
	data := &PageData{
		CurrentUser: user,
		Title:       "Edit Post",
		Post:        post,
		FormAction:  fmt.Sprintf("/posts/%d/edit", post.ID),
		SubmitLabel: "Save revision",
		CancelURL:   fmt.Sprintf("/posts/%d", post.ID),
		IsEditing:   true,
	}

	if sanitizeErr != nil {
		post.Content = rawContent
		data.Error = "Post contains unsupported control or invisible characters."
		app.render(w, "compose.html", data)
		return
	}

	if content == "" {
		data.Error = "Post content cannot be empty."
		app.render(w, "compose.html", data)
		return
	}
	if len(content) > maxPostLength {
		data.Error = fmt.Sprintf("Post content too long (max %d characters).", maxPostLength)
		app.render(w, "compose.html", data)
		return
	}

	contentHTML, err := RenderMarkdown(content)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Edit Error", "Failed to render markdown.")
		return
	}

	if _, err := app.EditPost(id, user.ID, content, contentHTML); err != nil {
		if err == errPostUnchanged {
			data.Error = "No changes to save."
			app.render(w, "compose.html", data)
			return
		}
		app.renderStatus(w, r, http.StatusInternalServerError, "Edit Error", "Failed to save revision.")
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/posts/%d", id), http.StatusFound)
}

// --- Delete Post ---

func (app *App) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	post, err := app.GetPost(id)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	if err := app.DeletePost(id, user.ID); err != nil {
		app.renderStatus(w, r, http.StatusForbidden, "Forbidden", "You cannot delete this post.")
		return
	}

	// htmx: top-level deletes navigate away, replies just disappear.
	if r.Header.Get("HX-Request") == "true" {
		if post.ParentPostID == nil {
			w.Header().Set("HX-Redirect", "/")
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	}

	if referer := r.Header.Get("Referer"); referer != "" {
		http.Redirect(w, r, referer, http.StatusFound)
		return
	}
	if post.ParentPostID != nil {
		http.Redirect(w, r, fmt.Sprintf("/posts/%d", *post.ParentPostID), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// --- Toggle Like ---

func (app *App) handleToggleLike(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	liked, err := app.ToggleLike(user.ID, id)
	if err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	// Re-fetch the post to get updated count
	post, err := app.GetPost(id)
	if err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	post.IsLiked = liked

	// Return the updated like button fragment
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cls := "action-btn like-btn"
	if liked {
		cls += " active"
	}
	fmt.Fprintf(w, `<button class="%s" hx-post="/posts/%d/like" hx-swap="outerHTML">
		<svg class="icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg> <span class="count">%d</span>
	</button>`, cls, post.ID, post.LikeCount)
}

// --- Toggle Repost ---

func (app *App) handleToggleRepost(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	post, err := app.GetPost(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	revisionID := post.CurrentRevisionID
	if rawRevisionID := r.FormValue("revision_id"); rawRevisionID != "" {
		if parsedRevisionID, err := strconv.ParseInt(rawRevisionID, 10, 64); err == nil {
			if _, err := app.GetPostRevisionByID(id, parsedRevisionID); err == nil {
				revisionID = parsedRevisionID
			}
		}
	}

	reposted, err := app.ToggleRepost(user.ID, id, revisionID)
	if err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	post, err = app.GetPost(id)
	if err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	post.IsReposted = reposted

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cls := "action-btn repost-btn"
	if reposted {
		cls += " active"
	}
	fmt.Fprintf(w, `<button class="%s" hx-post="/posts/%d/repost" hx-swap="outerHTML">
		<svg class="icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17 1l4 4-4 4"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><path d="M7 23l-4-4 4-4"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg> <span class="count">%d</span>
	</button>`, cls, post.ID, post.RepostCount)
}

// --- Toggle Bookmark ---

func (app *App) handleToggleBookmark(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	bookmarked, err := app.ToggleBookmark(user.ID, id)
	if err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cls := "action-btn bookmark-btn"
	if bookmarked {
		cls += " active"
	}
	fmt.Fprintf(w, `<button class="%s" hx-post="/posts/%d/bookmark" hx-swap="outerHTML" title="Bookmark">
		<svg class="icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>
	</button>`, cls, id)
}

// --- Bookmarks Page ---

func (app *App) handleBookmarks(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)

	beforeCursor := r.URL.Query().Get("before")

	posts, nextCursor, err := app.GetBookmarkedPosts(user.ID, postsPerPage+1, beforeCursor)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Bookmarks Error", "Failed to load bookmarks.")
		return
	}

	hasMore := len(posts) > postsPerPage
	if hasMore {
		posts = posts[:postsPerPage]
	}

	app.HydratePosts(posts, user.ID)

	data := &PageData{
		CurrentUser: user,
		Title:       "Bookmarks",
		Posts:       posts,
		HasMore:     hasMore,
	}
	if hasMore {
		data.LoadMoreURL = "/bookmarks?before=" + url.QueryEscape(nextCursor)
	}

	if r.Header.Get("HX-Request") == "true" && beforeCursor != "" {
		app.renderFragment(w, data)
		return
	}

	app.render(w, "bookmarks.html", data)
}

// --- Replies ---

func (app *App) handleCreateReply(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	redirectURL := fmt.Sprintf("/posts/%d", postID)
	if referer := r.Header.Get("Referer"); referer != "" {
		redirectURL = referer
	}

	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" || len(content) > maxReplyLength {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	post, err := app.GetPost(postID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	postRevisionID := post.CurrentRevisionID
	if rawRevisionID := r.FormValue("post_revision_id"); rawRevisionID != "" {
		if parsedRevisionID, err := strconv.ParseInt(rawRevisionID, 10, 64); err == nil {
			if _, err := app.GetPostRevisionByID(postID, parsedRevisionID); err == nil {
				postRevisionID = parsedRevisionID
			}
		}
	}

	// Rate limit
	count, _ := app.CountRecentReplies(user.ID, time.Now().Add(-1*time.Hour))
	if count >= repliesPerHour {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	contentHTML, err := RenderMarkdown(content)
	if err != nil {
		http.Error(w, "Failed to render markdown", http.StatusInternalServerError)
		return
	}

	if _, err := app.CreatePost(user.ID, content, contentHTML, &postID, &postRevisionID, nil, nil); err != nil {
		http.Error(w, "Failed to create reply", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// --- Image Upload ---

func (app *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)

	// Rate limit uploads (fail closed: block on DB error)
	count, err := app.CountRecentUploads(user.ID, time.Now().Add(-1*time.Hour))
	if err != nil || count >= uploadsPerHourMax {
		http.Error(w, "Too many uploads. Please wait a bit.", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageSize+1024) // small buffer for form overhead

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to read upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	ext, ok := allowedImageTypes[ct]
	if !ok {
		http.Error(w, "Unsupported image type. Use PNG, JPEG, GIF, or WebP.", http.StatusBadRequest)
		return
	}

	if header.Size > maxImageSize {
		http.Error(w, fmt.Sprintf("Image too large (max %dMB)", maxImageSize>>20), http.StatusBadRequest)
		return
	}

	// Generate unique filename
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	filename := hex.EncodeToString(randBytes) + ext

	outPath := filepath.Join("uploads", filename)
	out, err := os.Create(outPath)
	if err != nil {
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		os.Remove(outPath)
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}

	// Track upload for rate limiting (fail closed: remove file if we can't record it)
	if err := app.RecordUpload(user.ID, filename, header.Size); err != nil {
		os.Remove(outPath)
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}

	// Return the markdown image syntax
	w.Header().Set("Content-Type", "application/json")
	imgURL := fmt.Sprintf("/uploads/%s", filename)
	fmt.Fprintf(w, `{"url":"%s","markdown":"![image](%s)"}`, imgURL, imgURL)
}

// --- Raw Markdown ---

func (app *App) handlePostRaw(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var post *Post
	if revision := r.URL.Query().Get("revision"); revision != "" {
		revisionNumber, err := strconv.Atoi(revision)
		if err != nil || revisionNumber < 1 {
			http.NotFound(w, r)
			return
		}
		post, err = app.GetPostRevision(id, revisionNumber)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	} else {
		post, err = app.GetPost(id)
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(post.Content))
}

// --- Markdown Preview ---

func (app *App) handlePreview(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	if len(content) > maxPostLength {
		content = content[:maxPostLength]
	}

	html, err := RenderMarkdown(content)
	if err != nil {
		http.Error(w, "Failed to render", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="markdown-body">%s</div>`, html)
}

// --- Profile ---

func (app *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	profile, err := app.GetUserByUsername(username)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	followers, following, postCount, _ := app.GetUserStats(profile.ID)
	profile.FollowerCount = followers
	profile.FollowingCount = following
	profile.PostCount = postCount

	user := GetCurrentUser(r)
	if user != nil && user.ID != profile.ID {
		profile.IsFollowing = app.IsFollowing(user.ID, profile.ID)
	}

	beforeID := int64(0)
	if v := r.URL.Query().Get("before"); v != "" {
		beforeID, _ = strconv.ParseInt(v, 10, 64)
	}

	activeTab := r.URL.Query().Get("tab")
	if activeTab != "replies" {
		activeTab = "posts"
	}

	var posts []*Post
	if activeTab == "replies" {
		posts, _ = app.GetUserReplies(profile.ID, postsPerPage+1, beforeID)
	} else {
		posts, _ = app.GetUserPosts(profile.ID, postsPerPage+1, beforeID)
	}
	hasMore := len(posts) > postsPerPage
	if hasMore {
		posts = posts[:postsPerPage]
	}

	var uid int64
	if user != nil {
		uid = user.ID
	}
	app.HydratePosts(posts, uid)

	data := &PageData{
		CurrentUser: user,
		Title:       profile.DisplayName,
		Profile:     profile,
		Posts:       posts,
		HasMore:     hasMore,
		ActiveTab:   activeTab,
	}
	if hasMore && len(posts) > 0 {
		data.NextCursor = posts[len(posts)-1].ID
		data.LoadMoreURL = fmt.Sprintf("/user/%s?tab=%s&before=%d", profile.Username, activeTab, data.NextCursor)
	}

	if r.Header.Get("HX-Request") == "true" && beforeID > 0 {
		app.renderFragment(w, data)
		return
	}

	app.render(w, "profile.html", data)
}

// --- Follow ---

func (app *App) handleToggleFollow(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r)
	username := r.PathValue("username")

	target, err := app.GetUserByUsername(username)
	if err != nil {
		app.renderNotFound(w, r)
		return
	}

	following, err := app.ToggleFollow(user.ID, target.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if following {
		fmt.Fprintf(w, `<button class="btn btn-secondary" hx-post="/user/%s/follow" hx-swap="outerHTML">Unfollow</button>`, target.Username)
	} else {
		fmt.Fprintf(w, `<button class="btn btn-primary" hx-post="/user/%s/follow" hx-swap="outerHTML">Follow</button>`, target.Username)
	}
}

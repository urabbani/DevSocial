package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (app *App) handlePostMarkdown(w http.ResponseWriter, r *http.Request) {
	post, err := app.loadMarkdownPost(r.PathValue("id"), r.URL.Query().Get("revision"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(app.renderPostMarkdown(post)))
}

func (app *App) handleUserMarkdown(w http.ResponseWriter, r *http.Request) {
	user, err := app.GetUserByUsername(r.PathValue("username"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	followers, following, postCount, err := app.GetUserStats(user.ID)
	if err != nil {
		http.Error(w, "Failed to load user.", http.StatusInternalServerError)
		return
	}
	user.FollowerCount = followers
	user.FollowingCount = following
	user.PostCount = postCount

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(app.renderUserMarkdown(user)))
}

func (app *App) loadMarkdownPost(idValue, revisionValue string) (*Post, error) {
	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil {
		return nil, err
	}

	if revisionValue != "" {
		revisionNumber, err := strconv.Atoi(revisionValue)
		if err != nil || revisionNumber < 1 {
			return nil, fmt.Errorf("invalid revision")
		}
		post, err := app.GetPostRevision(id, revisionNumber)
		if err != nil {
			return nil, err
		}
		post.Author, err = app.getUserByID(post.AuthorID)
		if err != nil {
			return nil, err
		}
		if post.ParentPostID != nil {
			if post.ParentPostRevisionID != nil {
				post.ParentPost, _ = app.GetPostRevisionWithAuthor(*post.ParentPostID, *post.ParentPostRevisionID)
			} else {
				post.ParentPost, _ = app.GetPostWithAuthor(*post.ParentPostID)
			}
		}
		if post.QuoteOfID != nil && post.QuoteOfRevisionID != nil {
			post.QuotedPost, _ = app.GetPostRevisionWithAuthor(*post.QuoteOfID, *post.QuoteOfRevisionID)
		}
		return post, nil
	}

	post, err := app.GetPostWithAuthor(id)
	if err != nil {
		return nil, err
	}
	if err := app.HydratePosts([]*Post{post}, 0); err != nil {
		return nil, err
	}
	return post, nil
}

func (app *App) renderPostMarkdown(post *Post) string {
	var b strings.Builder
	writeFrontmatterStart(&b)
	writeFrontmatterString(&b, "kind", "post")
	writeFrontmatterInt(&b, "id", post.ID)
	writeFrontmatterString(&b, "url", app.absoluteURL(fmt.Sprintf("/posts/%d", post.ID)))
	if post.Author != nil {
		writeFrontmatterString(&b, "author_username", post.Author.Username)
		writeFrontmatterString(&b, "author_display_name", post.Author.DisplayName)
		writeFrontmatterString(&b, "author_url", app.absoluteURL(fmt.Sprintf("/user/%s", post.Author.Username)))
		writeFrontmatterString(&b, "author_github_url", fmt.Sprintf("https://github.com/%s", post.Author.Username))
	}
	writeFrontmatterTime(&b, "created_at", post.CreatedAt)
	if post.EditedAt != nil {
		writeFrontmatterTime(&b, "edited_at", *post.EditedAt)
	}
	writeFrontmatterInt(&b, "revision_id", post.RevisionID)
	writeFrontmatterInt(&b, "revision_number", int64(post.RevisionNumber))
	writeFrontmatterInt(&b, "revision_count", int64(post.RevisionCount))
	writeFrontmatterTime(&b, "revision_created_at", post.RevisionCreatedAt)
	if post.ParentPostID != nil {
		writeFrontmatterInt(&b, "parent_post_id", *post.ParentPostID)
	}
	if post.ParentPostRevisionID != nil {
		writeFrontmatterInt(&b, "parent_post_revision_id", *post.ParentPostRevisionID)
	}
	if post.QuoteOfID != nil {
		writeFrontmatterInt(&b, "quote_of_id", *post.QuoteOfID)
	}
	if post.QuoteOfRevisionID != nil {
		writeFrontmatterInt(&b, "quote_of_revision_id", *post.QuoteOfRevisionID)
	}
	writeFrontmatterInt(&b, "reply_count", int64(post.ReplyCount))
	writeFrontmatterInt(&b, "like_count", int64(post.LikeCount))
	writeFrontmatterInt(&b, "repost_count", int64(post.RepostCount))
	writeFrontmatterEnd(&b)
	b.WriteString(app.apiMarkdown(post.Content))
	if !strings.HasSuffix(post.Content, "\n") {
		b.WriteByte('\n')
	}
	return b.String()
}

func (app *App) renderUserMarkdown(user *User) string {
	var b strings.Builder
	writeFrontmatterStart(&b)
	writeFrontmatterString(&b, "kind", "user")
	writeFrontmatterInt(&b, "id", user.ID)
	writeFrontmatterString(&b, "username", user.Username)
	writeFrontmatterString(&b, "display_name", user.DisplayName)
	writeFrontmatterString(&b, "profile_url", app.absoluteURL(fmt.Sprintf("/user/%s", user.Username)))
	writeFrontmatterString(&b, "github_url", fmt.Sprintf("https://github.com/%s", user.Username))
	writeFrontmatterString(&b, "avatar_url", user.AvatarURL)
	writeFrontmatterString(&b, "feed_url", app.absoluteURL(fmt.Sprintf("/user/%s/feed.xml", user.Username)))
	writeFrontmatterTime(&b, "created_at", user.CreatedAt)
	writeFrontmatterInt(&b, "follower_count", int64(user.FollowerCount))
	writeFrontmatterInt(&b, "following_count", int64(user.FollowingCount))
	writeFrontmatterInt(&b, "post_count", int64(user.PostCount))
	writeFrontmatterEnd(&b)
	b.WriteString("# @")
	b.WriteString(user.Username)
	b.WriteString("\n\n")
	if user.DisplayName != "" {
		b.WriteString(user.DisplayName)
		b.WriteString("\n\n")
	}
	b.WriteString("- Profile: ")
	b.WriteString(app.absoluteURL(fmt.Sprintf("/user/%s", user.Username)))
	b.WriteString("\n")
	b.WriteString("- GitHub: https://github.com/")
	b.WriteString(user.Username)
	b.WriteString("\n")
	b.WriteString("- RSS: ")
	b.WriteString(app.absoluteURL(fmt.Sprintf("/user/%s/feed.xml", user.Username)))
	b.WriteString("\n")
	return b.String()
}

func writeFrontmatterStart(b *strings.Builder) {
	b.WriteString("---\n")
}

func writeFrontmatterEnd(b *strings.Builder) {
	b.WriteString("---\n\n")
}

func writeFrontmatterString(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(strconv.Quote(value))
	b.WriteByte('\n')
}

func writeFrontmatterInt(b *strings.Builder, key string, value int64) {
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(strconv.FormatInt(value, 10))
	b.WriteByte('\n')
}

func writeFrontmatterTime(b *strings.Builder, key string, value time.Time) {
	writeFrontmatterString(b, key, value.UTC().Format(time.RFC3339))
}

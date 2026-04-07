package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type apiUser struct {
	ID             int64    `json:"id"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"display_name"`
	AvatarURL      string   `json:"avatar_url"`
	GitHubURL      string   `json:"github_url"`
	ProfileURL     string   `json:"profile_url"`
	FeedURL        string   `json:"feed_url"`
	CreatedAt      JSONTime `json:"created_at"`
	FollowerCount  int      `json:"follower_count"`
	FollowingCount int      `json:"following_count"`
	PostCount      int      `json:"post_count"`
}

type apiUserRef struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url"`
	ProfileURL  string   `json:"profile_url"`
	GitHubURL   string   `json:"github_url"`
	CreatedAt   JSONTime `json:"created_at"`
}

type apiPostRef struct {
	ID             int64      `json:"id"`
	URL            string     `json:"url"`
	Author         apiUserRef `json:"author"`
	RevisionNumber int        `json:"revision_number"`
	RevisionCount  int        `json:"revision_count"`
}

type apiPost struct {
	ID                   int64       `json:"id"`
	URL                  string      `json:"url"`
	Author               apiUserRef  `json:"author"`
	ContentMarkdown      string      `json:"content_markdown"`
	CreatedAt            JSONTime    `json:"created_at"`
	EditedAt             *JSONTime   `json:"edited_at,omitempty"`
	LikeCount            int         `json:"like_count"`
	RepostCount          int         `json:"repost_count"`
	ReplyCount           int         `json:"reply_count"`
	RevisionID           int64       `json:"revision_id"`
	RevisionNumber       int         `json:"revision_number"`
	RevisionCount        int         `json:"revision_count"`
	RevisionCreatedAt    JSONTime    `json:"revision_created_at"`
	ParentPostID         *int64      `json:"parent_post_id,omitempty"`
	ParentPostRevisionID *int64      `json:"parent_post_revision_id,omitempty"`
	QuoteOfID            *int64      `json:"quote_of_id,omitempty"`
	QuoteOfRevisionID    *int64      `json:"quote_of_revision_id,omitempty"`
	Depth                int         `json:"depth,omitempty"`
	ParentPost           *apiPostRef `json:"parent_post,omitempty"`
	QuotedPost           *apiPostRef `json:"quoted_post,omitempty"`
}

type apiPostListResponse struct {
	User       apiUser   `json:"user"`
	Posts      []apiPost `json:"posts"`
	HasMore    bool      `json:"has_more"`
	NextCursor int64     `json:"next_cursor,omitempty"`
}

type apiPostsQueryResponse struct {
	Posts      []apiPost `json:"posts"`
	HasMore    bool      `json:"has_more"`
	NextCursor int64     `json:"next_cursor,omitempty"`
}

type apiPostResponse struct {
	Post apiPost `json:"post"`
}

type apiPostThreadResponse struct {
	Post    apiPost   `json:"post"`
	Replies []apiPost `json:"replies"`
}

type apiRevision struct {
	ID              int64    `json:"id"`
	RevisionNumber  int      `json:"revision_number"`
	ContentMarkdown string   `json:"content_markdown"`
	CreatedAt       JSONTime `json:"created_at"`
}

type apiRevisionsResponse struct {
	Post      apiPost       `json:"post"`
	Revisions []apiRevision `json:"revisions"`
}

type JSONTime time.Time

var (
	inlineRelativeMarkdownURL    = regexp.MustCompile(`\]\(/([^/])`)
	referenceRelativeMarkdownURL = regexp.MustCompile(`(?m)^(\[[^\]]+\]:\s*)/([^/])`)
)

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	*t = JSONTime(parsed)
	return nil
}

func (app *App) handleAPIUser(w http.ResponseWriter, r *http.Request) {
	profile, err := app.GetUserByUsername(r.PathValue("username"))
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "User not found.")
		return
	}

	followers, following, postCount, err := app.GetUserStats(profile.ID)
	if err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to load user.")
		return
	}
	profile.FollowerCount = followers
	profile.FollowingCount = following
	profile.PostCount = postCount

	app.writeJSON(w, http.StatusOK, app.apiUser(profile))
}

func (app *App) handleAPIUserPosts(w http.ResponseWriter, r *http.Request) {
	app.handleAPIUserPostList(w, r, false)
}

func (app *App) handleAPIUserReplies(w http.ResponseWriter, r *http.Request) {
	app.handleAPIUserPostList(w, r, true)
}

func (app *App) handleAPIPosts(w http.ResponseWriter, r *http.Request) {
	beforeID, ok := parseBeforeID(r)
	if !ok {
		app.writeJSONError(w, http.StatusBadRequest, "Invalid before cursor.")
		return
	}
	limit, ok := clampPostsQueryLimit(r.URL.Query().Get("limit"), postsPerPage)
	if !ok {
		app.writeJSONError(w, http.StatusBadRequest, "Invalid limit.")
		return
	}

	var query PostQuery
	query.BeforeID = beforeID
	query.Limit = limit + 1

	if authorUsername := strings.TrimSpace(r.URL.Query().Get("author")); authorUsername != "" {
		author, err := app.GetUserByUsername(authorUsername)
		if err != nil {
			app.writeJSONError(w, http.StatusNotFound, "User not found.")
			return
		}
		query.AuthorID = &author.ID
	}

	hasParentValue := strings.TrimSpace(r.URL.Query().Get("has_parent"))
	if hasParentValue != "" {
		hasParent, ok := parseBoolQueryValue(hasParentValue)
		if !ok {
			app.writeJSONError(w, http.StatusBadRequest, "Invalid has_parent value.")
			return
		}
		query.HasParent = &hasParent
	}

	if parentValue := strings.TrimSpace(r.URL.Query().Get("parent_post_id")); parentValue != "" {
		parentPostID, err := strconv.ParseInt(parentValue, 10, 64)
		if err != nil || parentPostID < 1 {
			app.writeJSONError(w, http.StatusBadRequest, "Invalid parent_post_id.")
			return
		}
		query.ParentPostID = &parentPostID
		hasParent := true
		query.HasParent = &hasParent
	}

	posts, err := app.GetPosts(query)
	if err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to load posts.")
		return
	}
	if err := app.HydratePosts(posts, 0); err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to hydrate posts.")
		return
	}

	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}

	resp := apiPostsQueryResponse{
		Posts:   app.apiPosts(posts),
		HasMore: hasMore,
	}
	if hasMore && len(posts) > 0 {
		resp.NextCursor = posts[len(posts)-1].ID
	}

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *App) handleAPIUserPostList(w http.ResponseWriter, r *http.Request, replies bool) {
	profile, err := app.GetUserByUsername(r.PathValue("username"))
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "User not found.")
		return
	}

	followers, following, postCount, err := app.GetUserStats(profile.ID)
	if err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to load user.")
		return
	}
	profile.FollowerCount = followers
	profile.FollowingCount = following
	profile.PostCount = postCount

	beforeID, ok := parseBeforeID(r)
	if !ok {
		app.writeJSONError(w, http.StatusBadRequest, "Invalid before cursor.")
		return
	}

	var posts []*Post
	if replies {
		posts, err = app.GetUserReplies(profile.ID, postsPerPage+1, beforeID)
	} else {
		posts, err = app.GetUserPosts(profile.ID, postsPerPage+1, beforeID)
	}
	if err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to load posts.")
		return
	}
	if err := app.HydratePosts(posts, 0); err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to hydrate posts.")
		return
	}

	hasMore := len(posts) > postsPerPage
	if hasMore {
		posts = posts[:postsPerPage]
	}

	resp := apiPostListResponse{
		User:    app.apiUser(profile),
		Posts:   app.apiPosts(posts),
		HasMore: hasMore,
	}
	if hasMore && len(posts) > 0 {
		resp.NextCursor = posts[len(posts)-1].ID
	}

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *App) handleAPIPost(w http.ResponseWriter, r *http.Request) {
	post, err := app.loadAPIPost(r.PathValue("id"))
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "Post not found.")
		return
	}

	app.writeJSON(w, http.StatusOK, apiPostResponse{Post: app.apiPost(post)})
}

func (app *App) handleAPIPostThread(w http.ResponseWriter, r *http.Request) {
	post, err := app.loadAPIPost(r.PathValue("id"))
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "Post not found.")
		return
	}

	replies, err := app.GetReplies(post.ID)
	if err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to load thread.")
		return
	}
	if err := app.HydratePosts(replies, 0); err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to hydrate thread.")
		return
	}

	app.writeJSON(w, http.StatusOK, apiPostThreadResponse{
		Post:    app.apiPost(post),
		Replies: app.apiPosts(replies),
	})
}

func (app *App) handleAPIPostRevisions(w http.ResponseWriter, r *http.Request) {
	post, err := app.loadAPIPost(r.PathValue("id"))
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "Post not found.")
		return
	}

	revisions, err := app.GetPostRevisions(post.ID)
	if err != nil {
		app.writeJSONError(w, http.StatusInternalServerError, "Failed to load revisions.")
		return
	}

	apiRevisions := make([]apiRevision, 0, len(revisions))
	for _, revision := range revisions {
		apiRevisions = append(apiRevisions, apiRevision{
			ID:              revision.ID,
			RevisionNumber:  revision.RevisionNumber,
			ContentMarkdown: app.apiMarkdown(revision.Content),
			CreatedAt:       JSONTime(revision.CreatedAt),
		})
	}

	app.writeJSON(w, http.StatusOK, apiRevisionsResponse{
		Post:      app.apiPost(post),
		Revisions: apiRevisions,
	})
}

func (app *App) handleAPIPostRevision(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "Post not found.")
		return
	}
	revisionNumber, err := strconv.Atoi(r.PathValue("revision"))
	if err != nil || revisionNumber < 1 {
		app.writeJSONError(w, http.StatusNotFound, "Revision not found.")
		return
	}

	post, err := app.GetPostRevision(id, revisionNumber)
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "Revision not found.")
		return
	}
	post.Author, err = app.getUserByID(post.AuthorID)
	if err != nil {
		app.writeJSONError(w, http.StatusNotFound, "Revision not found.")
		return
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

	app.writeJSON(w, http.StatusOK, apiPostResponse{Post: app.apiPost(post)})
}

func (app *App) loadAPIPost(idValue string) (*Post, error) {
	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil {
		return nil, err
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

func (app *App) apiUser(u *User) apiUser {
	return apiUser{
		ID:             u.ID,
		Username:       u.Username,
		DisplayName:    u.DisplayName,
		AvatarURL:      u.AvatarURL,
		GitHubURL:      fmt.Sprintf("https://github.com/%s", u.Username),
		ProfileURL:     app.absoluteURL(fmt.Sprintf("/user/%s", u.Username)),
		FeedURL:        app.absoluteURL(fmt.Sprintf("/user/%s/feed.xml", u.Username)),
		CreatedAt:      JSONTime(u.CreatedAt),
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		PostCount:      u.PostCount,
	}
}

func (app *App) apiUserRef(u *User) apiUserRef {
	return apiUserRef{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		ProfileURL:  app.absoluteURL(fmt.Sprintf("/user/%s", u.Username)),
		GitHubURL:   fmt.Sprintf("https://github.com/%s", u.Username),
		CreatedAt:   JSONTime(u.CreatedAt),
	}
}

func (app *App) apiPostRef(p *Post) *apiPostRef {
	if p == nil || p.Author == nil {
		return nil
	}
	return &apiPostRef{
		ID:             p.ID,
		URL:            app.absoluteURL(fmt.Sprintf("/posts/%d", p.ID)),
		Author:         app.apiUserRef(p.Author),
		RevisionNumber: p.RevisionNumber,
		RevisionCount:  p.RevisionCount,
	}
}

func (app *App) apiPost(p *Post) apiPost {
	var editedAt *JSONTime
	if p.EditedAt != nil {
		value := JSONTime(*p.EditedAt)
		editedAt = &value
	}
	return apiPost{
		ID:                   p.ID,
		URL:                  app.absoluteURL(fmt.Sprintf("/posts/%d", p.ID)),
		Author:               app.apiUserRef(p.Author),
		ContentMarkdown:      app.apiMarkdown(p.Content),
		CreatedAt:            JSONTime(p.CreatedAt),
		EditedAt:             editedAt,
		LikeCount:            p.LikeCount,
		RepostCount:          p.RepostCount,
		ReplyCount:           p.ReplyCount,
		RevisionID:           p.RevisionID,
		RevisionNumber:       p.RevisionNumber,
		RevisionCount:        p.RevisionCount,
		RevisionCreatedAt:    JSONTime(p.RevisionCreatedAt),
		ParentPostID:         p.ParentPostID,
		ParentPostRevisionID: p.ParentPostRevisionID,
		QuoteOfID:            p.QuoteOfID,
		QuoteOfRevisionID:    p.QuoteOfRevisionID,
		Depth:                p.Depth,
		ParentPost:           app.apiPostRef(p.ParentPost),
		QuotedPost:           app.apiPostRef(p.QuotedPost),
	}
}

func (app *App) apiPosts(posts []*Post) []apiPost {
	out := make([]apiPost, 0, len(posts))
	for _, post := range posts {
		out = append(out, app.apiPost(post))
	}
	return out
}

func (app *App) apiMarkdown(content string) string {
	absoluteRoot := app.absoluteURL("/")
	if absoluteRoot == "" {
		return content
	}
	absoluteRoot = strings.TrimSuffix(absoluteRoot, "/") + "/"
	content = inlineRelativeMarkdownURL.ReplaceAllString(content, "]("+absoluteRoot+"$1")
	content = referenceRelativeMarkdownURL.ReplaceAllString(content, "${1}"+absoluteRoot+"$2")
	return content
}

func (app *App) writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	enc.Encode(value)
}

func (app *App) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	app.writeJSON(w, statusCode, map[string]string{"error": message})
}

func parseBeforeID(r *http.Request) (int64, bool) {
	value := r.URL.Query().Get("before")
	if value == "" {
		return 0, true
	}
	beforeID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return beforeID, true
}

func parseBoolQueryValue(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	default:
		return false, false
	}
}

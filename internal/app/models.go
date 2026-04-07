package app

import "time"

type User struct {
	ID          int64
	GitHubID    int64
	Username    string
	DisplayName string
	AvatarURL   string
	Bio         string
	CreatedAt   time.Time
	// Computed fields (filled by queries as needed)
	FollowerCount  int
	FollowingCount int
	PostCount      int
	IsFollowing    bool // whether current user follows this user
}

type PostRevision struct {
	ID             int64
	PostID         int64
	RevisionNumber int
	Content        string
	ContentHTML    string
	CreatedAt      time.Time
}

type Post struct {
	ID                   int64
	AuthorID             int64
	Content              string
	ContentHTML          string
	ParentPostID         *int64
	ParentPostRevisionID *int64
	QuoteOfID            *int64
	QuoteOfRevisionID    *int64
	CreatedAt            time.Time
	EditedAt             *time.Time
	LikeCount            int
	RepostCount          int
	ReplyCount           int
	RevisionID           int64
	RevisionNumber       int
	RevisionCreatedAt    time.Time
	CurrentRevisionID    int64
	RevisionCount        int
	Depth                int
	// Joined fields
	Author     *User
	ParentPost *Post
	QuotedPost *Post
	// Current user state
	IsLiked                    bool
	IsReposted                 bool
	IsBookmarked               bool
	ViewerRepostRevisionID     int64
	ViewerRepostRevisionNumber int
	// Timeline annotation
	RepostedBy *User
}

type ActivityItem struct {
	Type          string
	ActorID       int64
	Actor         *User
	CreatedAt     time.Time
	EventPostID   *int64
	SubjectPostID *int64
	EventPost     *Post
	SubjectPost   *Post
}

type PageData struct {
	CurrentUser  *User
	Title        string
	BodyClass    string
	StatusCode   int
	ErrorTitle   string
	ErrorMessage string
	Posts        []*Post
	Post         *Post
	Replies      []*Post
	Revisions    []*PostRevision
	Profile      *User
	QuotePost    *Post
	ParentPost   *Post
	NextCursor   int64
	HasMore      bool
	LoadMoreURL  string
	Error        string
	Flash        string
	FormAction   string
	SubmitLabel  string
	CancelURL    string
	IsEditing    bool
	ActiveTab    string
	DocsHTML     string
	DocsMarkdown string
	Activity     []*ActivityItem
}

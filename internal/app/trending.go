package app

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// Trending feed configuration. The feed is computed by a background goroutine
// every trendingRebuildInterval and stored in the trending_posts table. Reads
// against the trending tab are then plain SQL against that materialized view.
const (
	// Only posts created within this many days are eligible for trending.
	// The gravity formula already buries older posts; this is just pruning.
	trendingWindowDays = 7

	// Maximum number of posts kept in the trending_posts table.
	trendingTopN = 200

	// Decay exponent in the score denominator. 1.8 matches Hacker News.
	trendingDecayExponent = 1.8

	// How often the background goroutine rebuilds the trending table.
	trendingRebuildInterval = 3 * time.Minute
)

// startTrendingRebuilder runs an initial trending rebuild synchronously and
// then spawns a goroutine that rebuilds on a fixed interval. Errors are
// logged and otherwise ignored — a failed rebuild leaves the previous
// trending table in place. The goroutine has no shutdown story; it dies
// with the process, which matches the rest of this binary's lifecycle.
func (app *App) startTrendingRebuilder() {
	if err := app.RebuildTrending(); err != nil {
		log.Printf("trending: initial rebuild failed: %v", err)
	}
	go func() {
		ticker := time.NewTicker(trendingRebuildInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := app.RebuildTrending(); err != nil {
				log.Printf("trending: rebuild failed: %v", err)
			}
		}
	}()
}

// RebuildTrending recomputes the trending_posts table from posts in the last
// trendingWindowDays days. Posts are scored using a Hacker News-style gravity
// formula:
//
//	score = engagement / (age_hours + 2)^trendingDecayExponent
//
// where engagement = like_count + repost_count + comment_count. Posts with
// zero engagement are excluded entirely. The top trendingTopN scored posts
// are written to trending_posts in rank order (1 = highest score).
//
// The whole rebuild is wrapped in a single transaction so readers always see
// either the previous state or the new state, never a half-rebuilt table.
func (app *App) RebuildTrending() error {
	cutoff := time.Now().UTC().Add(-trendingWindowDays * 24 * time.Hour)

	rows, err := app.DB.Query(`
		SELECT id, created_at, like_count + repost_count + comment_count AS engagement
		FROM posts
		WHERE parent_post_id IS NULL
		  AND created_at > ?
		  AND (like_count + repost_count + comment_count) > 0
	`, cutoff)
	if err != nil {
		return fmt.Errorf("trending: query candidates: %w", err)
	}

	type scored struct {
		id    int64
		score float64
	}
	var candidates []scored
	now := time.Now().UTC()
	for rows.Next() {
		var id int64
		var createdAt time.Time
		var engagement int64
		if err := rows.Scan(&id, &createdAt, &engagement); err != nil {
			rows.Close()
			return fmt.Errorf("trending: scan candidate: %w", err)
		}
		ageHours := now.Sub(createdAt).Hours()
		if ageHours < 0 {
			ageHours = 0
		}
		score := float64(engagement) / math.Pow(ageHours+2, trendingDecayExponent)
		candidates = append(candidates, scored{id: id, score: score})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("trending: iterate candidates: %w", err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) > trendingTopN {
		candidates = candidates[:trendingTopN]
	}

	tx, err := app.DB.Begin()
	if err != nil {
		return fmt.Errorf("trending: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM trending_posts`); err != nil {
		return fmt.Errorf("trending: delete: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO trending_posts (rank, post_id, score) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("trending: prepare insert: %w", err)
	}
	defer stmt.Close()
	for i, c := range candidates {
		if _, err := stmt.Exec(i+1, c.id, c.score); err != nil {
			return fmt.Errorf("trending: insert rank %d: %w", i+1, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("trending: commit: %w", err)
	}
	return nil
}

// GetTrendingPosts returns posts from the trending_posts materialized table,
// ordered by precomputed rank. Pagination uses afterRank as a cursor: pass 0
// for the first page, then the rank of the last post returned for subsequent
// pages. Returns a parallel slice of ranks so callers can pick the right
// cursor value after their own slicing — important when the rank space has
// gaps (deletions cascade into trending_posts between rebuilds).
func (app *App) GetTrendingPosts(limit int, afterRank int64) ([]*Post, []int64, error) {
	rows, err := app.DB.Query(`
		SELECT t.rank,
		       p.id, p.author_id, p.content, p.content_html, p.parent_post_id, p.parent_post_revision_id,
		       p.quote_of_id, p.quote_of_revision_id,
		       p.created_at, p.edited_at, p.like_count, p.repost_count, p.comment_count,
		       p.current_revision_id, p.revision_count
		FROM trending_posts t
		JOIN posts p ON p.id = t.post_id
		WHERE t.rank > ?
		ORDER BY t.rank
		LIMIT ?
	`, afterRank, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var posts []*Post
	var ranks []int64
	for rows.Next() {
		p := &Post{}
		var rank int64
		var parentPostID sql.NullInt64
		var parentPostRevisionID sql.NullInt64
		var quoteOfID sql.NullInt64
		var quoteOfRevisionID sql.NullInt64
		var editedAt sql.NullTime
		if err := rows.Scan(
			&rank,
			&p.ID, &p.AuthorID, &p.Content, &p.ContentHTML, &parentPostID, &parentPostRevisionID,
			&quoteOfID, &quoteOfRevisionID,
			&p.CreatedAt, &editedAt, &p.LikeCount, &p.RepostCount, &p.ReplyCount,
			&p.CurrentRevisionID, &p.RevisionCount,
		); err != nil {
			return nil, nil, err
		}
		if parentPostID.Valid {
			p.ParentPostID = &parentPostID.Int64
		}
		if parentPostRevisionID.Valid {
			p.ParentPostRevisionID = &parentPostRevisionID.Int64
		}
		if quoteOfID.Valid {
			p.QuoteOfID = &quoteOfID.Int64
		}
		if quoteOfRevisionID.Valid {
			p.QuoteOfRevisionID = &quoteOfRevisionID.Int64
		}
		if editedAt.Valid {
			p.EditedAt = &editedAt.Time
		}
		p.RevisionID = p.CurrentRevisionID
		p.RevisionNumber = p.RevisionCount
		p.RevisionCreatedAt = p.CreatedAt
		posts = append(posts, p)
		ranks = append(ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return posts, ranks, nil
}

import { useEffect, useState } from 'react';
import { useFeedStore } from '../../stores/feed';
import { useWorkspaceStore } from '../../stores/workspace';
import type { Post } from '../../api/client';

export function FeedView() {
  const workspace = useWorkspaceStore((s) => s.activeWorkspace);
  const { posts, loading, hasMore, fetchFeed, createPost, toggleReaction, fetchReplies, repliesByPost, deletePost } = useFeedStore();
  const [newPost, setNewPost] = useState('');
  const [replyingTo, setReplyingTo] = useState<number | null>(null);
  const [replyContent, setReplyContent] = useState('');

  useEffect(() => {
    if (workspace) {
      fetchFeed(workspace.id);
    }
  }, [workspace?.id]);

  if (!workspace) {
    return <div className="p-6 text-[var(--text-secondary)]">Select a workspace</div>;
  }

  const handleSubmit = async () => {
    if (!newPost.trim()) return;
    try {
      await createPost(workspace.id, newPost.trim());
      setNewPost('');
    } catch {
      // Silently fail — user can retry
    }
  };

  const handleReply = async (postId: number) => {
    if (!replyContent.trim()) return;
    try {
      await createPost(workspace.id, replyContent.trim(), 'discussion', postId);
      setReplyContent('');
      setReplyingTo(null);
      fetchReplies(postId);
    } catch {
      // Silently fail — user can retry
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      handleSubmit();
    }
  };

  const loadMore = () => {
    if (hasMore && !loading && posts.length > 0) {
      fetchFeed(workspace.id, posts[posts.length - 1].id);
    }
  };

  const toggleReplies = (postId: number) => {
    if (repliesByPost[postId]) {
      return;
    }
    fetchReplies(postId);
  };

  return (
    <div className="flex flex-col h-full">
      {/* New post */}
      <div className="p-4 border-b border-[var(--border)]">
        <textarea
          value={newPost}
          onChange={(e) => setNewPost(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Share something with your team..."
          className="w-full bg-[var(--bg-tertiary)] text-[var(--text-primary)] rounded-lg p-3 resize-none border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none"
          rows={3}
        />
        <div className="flex justify-end mt-2">
          <button
            onClick={handleSubmit}
            disabled={!newPost.trim()}
            className="px-4 py-1.5 bg-[var(--accent)] text-white text-sm rounded hover:bg-[var(--accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            Post
          </button>
        </div>
      </div>

      {/* Feed */}
      <div className="flex-1 overflow-y-auto">
        {posts.length === 0 && !loading && (
          <div className="p-8 text-center text-[var(--text-secondary)]">
            <p className="text-lg mb-1">No posts yet</p>
            <p className="text-sm">Be the first to share something!</p>
          </div>
        )}
        {posts.map((post) => (
          <PostCard
            key={post.id}
            post={post}
            replies={repliesByPost[post.id]}
            isReplying={replyingTo === post.id}
            onToggleReaction={() => toggleReaction(post.id)}
            onDelete={() => deletePost(post.id)}
            onToggleReplies={() => toggleReplies(post.id)}
            onReply={() => setReplyingTo(replyingTo === post.id ? null : post.id)}
            onReplySubmit={() => handleReply(post.id)}
            replyContent={replyContent}
            onReplyChange={setReplyContent}
          />
        ))}
        {hasMore && (
          <div className="p-4 text-center">
            <button
              onClick={loadMore}
              disabled={loading}
              className="text-sm text-[var(--accent)] hover:underline disabled:opacity-50"
            >
              {loading ? 'Loading...' : 'Load more'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function PostCard({
  post,
  replies,
  isReplying,
  onToggleReaction,
  onDelete,
  onToggleReplies,
  onReply,
  onReplySubmit,
  replyContent,
  onReplyChange,
}: {
  post: Post;
  replies?: Post[];
  isReplying: boolean;
  onToggleReaction: () => void;
  onDelete: () => void;
  onToggleReplies: () => void;
  onReply: () => void;
  onReplySubmit: () => void;
  replyContent: string;
  onReplyChange: (v: string) => void;
}) {
  const timeAgo = formatTimeAgo(post.created_at);

  return (
    <div className="px-4 py-3 border-b border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors">
      <div className="flex items-start gap-3">
        <div className="w-8 h-8 rounded-full bg-[var(--accent)] flex items-center justify-center text-white text-sm font-medium shrink-0">
          {(post.author?.username || 'AI')[0].toUpperCase()}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm text-[var(--text-primary)]">
              {post.author?.display_name || post.author?.username || 'AI'}
            </span>
            {post.is_ai && (
              <span className="text-[10px] px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded-full">AI</span>
            )}
            <span className="text-xs text-[var(--text-muted)]">{timeAgo}</span>
          </div>
          <div className="mt-1 text-sm text-[var(--text-primary)] whitespace-pre-wrap break-words">
            {post.content}
          </div>

          {/* Actions */}
          <div className="flex items-center gap-4 mt-2">
            <button
              onClick={onToggleReaction}
              className="flex items-center gap-1 text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] transition-colors"
            >
              {post.like_count > 0 ? (
                <span className="text-red-400">&#9829;</span>
              ) : (
                <span>&#9825;</span>
              )}
              {post.like_count > 0 && <span>{post.like_count}</span>}
            </button>
            <button
              onClick={onToggleReplies}
              className="text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] transition-colors"
            >
              {post.reply_count > 0 ? `${post.reply_count} replies` : 'Reply'}
            </button>
            <button
              onClick={onDelete}
              className="text-xs text-[var(--text-muted)] hover:text-red-400 transition-colors ml-auto"
            >
              Delete
            </button>
          </div>

          {/* Replies */}
          {replies && replies.length > 0 && (
            <div className="mt-2 ml-4 border-l-2 border-[var(--border)] pl-3 space-y-2">
              {replies.map((r) => (
                <div key={r.id} className="text-sm">
                  <span className="font-medium text-[var(--text-primary)]">
                    {r.author?.display_name || r.author?.username || 'AI'}
                  </span>
                  <span className="text-xs text-[var(--text-muted)] ml-2">{formatTimeAgo(r.created_at)}</span>
                  <div className="text-[var(--text-primary)] mt-0.5 whitespace-pre-wrap">{r.content}</div>
                </div>
              ))}
            </div>
          )}

          {/* Reply input */}
          {isReplying && (
            <div className="mt-2 flex gap-2">
              <input
                value={replyContent}
                onChange={(e) => onReplyChange(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) onReplySubmit(); }}
                placeholder="Write a reply..."
                className="flex-1 bg-[var(--bg-tertiary)] text-[var(--text-primary)] text-sm rounded px-3 py-1.5 border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none"
              />
              <button
                onClick={onReplySubmit}
                disabled={!replyContent.trim()}
                className="px-3 py-1.5 bg-[var(--accent)] text-white text-sm rounded hover:bg-[var(--accent-hover)] disabled:opacity-50 transition-colors"
              >
                Reply
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function formatTimeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

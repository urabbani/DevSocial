import { create } from 'zustand';
import { api, type Post } from '../api/client';

interface FeedState {
  posts: Post[];
  repliesByPost: Record<number, Post[]>;
  loading: boolean;
  hasMore: boolean;

  fetchFeed: (workspaceId: number, before?: number) => Promise<void>;
  createPost: (workspaceId: number, content: string, postType?: string, parentPostId?: number, attachmentIds?: number[]) => Promise<Post>;
  toggleReaction: (postId: number) => Promise<number>;
  fetchReplies: (postId: number) => Promise<void>;
  deletePost: (postId: number) => Promise<void>;
  addPost: (post: Post) => void;
}

export const useFeedStore = create<FeedState>((set, get) => ({
  posts: [],
  repliesByPost: {},
  loading: false,
  hasMore: true,

  fetchFeed: async (workspaceId: number, before?: number) => {
    set({ loading: true });
    try {
      const oldestId = before ?? (get().posts.length > 0 ? get().posts[0].id : undefined);
      const posts = await api.getFeed(workspaceId, oldestId);
      const hasMore = posts.length >= 30;
      set((s) => ({
        posts: before !== undefined ? [...posts, ...s.posts] : posts,
        hasMore,
        loading: false,
      }));
    } catch {
      set({ loading: false });
    }
  },

  createPost: async (workspaceId: number, content: string, postType?: string, parentPostId?: number, attachmentIds?: number[]) => {
    const post = await api.createPost(workspaceId, {
      content,
      post_type: postType || 'discussion',
      parent_post_id: parentPostId,
      attachment_ids: attachmentIds,
    });
    if (!parentPostId) {
      set((s) => ({ posts: [post, ...s.posts] }));
    } else {
      set((s) => ({
        repliesByPost: {
          ...s.repliesByPost,
          [parentPostId]: [...(s.repliesByPost[parentPostId] || []), post],
        },
      }));
    }
    return post;
  },

  toggleReaction: async (postId: number) => {
    const result = await api.togglePostReaction(postId);
    set((s) => ({
      posts: s.posts.map((p) => p.id === postId ? { ...p, like_count: result.like_count } : p),
    }));
    return result.like_count;
  },

  fetchReplies: async (postId: number) => {
    try {
      const replies = await api.getPostReplies(postId);
      set((s) => ({
        repliesByPost: { ...s.repliesByPost, [postId]: replies },
      }));
    } catch {
      // ignore
    }
  },

  deletePost: async (postId: number) => {
    await api.deletePost(postId);
    set((s) => ({
      posts: s.posts.filter((p) => p.id !== postId),
    }));
  },

  addPost: (post: Post) => {
    set((s) => ({ posts: [post, ...s.posts] }));
  },
}));

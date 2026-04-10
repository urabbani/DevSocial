import { useEffect } from 'react';
import { useAuthStore } from '../../stores/auth';
import { useWorkspaceStore } from '../../stores/workspace';
import { WorkspaceSidebar } from '../layout/WorkspaceSidebar';
import { ChannelList } from '../channels/ChannelList';
import { MessageView } from '../messages/MessageView';
import { useWebSocket } from '../../hooks/useWebSocket';

export function AppShell() {
  const { user, loading, fetchUser } = useAuthStore();
  const { fetchWorkspaces } = useWorkspaceStore();
  useWebSocket();

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  useEffect(() => {
    if (user) fetchWorkspaces();
  }, [user, fetchWorkspaces]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full bg-[var(--bg-primary)]">
        <div className="text-[var(--text-secondary)]">Loading...</div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="flex items-center justify-center h-full bg-[var(--bg-primary)]">
        <div className="text-center">
          <h1 className="text-3xl font-bold mb-2">DevSocial</h1>
          <p className="text-[var(--text-secondary)] mb-6">Where developers and AI collaborate</p>
          <a
            href="/auth/github"
            className="inline-block px-6 py-2 bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors"
          >
            Sign in with GitHub
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full">
      <WorkspaceSidebar />
      <ChannelList />
      <MessageView />
    </div>
  );
}

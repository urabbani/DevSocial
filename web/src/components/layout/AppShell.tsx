import { useEffect, useState } from 'react';
import { useAuthStore } from '../../stores/auth';
import { useWorkspaceStore } from '../../stores/workspace';
import { WorkspaceSidebar } from '../layout/WorkspaceSidebar';
import { ChannelList } from '../channels/ChannelList';
import { MessageView } from '../messages/MessageView';
import { AdminPanel } from '../admin/AdminPanel';
import { FileBrowser } from '../files/FileBrowser';
import { FeedView } from '../feed/FeedView';
import { TaskBoard } from '../tasks/TaskBoard';
import { SearchView } from '../search/SearchView';
import { NotificationBell } from "../notifications/NotificationBell";
import { useWebSocket } from '../../hooks/useWebSocket';

type View = 'chat' | 'feed' | 'files' | 'tasks' | 'search' | 'admin';

const TABS: { key: View; label: string }[] = [
  { key: 'chat', label: 'Chat' },
  { key: 'feed', label: 'Feed' },
  { key: 'tasks', label: 'Tasks' },
  { key: 'files', label: 'Files' },
  { key: 'search', label: 'Search' },
  { key: 'admin', label: 'Admin' },
];

export function AppShell() {
  const { user, loading, fetchUser } = useAuthStore();
  const { fetchWorkspaces, activeWorkspace } = useWorkspaceStore();
  const [view, setView] = useState<View>('chat');
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

      {/* View switcher tabs */}
      <div className="flex flex-col flex-1 min-w-0">
        <div className="flex items-center gap-1 px-4 py-1 border-b border-[var(--border)] bg-[var(--bg-secondary)] shrink-0">
          {TABS.map((tab) => (
            <TabButton key={tab.key} active={view === tab.key} onClick={() => setView(tab.key)}>
              {tab.label}
            </TabButton>
          ))}
          <div className="flex-1" />
          <NotificationBell />
          <span className="text-xs text-[var(--text-muted)]">{activeWorkspace?.name}</span>
        </div>
          <div className="flex-1" />
          <span className="text-xs text-[var(--text-muted)]">{activeWorkspace?.name}</span>
        </div>

        {view === 'chat' && <MessageView />}
        {view === 'feed' && <FeedView />}
        {view === 'tasks' && <TaskBoard />}
        {view === 'files' && <FileBrowser />}
        {view === 'search' && <SearchView />}
        {view === 'admin' && <AdminPanel onClose={() => setView('chat')} />}
      </div>
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
        active
          ? 'bg-[var(--bg-tertiary)] text-white'
          : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
      }`}
    >
      {children}
    </button>
  );
}

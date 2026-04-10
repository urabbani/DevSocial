import { useState } from 'react';
import { useAuthStore } from '../../stores/auth';
import { useWorkspaceStore } from '../../stores/workspace';
import { CreateWorkspace } from '../workspace/CreateWorkspace';

export function WorkspaceSidebar() {
  const { user, logout } = useAuthStore();
  const { workspaces, activeWorkspace, setActiveWorkspace } = useWorkspaceStore();
  const [showCreate, setShowCreate] = useState(false);

  if (!user) return null;

  return (
    <div className="w-16 flex flex-col items-center py-3 bg-[var(--bg-secondary)] border-r border-[var(--border)] shrink-0">
      {/* Workspace list */}
      <div className="flex flex-col items-center gap-2 flex-1 overflow-y-auto">
        {workspaces.map((ws) => (
          <button
            key={ws.id}
            onClick={() => setActiveWorkspace(ws)}
            className={`w-10 h-10 rounded flex items-center justify-center font-bold text-sm transition-colors hover:bg-[var(--bg-hover)] ${
              activeWorkspace?.id === ws.id ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-tertiary)] text-[var(--text-primary)]'
            }`}
            title={ws.name}
          >
            {ws.name.charAt(0).toUpperCase()}
          </button>
        ))}

        <button
          onClick={() => setShowCreate(true)}
          className="w-10 h-10 rounded flex items-center justify-center text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors text-lg"
          title="Create Workspace"
        >
          +
        </button>
      </div>

      {/* User avatar at bottom */}
      <div className="relative group">
        <img
          src={user.avatar_url}
          alt={user.username}
          className="w-8 h-8 rounded-full"
        />
        <div className="absolute bottom-0 right-0 w-2.5 h-2.5 bg-[var(--green)] rounded-full border-2 border-[var(--bg-secondary)]" />

        {/* Tooltip menu */}
        <div className="absolute bottom-full mb-2 left-1/2 -translate-x-1/2 hidden group-hover:block bg-[var(--bg-tertiary)] rounded shadow-lg p-2 min-w-48 z-50">
          <div className="px-3 py-2 text-sm font-medium truncate">{user.display_name || user.username}</div>
          <div className="px-3 py-1 text-xs text-[var(--text-muted)] truncate">@{user.username}</div>
          <hr className="border-[var(--border)] my-1" />
          <button
            onClick={logout}
            className="w-full text-left px-3 py-1.5 text-sm text-[var(--red)] hover:bg-[var(--bg-hover)] rounded"
          >
            Sign Out
          </button>
        </div>
      </div>

      {showCreate && <CreateWorkspace onClose={() => setShowCreate(false)} />}
    </div>
  );
}

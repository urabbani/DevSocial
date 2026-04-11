import { useState } from 'react';
import { useWorkspaceStore } from '../../stores/workspace';

export function ChannelList() {
  const { activeWorkspace, channels, activeChannel, setActiveChannel, createChannel } = useWorkspaceStore();
  const [showCreate, setShowCreate] = useState(false);
  const [newChannelName, setNewChannelName] = useState('');

  if (!activeWorkspace) {
    return (
      <div className="w-56 bg-[var(--bg-secondary)] border-r border-[var(--border)] flex items-center justify-center">
        <p className="text-sm text-[var(--text-muted)]">Select a workspace</p>
      </div>
    );
  }

  const handleCreate = async () => {
    if (!newChannelName.trim()) return;
    try {
      await createChannel(newChannelName.trim());
      setNewChannelName('');
      setShowCreate(false);
    } catch (_err) {
      // Create failed, keep the dialog open
    }
  };

  const textChannels = channels.filter((c) => c.type === 'text');
  const aiChannels = channels.filter((c) => c.type === 'ai');

  return (
    <div className="w-56 bg-[var(--bg-secondary)] border-r border-[var(--border)] flex flex-col shrink-0">
      {/* Workspace name */}
      <div className="h-12 flex items-center px-4 font-semibold text-sm border-b border-[var(--border)] shrink-0">
        {activeWorkspace.name}
      </div>

      {/* Channel list */}
      <div className="flex-1 overflow-y-auto py-2">
        {/* Text channels */}
        <div className="px-2 mb-1">
          <span className="text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider px-2">
            Text Channels
          </span>
        </div>
        {textChannels.map((ch) => (
          <button
            key={ch.id}
            onClick={() => setActiveChannel(ch)}
            className={`w-full text-left px-2 py-1.5 mx-1 rounded text-sm flex items-center gap-1.5 transition-colors ${
              activeChannel?.id === ch.id
                ? 'bg-[var(--bg-tertiary)] text-white'
                : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
            }`}
          >
            <span className="text-[var(--text-muted)]">#</span>
            <span className="truncate flex-1">{ch.name}</span>
            {ch.unread_count > 0 && (
              <span className="bg-[var(--red)] text-white text-xs rounded-full px-1.5 min-w-5 text-center">
                {ch.unread_count > 99 ? '99+' : ch.unread_count}
              </span>
            )}
          </button>
        ))}

        {/* AI channels */}
        {aiChannels.length > 0 && (
          <>
            <div className="px-2 mt-4 mb-1">
              <span className="text-[10px] font-bold text-[var(--text-muted)] uppercase tracking-wider px-2">
                AI Assistants
              </span>
            </div>
            {aiChannels.map((ch) => (
              <button
                key={ch.id}
                onClick={() => setActiveChannel(ch)}
                className={`w-full text-left px-2 py-1.5 mx-1 rounded text-sm flex items-center gap-1.5 transition-colors ${
                  activeChannel?.id === ch.id
                    ? 'bg-[var(--bg-tertiary)] text-white'
                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
                }`}
              >
                <span className="text-[var(--accent)]">AI</span>
                <span className="truncate flex-1">{ch.name}</span>
                {ch.unread_count > 0 && (
                  <span className="bg-[var(--red)] text-white text-xs rounded-full px-1.5 min-w-5 text-center">
                    {ch.unread_count}
                  </span>
                )}
              </button>
            ))}
          </>
        )}
      </div>

      {/* Create channel */}
      <div className="p-2 border-t border-[var(--border)] shrink-0">
        {showCreate ? (
          <div className="flex gap-1">
            <input
              type="text"
              value={newChannelName}
              onChange={(e) => setNewChannelName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              placeholder="channel-name"
              className="flex-1 px-2 py-1 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
              autoFocus
            />
            <button onClick={handleCreate} className="text-[var(--accent)] hover:text-white text-sm px-1">
              +
            </button>
            <button onClick={() => { setShowCreate(false); setNewChannelName(''); }} className="text-[var(--text-muted)] text-sm px-1">
              x
            </button>
          </div>
        ) : (
          <button
            onClick={() => setShowCreate(true)}
            className="w-full text-left px-2 py-1 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] rounded hover:bg-[var(--bg-hover)] transition-colors"
          >
            + Add Channel
          </button>
        )}
      </div>
    </div>
  );
}

import { useState } from 'react';
import { useWorkspaceStore } from '../../stores/workspace';

interface Props {
  onClose: () => void;
}

export function CreateWorkspace({ onClose }: Props) {
  const { createWorkspace } = useWorkspaceStore();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    await createWorkspace(name.trim(), description.trim());
    onClose();
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-[var(--bg-secondary)] rounded-lg shadow-xl w-96 p-6">
        <h2 className="text-lg font-semibold mb-4">Create Workspace</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-3">
            <label className="block text-sm text-[var(--text-secondary)] mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Project"
              className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-[var(--text-primary)] text-sm focus:outline-none focus:border-[var(--accent)]"
              autoFocus
            />
          </div>
          <div className="mb-4">
            <label className="block text-sm text-[var(--text-secondary)] mb-1">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-[var(--text-primary)] text-sm focus:outline-none focus:border-[var(--accent)]"
            />
          </div>
          <div className="flex gap-2 justify-end">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!name.trim()}
              className="px-4 py-2 bg-[var(--accent)] text-white rounded text-sm hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50"
            >
              Create
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

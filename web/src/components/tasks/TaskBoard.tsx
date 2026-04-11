import { useEffect, useState } from 'react';
import { useTaskStore } from '../../stores/tasks';
import { useWorkspaceStore } from '../../stores/workspace';
import type { Task } from '../../api/client';

const STATUS_OPTIONS = ['todo', 'in_progress', 'done'];
const PRIORITY_OPTIONS = ['low', 'medium', 'high', 'urgent'];

const STATUS_LABELS: Record<string, string> = {
  todo: 'To Do',
  in_progress: 'In Progress',
  done: 'Done',
};

const PRIORITY_COLORS: Record<string, string> = {
  low: 'text-[var(--text-muted)]',
  medium: 'text-yellow-500',
  high: 'text-orange-500',
  urgent: 'text-red-500',
};

export function TaskBoard() {
  const workspace = useWorkspaceStore((s) => s.activeWorkspace);
  const { tasks, loading, statusFilter, setFilter, fetchTasks, createTask, updateTask, deleteTask } = useTaskStore();
  const [showForm, setShowForm] = useState(false);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState('medium');

  useEffect(() => {
    if (workspace) {
      fetchTasks(workspace.id);
    }
  }, [workspace?.id, statusFilter]);

  if (!workspace) {
    return <div className="p-6 text-[var(--text-secondary)]">Select a workspace</div>;
  }

  const handleCreate = async () => {
    if (!title.trim()) return;
    try {
      await createTask(workspace.id, { title: title.trim(), description: description.trim() || undefined, priority });
      setTitle('');
      setDescription('');
      setPriority('medium');
      setShowForm(false);
    } catch {
      // Silently fail — user can retry
    }
  };

  const handleStatusChange = (taskId: number, status: string) => {
    updateTask(taskId, { status });
  };

  const filteredTasks = statusFilter ? tasks.filter((t) => t.status === statusFilter) : tasks;

  // Group by status for kanban (use filteredTasks when filter active)
  const grouped = STATUS_OPTIONS.reduce((acc, status) => {
    acc[status] = (statusFilter ? filteredTasks : tasks).filter((t) => t.status === status);
    return acc;
  }, {} as Record<string, typeof tasks>);

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="p-4 border-b border-[var(--border)] flex items-center gap-3">
        <div className="flex items-center gap-1">
          <button
            onClick={() => setFilter('')}
            className={`px-2.5 py-1 text-xs rounded transition-colors ${!statusFilter ? 'bg-[var(--bg-tertiary)] text-white' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}`}
          >
            All ({tasks.length})
          </button>
          {STATUS_OPTIONS.map((s) => (
            <button
              key={s}
              onClick={() => setFilter(s === statusFilter ? '' : s)}
              className={`px-2.5 py-1 text-xs rounded transition-colors ${statusFilter === s ? 'bg-[var(--bg-tertiary)] text-white' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}`}
            >
              {STATUS_LABELS[s]} ({grouped[s].length})
            </button>
          ))}
        </div>
        <div className="flex-1" />
        <button
          onClick={() => setShowForm(!showForm)}
          className="px-3 py-1.5 bg-[var(--accent)] text-white text-sm rounded hover:bg-[var(--accent-hover)] transition-colors"
        >
          {showForm ? 'Cancel' : '+ New Task'}
        </button>
      </div>

      {/* Create form */}
      {showForm && (
        <div className="p-4 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Task title..."
            className="w-full bg-[var(--bg-tertiary)] text-[var(--text-primary)] rounded px-3 py-2 text-sm border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none mb-2"
            autoFocus
          />
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Description (optional)"
            className="w-full bg-[var(--bg-tertiary)] text-[var(--text-primary)] rounded px-3 py-2 text-sm border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none mb-2 resize-none"
            rows={2}
          />
          <div className="flex items-center gap-3">
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
              className="bg-[var(--bg-tertiary)] text-[var(--text-primary)] rounded px-2 py-1 text-sm border border-[var(--border)]"
            >
              {PRIORITY_OPTIONS.map((p) => (
                <option key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</option>
              ))}
            </select>
            <button
              onClick={handleCreate}
              disabled={!title.trim()}
              className="px-4 py-1.5 bg-[var(--accent)] text-white text-sm rounded hover:bg-[var(--accent-hover)] disabled:opacity-50 transition-colors"
            >
              Create
            </button>
          </div>
        </div>
      )}

      {/* Task list / Kanban */}
      <div className="flex-1 overflow-y-auto p-4">
        {loading && <div className="text-sm text-[var(--text-secondary)]">Loading...</div>}
        {!loading && filteredTasks.length === 0 && (
          <div className="text-center text-[var(--text-secondary)] py-8">
            <p>No tasks found</p>
          </div>
        )}

        {!statusFilter ? (
          // Kanban view
          <div className="grid grid-cols-3 gap-4">
            {STATUS_OPTIONS.map((status) => (
              <div key={status} className="bg-[var(--bg-secondary)] rounded-lg p-3">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-medium text-[var(--text-primary)]">{STATUS_LABELS[status]}</h3>
                  <span className="text-xs text-[var(--text-muted)]">{grouped[status].length}</span>
                </div>
                <div className="space-y-2">
                  {grouped[status].map((task) => (
                    <TaskCard key={task.id} task={task} onStatusChange={handleStatusChange} onDelete={deleteTask} />
                  ))}
                </div>
              </div>
            ))}
          </div>
        ) : (
          // List view
          <div className="space-y-2">
            {filteredTasks.map((task) => (
              <TaskCard key={task.id} task={task} onStatusChange={handleStatusChange} onDelete={deleteTask} showStatus />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function TaskCard({ task, onStatusChange, onDelete, showStatus }: {
  task: Task;
  onStatusChange: (id: number, status: string) => void;
  onDelete: (id: number) => void;
  showStatus?: boolean;
}) {
  const nextStatus = task.status === 'todo' ? 'in_progress' : task.status === 'in_progress' ? 'done' : 'todo';

  return (
    <div className="bg-[var(--bg-tertiary)] rounded-lg p-3 border border-[var(--border)] hover:border-[var(--accent)]/30 transition-colors">
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <button
              onClick={() => onStatusChange(task.id, nextStatus)}
              className="w-4 h-4 rounded border-2 border-[var(--border)] hover:border-[var(--accent)] shrink-0 flex items-center justify-center transition-colors"
              title={`Move to ${STATUS_LABELS[nextStatus]}`}
            >
              {task.status === 'done' && <span className="text-[var(--accent)] text-xs">&#10003;</span>}
            </button>
            <span className={`text-sm font-medium ${task.status === 'done' ? 'line-through text-[var(--text-muted)]' : 'text-[var(--text-primary)]'}`}>
              {task.title}
            </span>
          </div>
          {task.description && (
            <p className="text-xs text-[var(--text-secondary)] mt-1 ml-6 line-clamp-2">{task.description}</p>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {showStatus && (
            <select
              value={task.status}
              onChange={(e) => onStatusChange(task.id, e.target.value)}
              className="bg-transparent text-xs text-[var(--text-secondary)] border-none cursor-pointer"
            >
              {STATUS_OPTIONS.map((s) => (
                <option key={s} value={s}>{STATUS_LABELS[s]}</option>
              ))}
            </select>
          )}
          <span className={`text-xs font-medium ${PRIORITY_COLORS[task.priority] || 'text-[var(--text-secondary)]'}`}>
            {task.priority}
          </span>
          <button
            onClick={() => onDelete(task.id)}
            className="text-[var(--text-muted)] hover:text-red-400 text-xs transition-colors"
          >
            &times;
          </button>
        </div>
      </div>
      {task.creator_name && (
        <div className="text-[10px] text-[var(--text-muted)] mt-1 ml-6">
          by {task.creator_name} &middot; {new Date(task.created_at).toLocaleDateString()}
        </div>
      )}
    </div>
  );
}

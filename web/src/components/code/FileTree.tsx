import { useState } from 'react';
import { useDocumentStore } from '../../stores/documents';
import { useAuthStore } from '../../stores/auth';
import type { CodeDocument } from '../../api/client';

interface FileTreeProps {
  onDocumentSelect: (doc: CodeDocument) => void;
  onNewDocument: () => void;
  activeDocumentId: number | null;
}

const LANGUAGE_ICONS: Record<string, string> = {
  python: '🐍',
  javascript: '📜',
  typescript: '📘',
  go: '🐹',
  rust: '🦀',
  java: '☕',
  c: '🔧',
  cpp: '⚙️',
  csharp: '💠',
  php: '🐘',
  ruby: '💎',
  swift: '🍎',
  kotlin: '🎯',
  bash: '💻',
  html: '🌐',
  css: '🎨',
  scss: '🎨',
  json: '📋',
  xml: '📋',
  yaml: '📋',
  markdown: '📝',
  sql: '🗄️',
  dockerfile: '🐳',
  text: '📄',
};

export function FileTree({
  onDocumentSelect,
  onNewDocument,
  activeDocumentId,
}: FileTreeProps) {
  const { documents, viewers, deleteDocument } = useDocumentStore();
  const { user } = useAuthStore();
  const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null);

  const handleDelete = async (id: number) => {
    try {
      await deleteDocument(id);
      setDeleteConfirm(null);
    } catch {
      // Error handled by store
    }
  };

  const groupedDocs = documents.reduce((acc, doc) => {
    const lang = doc.language || 'text';
    if (!acc[lang]) acc[lang] = [];
    acc[lang].push(doc);
    return acc;
  }, {} as Record<string, CodeDocument[]>);

  return (
    <div className="h-full flex flex-col bg-[var(--bg-secondary)] border-r border-[var(--border)]">
      {/* Header */}
      <div className="p-3 border-b border-[var(--border)] flex items-center justify-between">
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Files</h3>
        <button
          onClick={onNewDocument}
          className="text-xs px-2 py-1 bg-[var(--accent)] text-white rounded hover:opacity-90 transition-opacity"
          title="New file"
        >
          + New
        </button>
      </div>

      {/* File List */}
      <div className="flex-1 overflow-y-auto p-2">
        {documents.length === 0 ? (
          <div className="text-center py-8 text-sm text-[var(--text-muted)]">
            <p>No files yet</p>
            <p className="text-xs mt-1">Create your first file to get started</p>
          </div>
        ) : (
          Object.entries(groupedDocs).map(([lang, docs]) => (
            <div key={lang} className="mb-4">
              <div className="text-xs text-[var(--text-muted)] uppercase tracking-wider px-2 mb-1 flex items-center gap-1">
                <span>{LANGUAGE_ICONS[lang] || '📄'}</span>
                <span>{lang}</span>
                <span className="ml-auto">{docs.length}</span>
              </div>
              {docs.map((doc) => {
                const docViewers = viewers[doc.id] || [];
                const isActive = doc.id === activeDocumentId;
                const isDeleting = deleteConfirm === doc.id;

                return (
                  <div
                    key={doc.id}
                    className={`group flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer transition-colors ${
                      isActive
                        ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)]'
                        : 'hover:bg-[var(--bg-hover)] text-[var(--text-secondary)]'
                    }`}
                    onClick={() => !isDeleting && onDocumentSelect(doc)}
                  >
                    <span className="text-sm">{LANGUAGE_ICONS[doc.language] || '📄'}</span>
                    <span className="flex-1 text-sm truncate">{doc.filename}</span>

                    {/* Viewer count badge */}
                    {docViewers.length > 0 && (
                      <span className="text-xs px-1.5 py-0.5 bg-[var(--bg-primary)] rounded-full text-[var(--text-muted)]">
                        {docViewers.length}
                      </span>
                    )}

                    {/* Delete button (only show for owner) */}
                    {doc.created_by === user?.id && !isDeleting && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setDeleteConfirm(doc.id);
                        }}
                        className="opacity-0 group-hover:opacity-100 text-xs text-[var(--text-muted)] hover:text-red-500 transition-opacity"
                        title="Delete file"
                      >
                        ×
                      </button>
                    )}

                    {/* Delete confirmation */}
                    {isDeleting && (
                      <div className="flex items-center gap-1">
                        <button
                                                          onClick={(e) => {
                            e.stopPropagation();
                            handleDelete(doc.id);
                          }}
                          className="text-xs text-red-500 hover:text-red-600"
                          title="Confirm delete"
                        >
                          ✓
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            setDeleteConfirm(null);
                          }}
                          className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                          title="Cancel"
                        >
                          ✕
                        </button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ))
        )}
      </div>
    </div>
  );
}

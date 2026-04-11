import { useEffect, useState, useCallback, useRef } from 'react';
import { useWorkspaceStore } from '../../stores/workspace';
import { useDocumentStore } from '../../stores/documents';
import { useAuthStore } from '../../stores/auth';
import { CodeEditor } from './CodeEditor';
import { FileTree } from './FileTree';
import { OutputPanel } from './OutputPanel';
import type { CodeDocument } from '../../api/client';

export function CodeEditorView() {
  const { activeWorkspace } = useWorkspaceStore();
  const { user } = useAuthStore();
  const {
    documents,
    activeDocumentId,
    viewers,
    editors,
    fetchDocuments,
    createDocument,
    setActiveDocument,
    clearViewers,
  } = useDocumentStore();

  const [content, setContent] = useState('');
  const [showNewFileModal, setShowNewFileModal] = useState(false);
  const [newFileForm, setNewFileForm] = useState({ filename: '', title: '', language: '', content: '' });
  const autoSaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const contentRef = useRef(content);
  contentRef.current = content;

  // Fetch documents on workspace change
  useEffect(() => {
    if (activeWorkspace?.id) {
      fetchDocuments(activeWorkspace.id);
    }
  }, [activeWorkspace?.id, fetchDocuments]);

  // Clear auto-save timer when switching documents
  useEffect(() => {
    if (autoSaveTimer.current) {
      clearTimeout(autoSaveTimer.current);
      autoSaveTimer.current = null;
    }
  }, [activeDocumentId]);

  // Load document content when active document changes
  useEffect(() => {
    const activeDoc = documents.find((d) => d.id === activeDocumentId);
    if (activeDoc) {
      setContent(activeDoc.content);
    } else {
      setContent('');
    }
  }, [activeDocumentId, documents]);

  // Cleanup viewers on unmount
  useEffect(() => {
    return () => {
      if (activeDocumentId) {
        clearViewers(activeDocumentId);
      }
    };
  }, [activeDocumentId, clearViewers]);

  // Send doc_open/doc_close via WebSocket when switching documents
  useEffect(() => {
    const sendWSMessage = (window as any).sendWSMessage;
    if (activeDocumentId && user && sendWSMessage) {
      sendWSMessage({
        type: 'doc_open',
        document_id: activeDocumentId,
        content: user.username,
      });
    }

    return () => {
      if (activeDocumentId && sendWSMessage) {
        sendWSMessage({
          type: 'doc_close',
          document_id: activeDocumentId,
        });
      }
    };
  }, [activeDocumentId, user]);

  // Auto-save with debounce
  const handleContentChange = useCallback((newContent: string) => {
    setContent(newContent);

    if (autoSaveTimer.current) {
      clearTimeout(autoSaveTimer.current);
    }

    autoSaveTimer.current = setTimeout(async () => {
      if (!activeDocumentId) return;
      const doc = useDocumentStore.getState().documents.find((d) => d.id === activeDocumentId);
      if (doc) {
        try {
          await useDocumentStore.getState().updateDocument(activeDocumentId, {
            content: newContent,
            version: doc.version,
          });
        } catch {
          // Version conflict - will resolve on next edit
        }
      }
    }, 2000);
  }, [activeDocumentId]);

  // Save on blur
  const handleBlur = useCallback(async () => {
    if (autoSaveTimer.current) {
      clearTimeout(autoSaveTimer.current);
    }

    if (activeDocumentId) {
      const doc = useDocumentStore.getState().documents.find((d) => d.id === activeDocumentId);
      if (doc && contentRef.current !== doc.content) {
        try {
          await useDocumentStore.getState().updateDocument(activeDocumentId, {
            content: contentRef.current,
            version: doc.version,
          });
        } catch {
          // Version conflict
        }
      }
    }
  }, [activeDocumentId]);

  const handleDocumentSelect = useCallback((doc: CodeDocument) => {
    setActiveDocument(doc.id);
  }, [setActiveDocument]);

  const handleCreateDocument = useCallback(async () => {
    if (!activeWorkspace?.id) return;

    try {
      await createDocument(activeWorkspace.id, {
        filename: newFileForm.filename || 'untitled.txt',
        title: newFileForm.title || 'Untitled',
        language: newFileForm.language || 'text',
        content: newFileForm.content || '',
      });
      setShowNewFileModal(false);
      setNewFileForm({ filename: '', title: '', language: '', content: '' });
    } catch {
      // Error handled by store
    }
  }, [activeWorkspace?.id, createDocument, newFileForm]);

  const activeDoc = documents.find((d) => d.id === activeDocumentId);
  const docViewers = activeDocumentId ? (viewers[activeDocumentId] || []) : [];

  if (!activeWorkspace) {
    return (
      <div className="h-full flex items-center justify-center text-[var(--text-muted)]">
        <p>Select a workspace to view code files</p>
      </div>
    );
  }

  return (
    <div className="h-full flex">
      {/* File Tree Sidebar */}
      <div className="w-64 flex-shrink-0">
        <FileTree
          onDocumentSelect={handleDocumentSelect}
          onNewDocument={() => setShowNewFileModal(true)}
          activeDocumentId={activeDocumentId}
        />
      </div>

      {/* Editor Area */}
      <div className="flex-1 flex flex-col">
        {activeDoc ? (
          <>
            {/* Editor Header */}
            <div className="h-12 border-b border-[var(--border)] flex items-center justify-between px-4 bg-[var(--bg-secondary)]">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-[var(--text-primary)]">{activeDoc.filename}</span>
                <span className="text-xs px-2 py-0.5 bg-[var(--bg-tertiary)] text-[var(--text-muted)] rounded">
                  {activeDoc.language}
                </span>
                {editors[activeDocumentId!] && (
                  <span className="text-xs px-2 py-0.5 bg-[var(--accent)]/20 text-[var(--accent)] rounded animate-pulse">
                    {editors[activeDocumentId!]} is editing...
                  </span>
                )}
                {docViewers.length > 0 && !editors[activeDocumentId!] && (
                  <span className="text-xs text-[var(--text-muted)]">
                    {docViewers.map((v) => v.username).join(', ')} viewing
                  </span>
                )}
              </div>
              <div className="flex items-center gap-3">
                <div className="text-xs text-[var(--text-muted)]">
                  Auto-saving...
                </div>
                <div className="text-xs text-[var(--text-muted)] flex items-center gap-1">
                  <kbd className="px-1.5 py-0.5 bg-[var(--bg-tertiary)] rounded text-[10px]">Ctrl+S</kbd>
                  <span>Save</span>
                  <kbd className="px-1.5 py-0.5 bg-[var(--bg-tertiary)] rounded text-[10px] ml-2">Ctrl+Enter</kbd>
                  <span>Run</span>
                </div>
              </div>
            </div>

            {/* Monaco Editor + Output */}
            <div className="flex-1 flex flex-col min-h-0">
              <div className="flex-1 min-h-0" onBlur={handleBlur}>
                <CodeEditor
                  value={content}
                  language={activeDoc.language}
                  onChange={handleContentChange}
                />
              </div>
              <OutputPanel documentId={activeDocumentId} language={activeDoc.language} />
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-[var(--text-muted)]">
            <div className="text-center">
              <p className="text-lg mb-2">No file selected</p>
              <p className="text-sm mb-4">Select a file from the sidebar or create a new one</p>
              <button
                onClick={() => setShowNewFileModal(true)}
                className="px-4 py-2 bg-[var(--accent)] text-white rounded hover:opacity-90 transition-opacity"
              >
                Create New File
              </button>
            </div>
          </div>
        )}
      </div>

      {/* New File Modal */}
      {showNewFileModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-[var(--bg-secondary)] rounded-lg p-6 w-96 border border-[var(--border)]">
            <h3 className="text-lg font-medium text-[var(--text-primary)] mb-4">Create New File</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm text-[var(--text-secondary)] mb-1">Filename</label>
                <input
                  type="text"
                  value={newFileForm.filename}
                  onChange={(e) => setNewFileForm({ ...newFileForm, filename: e.target.value })}
                  placeholder="main.py"
                  className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
                />
              </div>
              <div>
                <label className="block text-sm text-[var(--text-secondary)] mb-1">Title</label>
                <input
                  type="text"
                  value={newFileForm.title}
                  onChange={(e) => setNewFileForm({ ...newFileForm, title: e.target.value })}
                  placeholder="Main Script"
                  className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
                />
              </div>
              <div>
                <label className="block text-sm text-[var(--text-secondary)] mb-1">Language</label>
                <select
                  value={newFileForm.language}
                  onChange={(e) => setNewFileForm({ ...newFileForm, language: e.target.value })}
                  className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
                >
                  <option value="text">Plain Text</option>
                  <option value="python">Python</option>
                  <option value="javascript">JavaScript</option>
                  <option value="typescript">TypeScript</option>
                  <option value="go">Go</option>
                  <option value="rust">Rust</option>
                  <option value="bash">Bash</option>
                  <option value="html">HTML</option>
                  <option value="css">CSS</option>
                  <option value="json">JSON</option>
                  <option value="markdown">Markdown</option>
                </select>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button
                onClick={() => setShowNewFileModal(false)}
                className="px-4 py-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateDocument}
                disabled={!newFileForm.filename}
                className="px-4 py-2 bg-[var(--accent)] text-white rounded hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                Create
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { useState, useEffect, useRef } from 'react';
import { api, type FileInfo } from '../../api/client';
import { useWorkspaceStore } from '../../stores/workspace';

export function FileBrowser() {
  const { activeWorkspace } = useWorkspaceStore();
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const workspaceId = activeWorkspace?.id ?? 0;

  useEffect(() => {
    if (workspaceId) loadFiles();
  }, [workspaceId]);

  async function loadFiles() {
    setLoading(true);
    try {
      const f = await api.listFiles(workspaceId);
      setFiles(f);
    } catch {
      setFiles([]);
    }
    setLoading(false);
  }

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const fileList = e.target.files;
    if (!fileList?.length) return;

    setUploading(true);
    setError('');
    for (const file of Array.from(fileList)) {
      try {
        const uploaded = await api.uploadFile(workspaceId, file);
        setFiles((prev) => [uploaded, ...prev]);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Upload failed');
      }
    }
    setUploading(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }

  async function handleDelete(id: number) {
    try {
      await api.deleteFile(id);
      setFiles((prev) => prev.filter((f) => f.id !== id));
    } catch {
      // Ignore
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  if (!activeWorkspace) {
    return (
      <div className="flex items-center justify-center h-full">
        <p className="text-sm text-[var(--text-muted)]">Select a workspace</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)]">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border)] shrink-0">
        <h3 className="text-sm font-semibold">Files</h3>
        <div>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            onChange={handleUpload}
            className="hidden"
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="text-xs px-3 py-1.5 bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] disabled:opacity-50"
          >
            {uploading ? 'Uploading...' : '+ Upload'}
          </button>
        </div>
      </div>

      {error && (
        <div className="px-4 py-2 text-xs text-[var(--red)]">{error}</div>
      )}

      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="p-4 text-sm text-[var(--text-muted)]">Loading files...</div>
        ) : files.length === 0 ? (
          <div className="p-4 text-sm text-[var(--text-muted)]">
            No files yet. Upload documents, data, or code to get started.
          </div>
        ) : (
          <div className="divide-y divide-[var(--border)]">
            {files.map((file) => (
              <div key={file.id} className="flex items-center gap-3 px-4 py-3 hover:bg-[var(--bg-secondary)] group">
                <div className="flex-1 min-w-0">
                  <a
                    href={api.getFileUrl(file.id)}
                    target="_blank"
                    rel="noreferrer"
                    className="text-sm text-[var(--accent)] hover:underline truncate block"
                  >
                    {file.filename}
                  </a>
                  <div className="text-xs text-[var(--text-muted)] flex gap-3 mt-0.5">
                    <span>{formatSize(file.size_bytes)}</span>
                    <span>{file.uploader_name}</span>
                    <span>{new Date(file.created_at).toLocaleDateString()}</span>
                  </div>
                </div>
                <button
                  onClick={() => handleDelete(file.id)}
                  className="text-xs text-[var(--text-muted)] hover:text-[var(--red)] opacity-0 group-hover:opacity-100 transition-opacity"
                >
                  delete
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

import { useEffect, useState, useCallback } from 'react';
import { api, type SearchResult } from '../../api/client';
import { useWorkspaceStore } from '../../stores/workspace';

const TYPE_ICONS: Record<string, string> = {
  message: '#',
  post: '📝',
  task: '☑',
  file: '📎',
};

const TYPE_COLORS: Record<string, string> = {
  message: 'text-blue-400',
  post: 'text-green-400',
  task: 'text-yellow-400',
  file: 'text-purple-400',
};

export function SearchView() {
  const workspace = useWorkspaceStore((s) => s.activeWorkspace);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedType, setSelectedType] = useState<string>('');

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim() || !workspace) return;
    setLoading(true);
    try {
      const r = await api.search(q, workspace.id);
      setResults(r);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, [workspace]);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (query.trim().length >= 2) {
        doSearch(query);
      } else {
        setResults([]);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [query, doSearch]);

  if (!workspace) {
    return <div className="p-6 text-[var(--text-secondary)]">Select a workspace</div>;
  }

  const filtered = selectedType ? results.filter((r) => r.type === selectedType) : results;

  const typeCounts = results.reduce((acc, r) => {
    acc[r.type] = (acc[r.type] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  return (
    <div className="flex flex-col h-full">
      <div className="p-4 border-b border-[var(--border)]">
        <div className="relative">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search messages, posts, tasks, files..."
            className="w-full bg-[var(--bg-tertiary)] text-[var(--text-primary)] rounded-lg px-4 py-2.5 pl-9 text-sm border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none"
            autoFocus
          />
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] text-sm">
            &#128269;
          </span>
        </div>

        {/* Type filters */}
        {results.length > 0 && (
          <div className="flex items-center gap-3 mt-2">
            {Object.entries(typeCounts).map(([type, count]) => (
              <button
                key={type}
                onClick={() => setSelectedType(selectedType === type ? '' : type)}
                className={`text-xs px-2 py-0.5 rounded transition-colors ${selectedType === type ? 'bg-[var(--bg-tertiary)] text-white' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}`}
              >
                {TYPE_ICONS[type]} {type} ({count})
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        {loading && <div className="text-sm text-[var(--text-secondary)]">Searching...</div>}
        {!loading && query.trim().length >= 2 && results.length === 0 && (
          <div className="text-center text-[var(--text-secondary)] py-8">
            <p>No results found for "{query}"</p>
          </div>
        )}
        <div className="space-y-2">
          {filtered.map((result) => (
            <div key={`${result.type}-${result.id}`} className="bg-[var(--bg-secondary)] rounded-lg p-3 border border-[var(--border)] hover:border-[var(--accent)]/30 transition-colors">
              <div className="flex items-center gap-2 mb-1">
                <span className={`text-xs ${TYPE_COLORS[result.type]}`}>{TYPE_ICONS[result.type]}</span>
                <span className="text-xs text-[var(--text-muted)] capitalize">{result.type}</span>
                {result.author && (
                  <span className="text-xs text-[var(--text-secondary)]">
                    &middot; {result.author}
                  </span>
                )}
                <span className="text-[10px] text-[var(--text-muted)] ml-auto">
                  {new Date(result.date).toLocaleDateString()}
                </span>
              </div>
              <p className="text-sm text-[var(--text-primary)]">{result.title}</p>
              {result.preview !== result.title && (
                <p className="text-xs text-[var(--text-secondary)] mt-1 line-clamp-2">{result.preview}</p>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

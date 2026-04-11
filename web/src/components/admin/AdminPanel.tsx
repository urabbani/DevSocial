import { useState, useEffect } from 'react';
import { api } from '../../api/client';

export function AdminPanel({ onClose }: { onClose: () => void }) {
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [models, setModels] = useState<string[]>([]);
  const [health, setHealth] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [reindexing, setReindexing] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    load();
  }, []);

  async function load() {
    setLoading(true);
    try {
      const [s, m, h] = await Promise.all([
        api.getSettings().catch(() => ({})),
        api.getModels().catch(() => []),
        api.getHealth().catch(() => ({})),
      ]);
      setSettings(s);
      setModels(m);
      setHealth(h);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load settings');
    }
    setLoading(false);
  }

  async function save() {
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      await api.updateSettings(settings);
      setSuccess('Settings saved');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save');
    }
    setSaving(false);
  }

  async function reindexEmbeddings() {
    setReindexing(true);
    setError('');
    setSuccess('');
    try {
      await api.reindexEmbeddings();
      setSuccess('Re-indexing started. This may take several minutes.');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start re-indexing');
    }
    setReindexing(false);
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-[var(--text-secondary)]">Loading settings...</div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)]">
      <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] shrink-0">
        <h2 className="text-lg font-semibold">Admin Settings</h2>
        <button onClick={onClose} className="text-[var(--text-muted)] hover:text-white text-xl px-2">
          x
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* Health Status */}
        <section>
          <h3 className="text-sm font-bold text-[var(--text-muted)] uppercase tracking-wider mb-3">
            Service Status
          </h3>
          <div className="grid grid-cols-2 gap-3">
            {Object.entries(health).map(([service, status]) => (
              <div key={service} className="flex items-center gap-2 p-3 rounded bg-[var(--bg-secondary)]">
                <span className={`w-2 h-2 rounded-full ${status === 'ok' ? 'bg-[var(--green)]' : 'bg-[var(--red)]'}`} />
                <span className="text-sm font-medium capitalize">{service}</span>
                <span className="text-xs text-[var(--text-muted)] ml-auto">
                  {status === 'ok' ? 'Connected' : status}
                </span>
              </div>
            ))}
          </div>
        </section>

        {/* AI Model */}
        <section>
          <h3 className="text-sm font-bold text-[var(--text-muted)] uppercase tracking-wider mb-3">
            AI Configuration
          </h3>
          <div className="space-y-4 bg-[var(--bg-secondary)] rounded p-4">
            <div>
              <label className="block text-sm text-[var(--text-secondary)] mb-1">Active Model</label>
              <select
                value={settings.ai_model || ''}
                onChange={(e) => setSettings({ ...settings, ai_model: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)]"
              >
                {models.length > 0 ? (
                  models.map((m) => (
                    <option key={m} value={m}>{m}</option>
                  ))
                ) : (
                  <option value={settings.ai_model || 'claude-sonnet'}>{settings.ai_model || 'claude-sonnet'}</option>
                )}
              </select>
            </div>
            <div>
              <label className="block text-sm text-[var(--text-secondary)] mb-1">Fallback Model</label>
              <input
                type="text"
                value={settings.ai_fallback_model || ''}
                onChange={(e) => setSettings({ ...settings, ai_fallback_model: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)]"
                placeholder="gpt-4o"
              />
            </div>
            <div>
              <label className="block text-sm text-[var(--text-secondary)] mb-1">Temperature</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="2"
                value={settings.ai_temperature || '0.7'}
                onChange={(e) => setSettings({ ...settings, ai_temperature: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)]"
              />
            </div>
            <div>
              <label className="block text-sm text-[var(--text-secondary)] mb-1">System Prompt</label>
              <textarea
                value={settings.ai_system_prompt || ''}
                onChange={(e) => setSettings({ ...settings, ai_system_prompt: e.target.value })}
                rows={4}
                className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)] resize-y"
              />
            </div>
            <div>
              <label className="block text-sm text-[var(--text-secondary)] mb-1">Max Context Messages</label>
              <input
                type="number"
                min="5"
                max="200"
                value={settings.ai_max_context_messages || '50'}
                onChange={(e) => setSettings({ ...settings, ai_max_context_messages: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)]"
              />
            </div>
          </div>
        </section>

        {/* Search & Embeddings */}
        <section>
          <h3 className="text-sm font-bold text-[var(--text-muted)] uppercase tracking-wider mb-3">
            Search & Embeddings
          </h3>
          <div className="space-y-4 bg-[var(--bg-secondary)] rounded p-4">
            <div>
              <label className="block text-sm text-[var(--text-secondary)] mb-1">Search Mode</label>
              <select
                value={settings.ai_search_mode || 'keyword'}
                onChange={(e) => setSettings({ ...settings, ai_search_mode: e.target.value })}
                className="w-full px-3 py-2 bg-[var(--bg-primary)] border border-[var(--border)] rounded text-sm text-[var(--text-primary)]"
              >
                <option value="keyword">Keyword (exact match)</option>
                <option value="semantic">Semantic (meaning-based)</option>
                <option value="hybrid">Hybrid (keyword + semantic)</option>
              </select>
            </div>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm font-medium text-[var(--text-primary)]">Re-index Embeddings</div>
                <div className="text-xs text-[var(--text-muted)]">Generate embeddings for semantic search</div>
              </div>
              <button
                onClick={reindexEmbeddings}
                disabled={reindexing}
                className="px-4 py-2 bg-[var(--bg-tertiary)] text-white rounded hover:bg-[var(--accent)] disabled:opacity-50 text-sm"
              >
                {reindexing ? 'Starting...' : 'Re-index'}
              </button>
            </div>
          </div>
        </section>

        {error && <div className="text-sm text-[var(--red)] bg-[var(--red)]/10 rounded p-3">{error}</div>}
        {success && <div className="text-sm text-[var(--green)] bg-[var(--green)]/10 rounded p-3">{success}</div>}
      </div>

      <div className="px-6 py-4 border-t border-[var(--border)] shrink-0">
        <button
          onClick={save}
          disabled={saving}
          className="px-6 py-2 bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] disabled:opacity-50 text-sm font-medium"
        >
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
      </div>
    </div>
  );
}

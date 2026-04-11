import { useState, useEffect, useCallback } from 'react';
import { api } from '../../api/client';

interface OutputPanelProps {
  documentId: number | null;
  language: string;
}

interface ExecutionResult {
  exit_code: number;
  stdout: string;
  stderr: string;
  duration?: string;
}

type ExecutionStatus = 'idle' | 'running' | 'success' | 'error';

export function OutputPanel({ documentId, language }: OutputPanelProps) {
  const [status, setStatus] = useState<ExecutionStatus>('idle');
  const [output, setOutput] = useState('');
  const [error, setError] = useState('');
  const [duration, setDuration] = useState('');
  const [isCollapsed, setIsCollapsed] = useState(false);

  // Clear output when document changes
  useEffect(() => {
    setStatus('idle');
    setOutput('');
    setError('');
    setDuration('');
  }, [documentId]);

  // Listen for run-code event from Monaco editor (Ctrl+Enter)
  useEffect(() => {
    const handleRunCode = () => {
      handleExecute();
    };

    window.addEventListener('run-code', handleRunCode);
    return () => window.removeEventListener('run-code', handleRunCode);
  }, [documentId]);

  const handleExecute = useCallback(async () => {
    if (!documentId) return;

    setStatus('running');
    setOutput('');
    setError('');

    try {
      const result: ExecutionResult = await api.executeDocument(documentId);

      if (result.exit_code === 0) {
        setStatus('success');
        setOutput(result.stdout);
      } else {
        setStatus('error');
        setOutput(result.stdout);
        setError(result.stderr);
      }
      setDuration(result.duration || '');
    } catch (err) {
      setStatus('error');
      setError((err as Error).message);
    }
  }, [documentId]);

  const handleClear = () => {
    setStatus('idle');
    setOutput('');
    setError('');
    setDuration('');
  };

  if (isCollapsed) {
    return (
      <div className="h-8 border-t border-[var(--border)] bg-[var(--bg-secondary)] flex items-center px-3 justify-between">
        <button
          onClick={() => setIsCollapsed(false)}
          className="text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] flex items-center gap-1"
        >
          <span>▶</span>
          <span>Output</span>
          {status !== 'idle' && (
            <span className={`ml-2 w-2 h-2 rounded-full ${
              status === 'running' ? 'bg-yellow-500 animate-pulse' :
              status === 'success' ? 'bg-green-500' :
              status === 'error' ? 'bg-red-500' : 'bg-gray-500'
            }`} />
          )}
        </button>
      </div>
    );
  }

  return (
    <div className="h-48 border-t border-[var(--border)] bg-[var(--bg-secondary)] flex flex-col">
      {/* Header */}
      <div className="h-8 border-b border-[var(--border)] flex items-center px-3 justify-between shrink-0">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setIsCollapsed(true)}
            className="text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
          >
            ▼
          </button>
          <span className="text-xs font-medium text-[var(--text-primary)]">Output</span>
          {status !== 'idle' && (
            <>
              <span className="text-xs px-2 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--text-muted)]">
                {language}
              </span>
              <span className={`ml-2 w-2 h-2 rounded-full ${
                status === 'running' ? 'bg-yellow-500 animate-pulse' :
                status === 'success' ? 'bg-green-500' :
                status === 'error' ? 'bg-red-500' : 'bg-gray-500'
              }`} />
              {duration && <span className="text-xs text-[var(--text-muted)] ml-1">{duration}</span>}
            </>
          )}
        </div>
        <div className="flex items-center gap-2">
          {status !== 'idle' && (
            <button
              onClick={handleClear}
              className="text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            >
              Clear
            </button>
          )}
          <button
            onClick={handleExecute}
            disabled={!documentId || status === 'running'}
            className="text-xs px-2 py-1 bg-[var(--accent)] text-white rounded hover:opacity-90 transition-opacity disabled:opacity-50 flex items-center gap-1"
            title="Run code (Ctrl+Enter)"
          >
            {status === 'running' ? (
              <>
                <span className="animate-spin">◌</span>
                Running...
              </>
            ) : (
              <>
                <span>▶</span>
                Run
              </>
            )}
          </button>
        </div>
      </div>

      {/* Output Content */}
      <div className="flex-1 overflow-auto p-3 font-mono text-xs">
        {status === 'idle' && !output && !error && (
          <div className="h-full flex items-center justify-center text-[var(--text-muted)]">
            <p>Press <kbd className="px-1 py-0.5 bg-[var(--bg-tertiary)] rounded">Ctrl+Enter</kbd> or click Run to execute code</p>
          </div>
        )}

        {error && (
          <div className="text-red-400 whitespace-pre-wrap mb-2">{error}</div>
        )}

        {output && (
          <pre className="whitespace-pre-wrap text-[var(--text-primary)]">{output}</pre>
        )}

        {status === 'running' && !output && !error && (
          <div className="h-full flex items-center justify-center text-[var(--text-muted)]">
            <span className="animate-pulse">Executing...</span>
          </div>
        )}
      </div>
    </div>
  );
}

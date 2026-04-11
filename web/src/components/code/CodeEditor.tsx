import { useEffect, useRef } from 'react';
import Editor from '@monaco-editor/react';

interface CodeEditorProps {
  value: string;
  language: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  height?: string;
}

const LANGUAGE_MAP: Record<string, string> = {
  python: 'python',
  javascript: 'javascript',
  typescript: 'typescript',
  go: 'go',
  rust: 'rust',
  java: 'java',
  c: 'c',
  cpp: 'cpp',
  csharp: 'csharp',
  php: 'php',
  ruby: 'ruby',
  swift: 'swift',
  kotlin: 'kotlin',
  bash: 'shell',
  html: 'html',
  css: 'css',
  scss: 'scss',
  json: 'json',
  xml: 'xml',
  yaml: 'yaml',
  markdown: 'markdown',
  sql: 'sql',
  dockerfile: 'dockerfile',
  text: 'plaintext',
};

export function CodeEditor({
  value,
  language,
  onChange,
  readOnly = false,
  height = '100%',
}: CodeEditorProps) {
  const editorRef = useRef<any>(null);

  const monacoLanguage = LANGUAGE_MAP[language] || 'plaintext';

  const handleEditorDidMount = (editor: any) => {
    editorRef.current = editor;

    // Add keyboard shortcuts
    editor.addCommand(window.monaco.KeyMod.CtrlCmd | window.monaco.KeyCode.KeyS, () => {
      // Save is handled by the parent component with auto-save
      // Just trigger a save event
      editor.trigger('save', 'save', {});
    });

    editor.addCommand(window.monaco.KeyMod.CtrlCmd | window.monaco.KeyCode.Enter, () => {
      // Run code - trigger custom event
      const event = new CustomEvent('run-code');
      window.dispatchEvent(event);
    });
  };

  return (
    <div className="h-full w-full">
      <Editor
        height={height}
        language={monacoLanguage}
        value={value}
        onChange={(newValue) => onChange(newValue || '')}
        theme="vs-dark"
        options={{
          readOnly,
          minimap: { enabled: true },
          fontSize: 14,
          lineNumbers: 'on',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          wordWrap: 'on',
          formatOnPaste: true,
          formatOnType: true,
        }}
        onMount={handleEditorDidMount}
      />
    </div>
  );
}

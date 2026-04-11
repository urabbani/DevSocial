import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import { ToolCallBlock } from './ToolCallBlock';
import { ChartBlock } from './ChartBlock';
import 'highlight.js/styles/github-dark.css';

interface MarkdownRendererProps {
  content: string;
  toolCalls?: any[];
}

// Custom code block renderer that handles ```chart and ```tool-call blocks
const CodeBlock = ({ children, className, ...props }: any) => {
  const match = /language-(\w+)/.exec(className || '');
  const language = match ? match[1] : '';

  if (!match) {
    return (
      <code className={className} {...props}>
        {children}
      </code>
    );
  }

  const codeContent = String(children).replace(/\n$/, '');

  // Handle chart blocks
  if (language === 'chart') {
    return <ChartBlock code={codeContent} />;
  }

  // Handle tool-call blocks (for displaying tool calls in markdown)
  if (language === 'tool-call') {
    try {
      const toolCall = JSON.parse(codeContent);
      return <ToolCallBlock toolCall={toolCall} />;
    } catch {
      return <pre className={className}>{children}</pre>;
    }
  }

  // Regular code blocks
  return (
    <pre className={className} {...props}>
      <code className={className}>{children}</code>
    </pre>
  );
};

export function MarkdownRenderer({ content, toolCalls }: MarkdownRendererProps) {
  return (
    <div className="prose prose-invert max-w-none prose-sm">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
          code: CodeBlock,
          // Custom link rendering for security
          a: ({ href, children, ...props }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-[var(--accent)] hover:underline"
              {...props}
            >
              {children}
            </a>
          ),
          // Custom paragraph styling
          p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
          // Custom list styling
          ul: ({ children }) => <ul className="list-disc list-inside mb-2">{children}</ul>,
          ol: ({ children }) => <ol className="list-decimal list-inside mb-2">{children}</ol>,
          // Custom heading styling
          h1: ({ children }) => <h1 className="text-xl font-bold mb-2">{children}</h1>,
          h2: ({ children }) => <h2 className="text-lg font-bold mb-2">{children}</h2>,
          h3: ({ children }) => <h3 className="text-md font-semibold mb-1">{children}</h3>,
        }}
      >
        {content}
      </ReactMarkdown>

      {/* Render tool calls below the markdown content */}
      {toolCalls && toolCalls.length > 0 && (
        <div className="mt-3 space-y-2">
          {toolCalls.map((toolCall) => (
            <ToolCallBlock key={toolCall.id} toolCall={toolCall} />
          ))}
        </div>
      )}
    </div>
  );
}

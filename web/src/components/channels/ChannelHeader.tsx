import type { Channel } from '../../api/client';

interface Props {
  channel: Channel;
}

export function ChannelHeader({ channel }: Props) {
  const icon = channel.type === 'ai' ? 'AI' : '#';
  const iconColor = channel.type === 'ai' ? 'text-[var(--accent)]' : 'text-[var(--text-muted)]';

  return (
    <div className="h-12 flex items-center px-4 border-b border-[var(--border)] shrink-0 gap-2">
      <span className={`font-semibold ${iconColor}`}>{icon}</span>
      <span className="font-semibold text-sm">{channel.name}</span>
      {channel.description && (
        <span className="text-xs text-[var(--text-muted)] ml-1">— {channel.description}</span>
      )}
    </div>
  );
}

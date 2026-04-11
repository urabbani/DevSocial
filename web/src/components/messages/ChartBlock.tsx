import { useEffect, useRef, useState } from 'react';
import * as echarts from 'echarts';

interface ChartBlockProps {
  code: string; // JSON string containing chart configuration
}

export function ChartBlock({ code }: ChartBlockProps) {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!chartRef.current) return;

    // Parse chart configuration
    let config: any;
    try {
      config = JSON.parse(code);
    } catch (err) {
      setError(`Invalid chart JSON: ${err instanceof Error ? err.message : 'Unknown error'}`);
      return;
    }

    // Initialize chart
    if (!chartInstance.current) {
      chartInstance.current = echarts.init(chartRef.current, 'dark');
    }

    try {
      // Apply configuration
      chartInstance.current.setOption({
        backgroundColor: 'transparent',
        ...config,
      });
      setError(null);
    } catch (err) {
      setError(`Failed to render chart: ${err instanceof Error ? err.message : 'Unknown error'}`);
    }

    // Handle resize
    const handleResize = () => {
      chartInstance.current?.resize();
    };

    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chartInstance.current?.dispose();
      chartInstance.current = null;
    };
  }, [code]);

  if (error) {
    return (
      <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-3 text-sm text-red-400">
        <div className="font-medium mb-1">Chart Error</div>
        <div className="text-xs">{error}</div>
      </div>
    );
  }

  return (
    <div
      ref={chartRef}
      className="w-full h-80 bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)]"
      style={{ minHeight: '320px' }}
    />
  );
}

'use client';

import { useEffect, useState, useRef } from 'react';
import { getContainerStats } from '@/lib/api';
import type { ContainerStatsPoint } from '@/types';

// Chart.js imports (loaded from CDN)
declare const Chart: {
  new (ctx: CanvasRenderingContext2D, config: unknown): {
    destroy: () => void;
    update: () => void;
  };
};

interface InlineChartProps {
  hostId: number;
  containerId: string;
  compact?: boolean; // true for table view (40px), false for card view (128px)
}

export default function InlineChart({ hostId, containerId, compact = false }: InlineChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<ReturnType<typeof Chart.prototype.constructor> | null>(null);
  const [hasData, setHasData] = useState<boolean | null>(null); // null = loading
  const [chartLoaded, setChartLoaded] = useState(false);

  const height = compact ? 40 : 128;

  // Wait for Chart.js to load
  useEffect(() => {
    let attempts = 0;
    const maxAttempts = 50; // 5 seconds total

    const checkChartJs = () => {
      if (typeof Chart !== 'undefined') {
        console.log('[InlineChart] Chart.js loaded successfully');
        setChartLoaded(true);
      } else if (attempts < maxAttempts) {
        attempts++;
        setTimeout(checkChartJs, 100);
      } else {
        console.error('[InlineChart] Chart.js failed to load after 5 seconds');
        setHasData(false);
      }
    };
    checkChartJs();
  }, []);

  useEffect(() => {
    if (!chartLoaded) return;

    let mounted = true;

    const loadAndRender = async () => {
      try {
        const stats = await getContainerStats(hostId, containerId, '1h');
        console.log(`[InlineChart] Stats for ${containerId}:`, stats?.length || 0, 'points');

        if (!mounted) return;

        if (!stats || stats.length === 0) {
          console.log(`[InlineChart] No stats data for ${containerId}`);
          setHasData(false);
          return;
        }

        console.log(`[InlineChart] Rendering chart for ${containerId}`);

        const canvas = canvasRef.current;
        if (!canvas) {
          console.log(`[InlineChart] Canvas ref is null for ${containerId}`);
          return;
        }

        // Destroy existing chart
        if (chartRef.current) {
          chartRef.current.destroy();
          chartRef.current = null;
        }

        const ctx = canvas.getContext('2d');
        if (!ctx) {
          console.log(`[InlineChart] Could not get 2d context for ${containerId}`);
          return;
        }

        // Take last 20 points for sparkline
        const recentStats = stats.slice(-20);
        const cpuData = recentStats.map((s: ContainerStatsPoint) => s.cpu_percent || 0);
        const memoryData = recentStats.map((s: ContainerStatsPoint) => (s.memory_usage || 0) / 1024 / 1024);

        // Set canvas dimensions explicitly
        const parentWidth = canvas.parentElement?.offsetWidth || 500;
        canvas.width = parentWidth;
        canvas.height = height;
        console.log(`[InlineChart] Canvas dimensions for ${containerId}: ${canvas.width}x${canvas.height}`);

        chartRef.current = new Chart(ctx, {
          type: 'line',
          data: {
            labels: recentStats.map(() => ''),
            datasets: [
              {
                label: 'CPU %',
                data: cpuData,
                borderColor: 'rgb(75, 192, 192)',
                backgroundColor: 'rgba(75, 192, 192, 0.1)',
                borderWidth: compact ? 1.5 : 2,
                pointRadius: 0,
                tension: 0.4,
                yAxisID: 'y',
                fill: true,
              },
              {
                label: 'Memory MB',
                data: memoryData,
                borderColor: 'rgb(255, 99, 132)',
                backgroundColor: 'rgba(255, 99, 132, 0.1)',
                borderWidth: compact ? 1.5 : 2,
                pointRadius: 0,
                tension: 0.4,
                yAxisID: 'y1',
                fill: true,
              },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
              mode: 'index',
              intersect: false,
            },
            plugins: {
              legend: {
                display: !compact, // Hide legend in compact mode
                position: 'top',
                labels: {
                  boxWidth: 10,
                  padding: 6,
                  font: { size: 10 },
                  color: '#94a3b8',
                },
              },
              tooltip: {
                enabled: true,
                mode: 'index',
                intersect: false,
                callbacks: {
                  label: function(context: { dataset: { label?: string; yAxisID?: string }; parsed: { y: number | null } }) {
                    let label = context.dataset.label || '';
                    if (label) label += ': ';
                    if (context.parsed.y !== null) {
                      label += context.parsed.y.toFixed(2);
                      if (context.dataset.yAxisID === 'y') {
                        label += '%';
                      } else {
                        label += ' MB';
                      }
                    }
                    return label;
                  },
                },
              },
            },
            scales: {
              x: { display: false },
              y: {
                display: !compact, // Hide axis labels in compact mode
                beginAtZero: true,
                position: 'left',
                title: {
                  display: !compact,
                  text: 'CPU %',
                  font: { size: 9 },
                  color: '#94a3b8'
                },
                ticks: {
                  display: !compact,
                  font: { size: 8 },
                  color: '#94a3b8'
                },
                grid: {
                  display: !compact,
                  color: 'rgba(148, 163, 184, 0.1)'
                },
              },
              y1: {
                display: !compact, // Hide axis labels in compact mode
                beginAtZero: true,
                position: 'right',
                title: {
                  display: !compact,
                  text: 'Memory MB',
                  font: { size: 9 },
                  color: '#94a3b8'
                },
                ticks: {
                  display: !compact,
                  font: { size: 8 },
                  color: '#94a3b8'
                },
                grid: { drawOnChartArea: false },
              },
            },
          },
        });

        console.log(`[InlineChart] Chart created successfully for ${containerId}`);
        setHasData(true);
      } catch (error) {
        console.error('Error loading inline chart:', error);
        if (mounted) setHasData(false);
      }
    };

    loadAndRender();

    return () => {
      mounted = false;
      if (chartRef.current) {
        chartRef.current.destroy();
        chartRef.current = null;
      }
    };
  }, [chartLoaded, hostId, containerId, height, compact]);

  return (
    <div className={`relative ${compact ? 'h-10' : 'h-32'}`}>
      <canvas ref={canvasRef} className="w-full h-full"></canvas>
      {!chartLoaded && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--text-secondary)] bg-[var(--bg-tertiary)]">
          Loading Chart.js...
        </div>
      )}
      {chartLoaded && hasData === null && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--text-secondary)] bg-[var(--bg-tertiary)]">
          Loading chart data...
        </div>
      )}
      {chartLoaded && hasData === false && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--text-secondary)] bg-[var(--bg-tertiary)]">
          No stats data available
        </div>
      )}
    </div>
  );
}

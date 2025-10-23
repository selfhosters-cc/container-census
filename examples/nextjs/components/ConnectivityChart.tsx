/**
 * Container Connectivity Chart Client Component
 *
 * This component displays a bar chart showing container connectivity metrics
 * including average connections, dependency usage, and compose projects.
 *
 * Usage:
 * ```tsx
 * import { ConnectivityChart } from '@/components/ConnectivityChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const metrics = await api.getConnectionMetrics({ days: 30 });
 *
 *   return <ConnectivityChart metrics={metrics} />;
 * }
 * ```
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { ConnectionMetrics } from '@/lib/telemetry-api';

// Register Chart.js components
Chart.register(...registerables);

interface ConnectivityChartProps {
  metrics: ConnectionMetrics;
  title?: string;
}

export function ConnectivityChart({
  metrics,
  title = '🔗 Container Connectivity'
}: ConnectivityChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<Chart | null>(null);

  useEffect(() => {
    if (!canvasRef.current) return;

    // Destroy existing chart
    if (chartRef.current) {
      chartRef.current.destroy();
    }

    const ctx = canvasRef.current.getContext('2d');
    if (!ctx) return;

    const config: ChartConfiguration = {
      type: 'bar',
      data: {
        labels: ['Avg Connections', 'With Dependencies', 'Total Projects'],
        datasets: [{
          label: 'Metrics',
          data: [
            parseFloat(metrics.avg_connections_per_container.toFixed(2)),
            metrics.containers_with_deps,
            metrics.compose_project_count
          ],
          backgroundColor: ['#3498db', '#e74c3c', '#f39c12'],
          borderWidth: 2,
          borderColor: '#fff',
          borderRadius: 6
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        plugins: {
          legend: {
            display: false
          },
          title: {
            display: true,
            text: title,
            font: {
              size: 18,
              weight: 'bold'
            }
          },
          tooltip: {
            backgroundColor: 'rgba(0, 0, 0, 0.8)',
            padding: 12,
            callbacks: {
              label: function(context) {
                const label = context.label || '';
                const value = context.parsed.y;
                if (label === 'Avg Connections') {
                  return `${label}: ${value.toFixed(2)} per container`;
                } else if (label === 'With Dependencies') {
                  return `${label}: ${value.toLocaleString()} containers`;
                } else {
                  return `${label}: ${value.toLocaleString()} projects`;
                }
              }
            }
          }
        },
        scales: {
          y: {
            beginAtZero: true,
            grid: {
              color: 'rgba(0, 0, 0, 0.05)'
            }
          },
          x: {
            grid: {
              display: false
            }
          }
        }
      }
    };

    chartRef.current = new Chart(ctx, config);

    return () => {
      if (chartRef.current) {
        chartRef.current.destroy();
      }
    };
  }, [metrics, title]);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <canvas ref={canvasRef}></canvas>
      <div className="mt-4 text-center text-sm text-gray-600 dark:text-gray-400">
        <p>{metrics.total_dependencies.toLocaleString()} total dependencies across {metrics.containers_with_deps.toLocaleString()} containers</p>
      </div>
    </div>
  );
}

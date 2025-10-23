/**
 * Shared Volumes Chart Client Component
 *
 * This component displays a doughnut chart showing volume sharing patterns
 * across monitored containers.
 *
 * Usage:
 * ```tsx
 * import { SharedVolumesChart } from '@/components/SharedVolumesChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const metrics = await api.getConnectionMetrics({ days: 30 });
 *
 *   return <SharedVolumesChart metrics={metrics} />;
 * }
 * ```
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { ConnectionMetrics } from '@/lib/telemetry-api';

// Register Chart.js components
Chart.register(...registerables);

interface SharedVolumesChartProps {
  metrics: ConnectionMetrics;
  title?: string;
}

export function SharedVolumesChart({
  metrics,
  title = '📦 Shared Volumes Usage'
}: SharedVolumesChartProps) {
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
      type: 'doughnut',
      data: {
        labels: ['Shared Volumes', 'Other Volumes'],
        datasets: [{
          data: [
            metrics.shared_volume_count,
            metrics.total_volumes - metrics.shared_volume_count
          ],
          backgroundColor: ['#9b59b6', '#bdc3c7'],
          borderWidth: 2,
          borderColor: '#fff',
          hoverOffset: 15
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        plugins: {
          legend: {
            position: 'bottom',
            labels: {
              padding: 15,
              font: {
                size: 12
              }
            }
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
                const value = context.parsed.toLocaleString();
                const total = metrics.total_volumes;
                const percentage = total > 0 ? ((context.parsed / total) * 100).toFixed(1) : '0';
                return `${label}: ${value} (${percentage}%)`;
              }
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
        <p>{metrics.shared_volume_count.toLocaleString()} volumes shared between 2+ containers</p>
      </div>
    </div>
  );
}

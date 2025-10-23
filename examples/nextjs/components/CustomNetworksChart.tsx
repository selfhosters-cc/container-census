/**
 * Custom Networks Chart Client Component
 *
 * This component displays a bar chart showing network usage patterns,
 * distinguishing between custom and default Docker networks.
 *
 * Usage:
 * ```tsx
 * import { CustomNetworksChart } from '@/components/CustomNetworksChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const metrics = await api.getConnectionMetrics({ days: 30 });
 *
 *   return <CustomNetworksChart metrics={metrics} />;
 * }
 * ```
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { ConnectionMetrics } from '@/lib/telemetry-api';

// Register Chart.js components
Chart.register(...registerables);

interface CustomNetworksChartProps {
  metrics: ConnectionMetrics;
  title?: string;
}

export function CustomNetworksChart({
  metrics,
  title = '🕸️ Custom Networks'
}: CustomNetworksChartProps) {
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
        labels: ['Total Networks', 'Custom Networks', 'Default Networks'],
        datasets: [{
          label: 'Network Count',
          data: [
            metrics.network_count,
            metrics.custom_network_count,
            metrics.network_count - metrics.custom_network_count
          ],
          backgroundColor: ['#3498db', '#2ecc71', '#95a5a6'],
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
                const value = context.parsed.y.toLocaleString();
                return `${label}: ${value}`;
              }
            }
          }
        },
        scales: {
          y: {
            beginAtZero: true,
            ticks: {
              stepSize: 1
            },
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
        <p>{metrics.custom_network_count.toLocaleString()} user-created networks (excludes bridge, host, none)</p>
      </div>
    </div>
  );
}

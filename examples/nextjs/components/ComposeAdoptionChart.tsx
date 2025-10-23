/**
 * Compose Adoption Chart Client Component
 *
 * This component displays a pie chart showing Docker Compose adoption
 * across all monitored installations.
 *
 * Usage:
 * ```tsx
 * import { ComposeAdoptionChart } from '@/components/ComposeAdoptionChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const metrics = await api.getConnectionMetrics({ days: 30 });
 *
 *   return <ComposeAdoptionChart metrics={metrics} />;
 * }
 * ```
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { ConnectionMetrics } from '@/lib/telemetry-api';

// Register Chart.js components
Chart.register(...registerables);

interface ComposeAdoptionChartProps {
  metrics: ConnectionMetrics;
  title?: string;
}

export function ComposeAdoptionChart({
  metrics,
  title = '🐳 Docker Compose Adoption'
}: ComposeAdoptionChartProps) {
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
      type: 'pie',
      data: {
        labels: [
          `Using Compose (${metrics.compose_percentage.toFixed(1)}%)`,
          `Not Using Compose (${(100 - metrics.compose_percentage).toFixed(1)}%)`
        ],
        datasets: [{
          data: [
            metrics.containers_in_compose,
            metrics.total_containers - metrics.containers_in_compose
          ],
          backgroundColor: ['#3498db', '#95a5a6'],
          borderWidth: 2,
          borderColor: '#fff'
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
                return `${label}: ${value} containers`;
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
        <p>{metrics.compose_project_count} unique Compose projects</p>
        <p>{metrics.containers_in_compose.toLocaleString()} of {metrics.total_containers.toLocaleString()} containers use Compose</p>
      </div>
    </div>
  );
}

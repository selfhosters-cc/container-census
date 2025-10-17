/**
 * Top Images Chart Client Component
 *
 * This component displays a horizontal bar chart of the most popular
 * container images. It uses Chart.js for visualization and runs on
 * the client for interactivity.
 *
 * Usage:
 * ```tsx
 * import { TopImagesChart } from '@/components/TopImagesChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const images = await api.getTopImages({ limit: 10 });
 *
 *   return <TopImagesChart images={images} />;
 * }
 * ```
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { ImageCount } from '@/lib/telemetry-api';

// Register Chart.js components
Chart.register(...registerables);

interface TopImagesChartProps {
  images: ImageCount[];
  title?: string;
}

const colorPalette = [
  '#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8',
  '#F7DC6F', '#BB8FCE', '#85C1E2', '#F8B739', '#52B788',
  '#FF8FAB', '#6C5CE7', '#00D2D3', '#FDA7DF', '#74B9FF',
  '#A29BFE', '#FD79A8', '#FDCB6E', '#6C5CE7', '#00B894'
];

export function TopImagesChart({ images, title = 'Top Container Images' }: TopImagesChartProps) {
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
        labels: images.map(img => img.image),
        datasets: [{
          label: 'Container Count',
          data: images.map(img => img.count),
          backgroundColor: colorPalette,
          borderColor: colorPalette,
          borderWidth: 2,
          borderRadius: 6,
          barThickness: 'flex',
          maxBarThickness: 30
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        indexAxis: 'y',
        animation: {
          duration: 1500,
          easing: 'easeInOutQuart'
        },
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
            titleFont: {
              size: 14,
              weight: 'bold'
            },
            bodyFont: {
              size: 13
            },
            callbacks: {
              label: function(context) {
                return ' ' + context.parsed.x.toLocaleString() + ' containers';
              }
            }
          }
        },
        scales: {
          x: {
            beginAtZero: true,
            title: {
              display: true,
              text: 'Total Container Count',
              font: {
                size: 14,
                weight: 'bold'
              }
            },
            grid: {
              color: 'rgba(0, 0, 0, 0.05)'
            }
          },
          y: {
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
  }, [images, title]);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <canvas ref={canvasRef}></canvas>
    </div>
  );
}

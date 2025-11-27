/**
 * Hottest Images Chart Client Component
 *
 * Displays a dual ranking of the most popular container images:
 * - By total container count across all installations
 * - By adoption percentage (how many installations use it)
 *
 * Features rank badges with medals for top 3 positions.
 *
 * Usage:
 * ```tsx
 * import { HottestImagesChart } from '@/components/HottestImagesChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const hottest = await api.getHottest({ limit: 10, days: 7 });
 *
 *   return <HottestImagesChart data={hottest} />;
 * }
 * ```
 */

'use client';

import { useEffect, useRef, useState } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { HottestResponse, HotImage } from '@/lib/telemetry-api';

Chart.register(...registerables);

interface HottestImagesChartProps {
  data: HottestResponse;
  title?: string;
}

const colorPalette = [
  '#FFD700', '#C0C0C0', '#CD7F32', // Gold, Silver, Bronze for top 3
  '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8',
  '#F7DC6F', '#BB8FCE', '#85C1E2', '#F8B739', '#52B788',
  '#FF8FAB', '#6C5CE7', '#00D2D3', '#FDA7DF', '#74B9FF'
];

const getRankBadge = (rank: number): string => {
  switch (rank) {
    case 1: return '🥇';
    case 2: return '🥈';
    case 3: return '🥉';
    default: return `#${rank}`;
  }
};

export function HottestImagesChart({ data, title = 'Hottest Container Images' }: HottestImagesChartProps) {
  const containersCanvasRef = useRef<HTMLCanvasElement>(null);
  const adoptionCanvasRef = useRef<HTMLCanvasElement>(null);
  const containersChartRef = useRef<Chart | null>(null);
  const adoptionChartRef = useRef<Chart | null>(null);
  const [activeTab, setActiveTab] = useState<'containers' | 'adoption'>('containers');

  // Create container count chart
  useEffect(() => {
    if (!containersCanvasRef.current || !data.by_containers) return;

    if (containersChartRef.current) {
      containersChartRef.current.destroy();
    }

    const ctx = containersCanvasRef.current.getContext('2d');
    if (!ctx) return;

    const images = data.by_containers;

    const config: ChartConfiguration = {
      type: 'bar',
      data: {
        labels: images.map(img => `${getRankBadge(img.rank)} ${img.image}`),
        datasets: [{
          label: 'Total Containers',
          data: images.map(img => img.total_containers),
          backgroundColor: images.map((_, i) => colorPalette[i] || '#6C5CE7'),
          borderColor: images.map((_, i) => colorPalette[i] || '#6C5CE7'),
          borderWidth: 2,
          borderRadius: 6,
          maxBarThickness: 35
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        indexAxis: 'y',
        animation: {
          duration: 1200,
          easing: 'easeOutQuart'
        },
        plugins: {
          legend: {
            display: false
          },
          title: {
            display: true,
            text: 'By Total Containers',
            font: {
              size: 16,
              weight: 'bold'
            },
            padding: { bottom: 20 }
          },
          tooltip: {
            backgroundColor: 'rgba(0, 0, 0, 0.85)',
            padding: 14,
            titleFont: { size: 14, weight: 'bold' },
            bodyFont: { size: 13 },
            callbacks: {
              label: function(context) {
                const img = images[context.dataIndex];
                return [
                  ` ${img.total_containers.toLocaleString()} containers`,
                  ` ${img.installation_count} installations (${img.adoption_percentage}%)`
                ];
              }
            }
          }
        },
        scales: {
          x: {
            beginAtZero: true,
            title: {
              display: true,
              text: 'Container Count',
              font: { size: 13, weight: 'bold' }
            },
            grid: { color: 'rgba(0, 0, 0, 0.05)' }
          },
          y: {
            grid: { display: false },
            ticks: {
              font: { size: 12 }
            }
          }
        }
      }
    };

    containersChartRef.current = new Chart(ctx, config);

    return () => {
      if (containersChartRef.current) {
        containersChartRef.current.destroy();
      }
    };
  }, [data.by_containers]);

  // Create adoption chart
  useEffect(() => {
    if (!adoptionCanvasRef.current || !data.by_adoption) return;

    if (adoptionChartRef.current) {
      adoptionChartRef.current.destroy();
    }

    const ctx = adoptionCanvasRef.current.getContext('2d');
    if (!ctx) return;

    const images = data.by_adoption;

    const config: ChartConfiguration = {
      type: 'bar',
      data: {
        labels: images.map(img => `${getRankBadge(img.rank)} ${img.image}`),
        datasets: [{
          label: 'Adoption %',
          data: images.map(img => img.adoption_percentage),
          backgroundColor: images.map((_, i) => colorPalette[i] || '#6C5CE7'),
          borderColor: images.map((_, i) => colorPalette[i] || '#6C5CE7'),
          borderWidth: 2,
          borderRadius: 6,
          maxBarThickness: 35
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        indexAxis: 'y',
        animation: {
          duration: 1200,
          easing: 'easeOutQuart'
        },
        plugins: {
          legend: {
            display: false
          },
          title: {
            display: true,
            text: 'By Adoption Rate',
            font: {
              size: 16,
              weight: 'bold'
            },
            padding: { bottom: 20 }
          },
          tooltip: {
            backgroundColor: 'rgba(0, 0, 0, 0.85)',
            padding: 14,
            titleFont: { size: 14, weight: 'bold' },
            bodyFont: { size: 13 },
            callbacks: {
              label: function(context) {
                const img = images[context.dataIndex];
                return [
                  ` ${img.adoption_percentage}% of installations`,
                  ` ${img.installation_count} out of ${data.total_installations}`,
                  ` ${img.total_containers.toLocaleString()} total containers`
                ];
              }
            }
          }
        },
        scales: {
          x: {
            beginAtZero: true,
            max: 100,
            title: {
              display: true,
              text: 'Adoption Rate (%)',
              font: { size: 13, weight: 'bold' }
            },
            grid: { color: 'rgba(0, 0, 0, 0.05)' }
          },
          y: {
            grid: { display: false },
            ticks: {
              font: { size: 12 }
            }
          }
        }
      }
    };

    adoptionChartRef.current = new Chart(ctx, config);

    return () => {
      if (adoptionChartRef.current) {
        adoptionChartRef.current.destroy();
      }
    };
  }, [data.by_adoption, data.total_installations]);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold text-gray-900 dark:text-white">{title}</h2>
        <div className="flex gap-2">
          <button
            onClick={() => setActiveTab('containers')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              activeTab === 'containers'
                ? 'bg-purple-600 text-white'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
            }`}
          >
            By Containers
          </button>
          <button
            onClick={() => setActiveTab('adoption')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              activeTab === 'adoption'
                ? 'bg-purple-600 text-white'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
            }`}
          >
            By Adoption
          </button>
        </div>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
        Based on {data.total_installations.toLocaleString()} installations over the last {data.period_days} days
      </p>
      <div className={activeTab === 'containers' ? '' : 'hidden'}>
        <canvas ref={containersCanvasRef}></canvas>
      </div>
      <div className={activeTab === 'adoption' ? '' : 'hidden'}>
        <canvas ref={adoptionCanvasRef}></canvas>
      </div>
    </div>
  );
}

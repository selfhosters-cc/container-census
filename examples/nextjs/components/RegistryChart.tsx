/**
 * Registry Distribution Chart Client Component
 *
 * Displays container registry distribution as a doughnut chart.
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { RegistryCount } from '@/lib/telemetry-api';

Chart.register(...registerables);

interface RegistryChartProps {
  data: RegistryCount[];
  title?: string;
}

const registryColors = {
  'Docker Hub': '#2496ED',
  'Docker Hub (default)': '#2496ED',
  'GitHub Container Registry': '#24292E',
  'Quay.io': '#40B4E5',
  'Google Container Registry': '#4285F4',
  'Microsoft Container Registry': '#00A4EF',
  'Other Private Registry': '#FF6B6B',
  'Other': '#999999'
};

export function RegistryChart({ data, title = 'Registry Distribution' }: RegistryChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<Chart | null>(null);

  useEffect(() => {
    if (!canvasRef.current) return;

    if (chartRef.current) {
      chartRef.current.destroy();
    }

    const ctx = canvasRef.current.getContext('2d');
    if (!ctx) return;

    const colors = data.map(d =>
      registryColors[d.registry as keyof typeof registryColors] || '#999999'
    );

    const config: ChartConfiguration = {
      type: 'doughnut',
      data: {
        labels: data.map(d => d.registry),
        datasets: [{
          data: data.map(d => d.count),
          backgroundColor: colors,
          borderColor: '#fff',
          borderWidth: 2
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        plugins: {
          title: {
            display: true,
            text: title,
            font: {
              size: 18,
              weight: 'bold'
            }
          },
          legend: {
            display: true,
            position: 'bottom'
          },
          tooltip: {
            backgroundColor: 'rgba(0, 0, 0, 0.8)',
            padding: 12,
            callbacks: {
              label: function(context) {
                const label = context.label || '';
                const value = context.parsed;
                const total = context.dataset.data.reduce((a: number, b: number) => a + b, 0) as number;
                const percentage = ((value / total) * 100).toFixed(1);
                return ` ${label}: ${value.toLocaleString()} (${percentage}%)`;
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
  }, [data, title]);

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <canvas ref={canvasRef}></canvas>
    </div>
  );
}

/**
 * Geography Distribution Chart Client Component
 *
 * Displays geographic distribution based on timezone data as a bar chart.
 */

'use client';

import { useEffect, useRef } from 'react';
import { Chart, ChartConfiguration, registerables } from 'chart.js';
import { GeographyData } from '@/lib/telemetry-api';

Chart.register(...registerables);

interface GeographyChartProps {
  data: GeographyData[];
  title?: string;
}

const regionColors: Record<string, string> = {
  'Americas': '#FF6B6B',
  'Europe': '#4ECDC4',
  'Asia': '#45B7D1',
  'Africa': '#FFA07A',
  'Oceania': '#98D8C8',
  'Pacific': '#F7DC6F',
  'Antarctica': '#BB8FCE',
  'Other': '#85C1E2',
  'Unknown': '#999999'
};

export function GeographyChart({ data, title = 'Geographic Distribution' }: GeographyChartProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<Chart | null>(null);

  useEffect(() => {
    if (!canvasRef.current) return;

    if (chartRef.current) {
      chartRef.current.destroy();
    }

    const ctx = canvasRef.current.getContext('2d');
    if (!ctx) return;

    // Group by region
    const regionMap = new Map<string, number>();
    data.forEach(item => {
      const current = regionMap.get(item.region) || 0;
      regionMap.set(item.region, current + item.installations);
    });

    const regions = Array.from(regionMap.entries())
      .sort((a, b) => b[1] - a[1]);

    const colors = regions.map(([region]) => regionColors[region] || '#999999');

    const config: ChartConfiguration = {
      type: 'bar',
      data: {
        labels: regions.map(([region]) => region),
        datasets: [{
          label: 'Installations',
          data: regions.map(([, count]) => count),
          backgroundColor: colors,
          borderColor: colors,
          borderWidth: 2,
          borderRadius: 6
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
            display: false
          },
          tooltip: {
            backgroundColor: 'rgba(0, 0, 0, 0.8)',
            padding: 12,
            callbacks: {
              label: function(context) {
                return ` ${context.parsed.y} installation${context.parsed.y !== 1 ? 's' : ''}`;
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
            title: {
              display: true,
              text: 'Number of Installations'
            }
          },
          x: {
            title: {
              display: true,
              text: 'Region'
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

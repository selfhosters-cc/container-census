/**
 * Movers Chart Client Component
 *
 * Displays week-over-week biggest movers (risers and fallers).
 * Shows containers that gained or lost the most usage.
 *
 * Features:
 * - Two-column layout: Risers (green) | Fallers (red)
 * - Arrow indicators with percentage change
 * - Week comparison selector
 *
 * Usage:
 * ```tsx
 * import { MoversChart } from '@/components/MoversChart';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const movers = await api.getMovers({ limit: 10, weeks: 1 });
 *
 *   return <MoversChart data={movers} />;
 * }
 * ```
 */

'use client';

import { MoversResponse, Mover } from '@/lib/telemetry-api';

interface MoversChartProps {
  data: MoversResponse;
  title?: string;
}

const formatChange = (change: number): string => {
  const prefix = change > 0 ? '+' : '';
  return `${prefix}${change.toLocaleString()}`;
};

const formatPercentage = (pct: number): string => {
  const prefix = pct > 0 ? '+' : '';
  return `${prefix}${pct.toFixed(1)}%`;
};

const getRankBadge = (rank: number): string => {
  switch (rank) {
    case 1: return '🥇';
    case 2: return '🥈';
    case 3: return '🥉';
    default: return `#${rank}`;
  }
};

function MoverCard({ mover, type }: { mover: Mover; type: 'riser' | 'faller' }) {
  const isRiser = type === 'riser';
  const arrowIcon = isRiser ? '↑' : '↓';
  const colorClass = isRiser
    ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800'
    : 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800';
  const textColorClass = isRiser
    ? 'text-green-600 dark:text-green-400'
    : 'text-red-600 dark:text-red-400';
  const badgeColorClass = isRiser
    ? 'bg-green-100 dark:bg-green-800 text-green-800 dark:text-green-200'
    : 'bg-red-100 dark:bg-red-800 text-red-800 dark:text-red-200';

  return (
    <div className={`rounded-lg border p-4 ${colorClass} transition-transform hover:scale-[1.02]`}>
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-lg">{getRankBadge(mover.rank)}</span>
          <span className="font-semibold text-gray-900 dark:text-white truncate max-w-[180px]" title={mover.image}>
            {mover.image}
          </span>
        </div>
        <span className={`text-2xl font-bold ${textColorClass}`}>
          {arrowIcon}
        </span>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-sm text-gray-500 dark:text-gray-400">Change:</span>
          <span className={`font-bold ${textColorClass}`}>
            {formatChange(mover.change)} containers
          </span>
        </div>

        <div className="flex items-center justify-between">
          <span className="text-sm text-gray-500 dark:text-gray-400">Percentage:</span>
          <span className={`px-2 py-0.5 rounded-full text-sm font-medium ${badgeColorClass}`}>
            {formatPercentage(mover.change_percentage)}
          </span>
        </div>

        <div className="pt-2 border-t border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500 dark:text-gray-400">
              {mover.previous_count.toLocaleString()} → {mover.current_count.toLocaleString()}
            </span>
            <span className="text-gray-500 dark:text-gray-400">
              {mover.current_installations} installs
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

export function MoversChart({ data, title = 'Biggest Movers This Week' }: MoversChartProps) {
  const hasRisers = data.risers && data.risers.length > 0;
  const hasFallers = data.fallers && data.fallers.length > 0;
  const hasData = hasRisers || hasFallers;

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-xl font-bold text-gray-900 dark:text-white">{title}</h2>
        <span className="text-sm text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 px-3 py-1 rounded-full">
          {data.comparison_weeks === 1 ? 'Week over Week' : `${data.comparison_weeks} Weeks`}
        </span>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">
        Comparing {data.previous_week} to {data.current_week} • Min {data.min_installations} installations
      </p>

      {!hasData ? (
        <div className="text-center py-12 text-gray-500 dark:text-gray-400">
          <p className="text-lg">No movement data available yet</p>
          <p className="text-sm mt-2">Check back after weekly snapshots are generated</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Risers Column */}
          <div>
            <div className="flex items-center gap-2 mb-4">
              <span className="text-2xl">📈</span>
              <h3 className="text-lg font-semibold text-green-600 dark:text-green-400">
                Risers
              </h3>
              <span className="text-sm text-gray-500 dark:text-gray-400">
                ({data.risers?.length || 0})
              </span>
            </div>
            <div className="space-y-3">
              {hasRisers ? (
                data.risers.map((mover) => (
                  <MoverCard key={mover.image} mover={mover} type="riser" />
                ))
              ) : (
                <p className="text-gray-500 dark:text-gray-400 text-center py-8">
                  No risers this period
                </p>
              )}
            </div>
          </div>

          {/* Fallers Column */}
          <div>
            <div className="flex items-center gap-2 mb-4">
              <span className="text-2xl">📉</span>
              <h3 className="text-lg font-semibold text-red-600 dark:text-red-400">
                Fallers
              </h3>
              <span className="text-sm text-gray-500 dark:text-gray-400">
                ({data.fallers?.length || 0})
              </span>
            </div>
            <div className="space-y-3">
              {hasFallers ? (
                data.fallers.map((mover) => (
                  <MoverCard key={mover.image} mover={mover} type="faller" />
                ))
              ) : (
                <p className="text-gray-500 dark:text-gray-400 text-center py-8">
                  No fallers this period
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * New Entries Card Client Component
 *
 * Displays newly discovered container images that have gained
 * significant adoption (minimum threshold of installations).
 *
 * Features:
 * - Card grid layout with "NEW" badges
 * - Days since first seen indicator
 * - Adoption percentage progress bar
 *
 * Usage:
 * ```tsx
 * import { NewEntriesCard } from '@/components/NewEntriesCard';
 *
 * export default async function Page() {
 *   const api = createTelemetryAPI();
 *   const newEntries = await api.getNewEntries({ limit: 12, days: 30 });
 *
 *   return <NewEntriesCard data={newEntries} />;
 * }
 * ```
 */

'use client';

import { NewEntriesResponse, NewEntry } from '@/lib/telemetry-api';

interface NewEntriesCardProps {
  data: NewEntriesResponse;
  title?: string;
}

const getRankBadge = (rank: number): string => {
  switch (rank) {
    case 1: return '🥇';
    case 2: return '🥈';
    case 3: return '🥉';
    default: return `#${rank}`;
  }
};

const getDaysLabel = (days: number): string => {
  if (days === 0) return 'Today';
  if (days === 1) return '1 day ago';
  return `${days} days ago`;
};

function EntryCard({ entry }: { entry: NewEntry }) {
  const isVeryNew = entry.days_since_first_seen <= 7;
  const isRecent = entry.days_since_first_seen <= 14;

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:shadow-lg transition-shadow">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <span className="text-lg">{getRankBadge(entry.rank)}</span>
          <span
            className="font-semibold text-gray-900 dark:text-white truncate max-w-[150px]"
            title={entry.image}
          >
            {entry.image}
          </span>
        </div>
        <span className={`px-2 py-0.5 text-xs font-bold rounded-full ${
          isVeryNew
            ? 'bg-green-500 text-white animate-pulse'
            : isRecent
            ? 'bg-blue-500 text-white'
            : 'bg-purple-500 text-white'
        }`}>
          NEW
        </span>
      </div>

      <div className="space-y-3">
        {/* First Seen */}
        <div className="flex items-center justify-between text-sm">
          <span className="text-gray-500 dark:text-gray-400">First seen:</span>
          <span className="text-gray-700 dark:text-gray-300">
            {getDaysLabel(entry.days_since_first_seen)}
          </span>
        </div>

        {/* Containers */}
        <div className="flex items-center justify-between text-sm">
          <span className="text-gray-500 dark:text-gray-400">Containers:</span>
          <span className="font-semibold text-gray-900 dark:text-white">
            {entry.total_containers.toLocaleString()}
          </span>
        </div>

        {/* Installations */}
        <div className="flex items-center justify-between text-sm">
          <span className="text-gray-500 dark:text-gray-400">Installations:</span>
          <span className="font-semibold text-gray-900 dark:text-white">
            {entry.installation_count}
          </span>
        </div>

        {/* Adoption Progress Bar */}
        <div>
          <div className="flex items-center justify-between text-xs mb-1">
            <span className="text-gray-500 dark:text-gray-400">Adoption</span>
            <span className="font-medium text-purple-600 dark:text-purple-400">
              {entry.adoption_percentage}%
            </span>
          </div>
          <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
            <div
              className="bg-gradient-to-r from-purple-500 to-blue-500 h-2 rounded-full transition-all duration-500"
              style={{ width: `${Math.min(entry.adoption_percentage, 100)}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export function NewEntriesCard({ data, title = 'New Entries' }: NewEntriesCardProps) {
  const hasEntries = data.new_images && data.new_images.length > 0;

  return (
    <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-6">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-3">
          <span className="text-3xl">✨</span>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">{title}</h2>
        </div>
        {hasEntries && (
          <span className="bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200 px-3 py-1 rounded-full text-sm font-medium">
            {data.total_new_images} new
          </span>
        )}
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">
        Images first seen in the last {data.period_days} days with at least {data.min_installations} installations
      </p>

      {!hasEntries ? (
        <div className="text-center py-12 bg-white dark:bg-gray-800 rounded-lg border border-dashed border-gray-300 dark:border-gray-700">
          <span className="text-4xl mb-4 block">🔍</span>
          <p className="text-lg text-gray-600 dark:text-gray-400">No new entries yet</p>
          <p className="text-sm text-gray-500 dark:text-gray-500 mt-2">
            New container images will appear here once they reach {data.min_installations}+ installations
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {data.new_images.map((entry) => (
            <EntryCard key={entry.image} entry={entry} />
          ))}
        </div>
      )}
    </div>
  );
}

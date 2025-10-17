/**
 * Telemetry Stats Server Component
 *
 * This component fetches and displays summary statistics from the
 * telemetry collector API. It runs on the server for better SEO
 * and initial page load performance.
 *
 * Usage:
 * ```tsx
 * import { TelemetryStats } from '@/components/TelemetryStats';
 *
 * export default function Page() {
 *   return <TelemetryStats />;
 * }
 * ```
 */

import { createTelemetryAPI } from '@/lib/telemetry-api';

export async function TelemetryStats() {
  const api = createTelemetryAPI();
  const summary = await api.getSummary();

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <StatCard
        title="Total Installations"
        value={summary.installations.toLocaleString()}
        description="Unique Container Census installations"
      />
      <StatCard
        title="Total Containers"
        value={summary.total_containers.toLocaleString()}
        description="Containers being monitored"
      />
      <StatCard
        title="Total Hosts"
        value={summary.total_hosts.toLocaleString()}
        description="Docker hosts connected"
      />
      <StatCard
        title="Total Agents"
        value={summary.total_agents.toLocaleString()}
        description="Census agents deployed"
      />
      <StatCard
        title="Unique Images"
        value={summary.unique_images.toLocaleString()}
        description="Different container images"
      />
      <StatCard
        title="Total Submissions"
        value={summary.total_submissions.toLocaleString()}
        description="Telemetry reports received"
      />
    </div>
  );
}

interface StatCardProps {
  title: string;
  value: string;
  description: string;
}

function StatCard({ title, value, description }: StatCardProps) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
        {title}
      </h3>
      <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
        {value}
      </p>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
        {description}
      </p>
    </div>
  );
}

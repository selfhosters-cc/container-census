/**
 * Telemetry Dashboard Page
 *
 * This is a complete example page that displays telemetry data
 * from the Container Census telemetry collector API.
 *
 * This is a Server Component that fetches data at build time
 * (or on request if using dynamic rendering), then passes it
 * to Client Components for interactive charts.
 *
 * File location: app/telemetry/page.tsx
 */

import { createTelemetryAPI } from '@/lib/telemetry-api';
import { TelemetryStats } from '@/components/TelemetryStats';
import { TopImagesChart } from '@/components/TopImagesChart';
import { GrowthChart } from '@/components/GrowthChart';
import { RegistryChart } from '@/components/RegistryChart';

// Revalidate every 5 minutes
export const revalidate = 300;

export default async function TelemetryDashboard() {
  const api = createTelemetryAPI();

  // Fetch all data in parallel
  const [topImages, growth, registries] = await Promise.all([
    api.getTopImages({ limit: 10, days: 30 }),
    api.getGrowth({ days: 90 }),
    api.getRegistries({ days: 30 })
  ]);

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-2">
            Container Census Telemetry
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-300">
            Real-time insights from Container Census installations worldwide
          </p>
        </div>

        {/* Summary Stats */}
        <div className="mb-8">
          <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
            Overview
          </h2>
          <TelemetryStats />
        </div>

        {/* Charts Grid */}
        <div className="space-y-8">
          {/* Growth Chart */}
          <div>
            <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
              Growth Trends
            </h2>
            <GrowthChart data={growth} />
          </div>

          {/* Two Column Grid */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
            {/* Top Images */}
            <div>
              <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
                Most Popular Images
              </h2>
              <TopImagesChart images={topImages} />
            </div>

            {/* Registry Distribution */}
            <div>
              <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
                Container Registries
              </h2>
              <RegistryChart data={registries} />
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="mt-12 text-center text-sm text-gray-500 dark:text-gray-400">
          <p>
            Data is updated every 5 minutes. All statistics are based on anonymous
            telemetry from Container Census installations.
          </p>
          <p className="mt-2">
            <a
              href="https://github.com/yourusername/container-census"
              className="text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
              target="_blank"
              rel="noopener noreferrer"
            >
              Learn more about Container Census
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}

/**
 * Metadata for SEO
 */
export const metadata = {
  title: 'Container Census Telemetry',
  description: 'Real-time insights from Container Census installations worldwide',
};

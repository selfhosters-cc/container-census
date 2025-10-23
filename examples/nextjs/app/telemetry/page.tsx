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

'use client';

import { useState, useEffect } from 'react';
import { createTelemetryAPI } from '@/lib/telemetry-api';
import { TopImagesChart } from '@/components/TopImagesChart';
import { GrowthChart } from '@/components/GrowthChart';
import { RegistryChart } from '@/components/RegistryChart';
import { ContainerImagesTable } from '@/components/ContainerImagesTable';
import { ComposeAdoptionChart } from '@/components/ComposeAdoptionChart';
import { ConnectivityChart } from '@/components/ConnectivityChart';
import { SharedVolumesChart } from '@/components/SharedVolumesChart';
import { CustomNetworksChart } from '@/components/CustomNetworksChart';
import type { ImageCount, Growth, RegistryCount, ImageDetail, Summary, ConnectionMetrics } from '@/lib/telemetry-api';

type TabType = 'overview' | 'images' | 'architecture';

export default function TelemetryDashboard() {
  const [activeTab, setActiveTab] = useState<TabType>('overview');
  const [summary, setSummary] = useState<Summary | null>(null);
  const [topImages, setTopImages] = useState<ImageCount[]>([]);
  const [growth, setGrowth] = useState<Growth[]>([]);
  const [registries, setRegistries] = useState<RegistryCount[]>([]);
  const [imageDetails, setImageDetails] = useState<ImageDetail[]>([]);
  const [connectionMetrics, setConnectionMetrics] = useState<ConnectionMetrics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      try {
        const api = createTelemetryAPI();

        if (activeTab === 'overview') {
          const [summaryData, topImagesData, growthData, registriesData] = await Promise.all([
            api.getSummary(),
            api.getTopImages({ limit: 10, days: 30 }),
            api.getGrowth({ days: 90 }),
            api.getRegistries({ days: 30 })
          ]);
          setSummary(summaryData);
          setTopImages(topImagesData);
          setGrowth(growthData);
          setRegistries(registriesData);
        } else if (activeTab === 'architecture') {
          const metricsData = await api.getConnectionMetrics({ days: 30 });
          setConnectionMetrics(metricsData);
        } else {
          const response = await api.getImageDetails({ limit: 1000, days: 30 });
          setImageDetails(response.images);
        }
      } catch (error) {
        console.error('Failed to load telemetry data:', error);
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [activeTab]);

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

        {/* Tab Navigation */}
        <div className="mb-8">
          <div className="border-b border-gray-200 dark:border-gray-700">
            <nav className="-mb-px flex space-x-8">
              <button
                onClick={() => setActiveTab('overview')}
                className={`
                  py-4 px-1 border-b-2 font-medium text-sm transition-colors
                  ${activeTab === 'overview'
                    ? 'border-purple-500 text-purple-600 dark:text-purple-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }
                `}
              >
                Overview
              </button>
              <button
                onClick={() => setActiveTab('images')}
                className={`
                  py-4 px-1 border-b-2 font-medium text-sm transition-colors
                  ${activeTab === 'images'
                    ? 'border-purple-500 text-purple-600 dark:text-purple-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }
                `}
              >
                Container Images
              </button>
              <button
                onClick={() => setActiveTab('architecture')}
                className={`
                  py-4 px-1 border-b-2 font-medium text-sm transition-colors
                  ${activeTab === 'architecture'
                    ? 'border-purple-500 text-purple-600 dark:text-purple-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                  }
                `}
              >
                Architecture & Connectivity
              </button>
            </nav>
          </div>
        </div>

        {/* Tab Content */}
        {loading ? (
          <div className="text-center py-12">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-purple-600"></div>
            <p className="mt-4 text-gray-600 dark:text-gray-400">Loading...</p>
          </div>
        ) : activeTab === 'overview' ? (
          <>
            {/* Summary Stats */}
            <div className="mb-8">
              <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
                Overview
              </h2>
              {summary && (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                  <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                      Total Installations
                    </h3>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.installations.toLocaleString()}
                    </p>
                    <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                      Unique Container Census installations
                    </p>
                  </div>
                  <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                      Total Containers
                    </h3>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.total_containers.toLocaleString()}
                    </p>
                    <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                      Containers being monitored
                    </p>
                  </div>
                  <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                      Total Hosts
                    </h3>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.total_hosts.toLocaleString()}
                    </p>
                    <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                      Docker hosts connected
                    </p>
                  </div>
                  <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                      Total Agents
                    </h3>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.total_agents.toLocaleString()}
                    </p>
                    <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                      Census agents deployed
                    </p>
                  </div>
                  <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                      Unique Images
                    </h3>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.unique_images.toLocaleString()}
                    </p>
                    <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                      Different container images
                    </p>
                  </div>
                  <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                      Total Submissions
                    </h3>
                    <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                      {summary.total_submissions.toLocaleString()}
                    </p>
                    <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                      Telemetry reports received
                    </p>
                  </div>
                </div>
              )}
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
          </>
        ) : activeTab === 'architecture' ? (
          <>
            {/* Architecture Overview */}
            <div className="mb-8">
              <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
                Container Architecture & Connectivity
              </h2>
              <p className="text-gray-600 dark:text-gray-300 mb-6">
                Insights into how containers are connected through Docker Compose, networks, volumes, and dependencies.
              </p>

              {connectionMetrics && (
                <>
                  {/* Summary Cards */}
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                      <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                        Compose Adoption
                      </h3>
                      <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                        {connectionMetrics.compose_percentage.toFixed(1)}%
                      </p>
                      <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                        {connectionMetrics.containers_in_compose.toLocaleString()} of {connectionMetrics.total_containers.toLocaleString()} containers
                      </p>
                    </div>
                    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                      <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                        Custom Networks
                      </h3>
                      <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                        {connectionMetrics.custom_network_count.toLocaleString()}
                      </p>
                      <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                        of {connectionMetrics.network_count.toLocaleString()} total networks
                      </p>
                    </div>
                    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                      <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                        Shared Volumes
                      </h3>
                      <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                        {connectionMetrics.shared_volume_count.toLocaleString()}
                      </p>
                      <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                        Used by 2+ containers
                      </p>
                    </div>
                    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                      <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400">
                        Avg Connections
                      </h3>
                      <p className="mt-2 text-3xl font-semibold text-gray-900 dark:text-white">
                        {connectionMetrics.avg_connections_per_container.toFixed(2)}
                      </p>
                      <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                        Per container
                      </p>
                    </div>
                  </div>

                  {/* Charts Grid */}
                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                    <ComposeAdoptionChart metrics={connectionMetrics} />
                    <ConnectivityChart metrics={connectionMetrics} />
                    <SharedVolumesChart metrics={connectionMetrics} />
                    <CustomNetworksChart metrics={connectionMetrics} />
                  </div>
                </>
              )}
            </div>
          </>
        ) : (
          <ContainerImagesTable images={imageDetails} title="All Container Images" />
        )}

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

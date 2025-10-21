'use client';

import { useState, useMemo } from 'react';
import type { ImageDetail } from '@/lib/telemetry-api';

interface ContainerImagesTableProps {
  images: ImageDetail[];
  title?: string;
}

type SortColumn = 'name' | 'count' | 'registry' | 'installations' | 'percentage';
type SortOrder = 'asc' | 'desc';

export function ContainerImagesTable({ images, title = 'Container Images' }: ContainerImagesTableProps) {
  const [searchTerm, setSearchTerm] = useState('');
  const [sortColumn, setSortColumn] = useState<SortColumn>('count');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
  const [currentPage, setCurrentPage] = useState(0);
  const pageSize = 50;

  // Filter and sort images
  const processedImages = useMemo(() => {
    // Filter by search term
    let filtered = images;
    if (searchTerm) {
      filtered = images.filter(img =>
        img.image.toLowerCase().includes(searchTerm.toLowerCase())
      );
    }

    // Sort
    const sorted = [...filtered].sort((a, b) => {
      let aVal: string | number;
      let bVal: string | number;

      switch (sortColumn) {
        case 'name':
          aVal = a.image.toLowerCase();
          bVal = b.image.toLowerCase();
          break;
        case 'count':
          aVal = a.count;
          bVal = b.count;
          break;
        case 'registry':
          aVal = a.registry.toLowerCase();
          bVal = b.registry.toLowerCase();
          break;
        case 'installations':
          aVal = a.installation_count;
          bVal = b.installation_count;
          break;
        case 'percentage':
          // Calculate percentage for sorting
          aVal = totalContainers > 0 ? (a.count / totalContainers) * 100 : 0;
          bVal = totalContainers > 0 ? (b.count / totalContainers) * 100 : 0;
          break;
        default:
          return 0;
      }

      if (aVal < bVal) return sortOrder === 'asc' ? -1 : 1;
      if (aVal > bVal) return sortOrder === 'asc' ? 1 : -1;
      return 0;
    });

    return sorted;
  }, [images, searchTerm, sortColumn, sortOrder]);

  // Calculate pagination
  const totalPages = Math.ceil(processedImages.length / pageSize);
  const paginatedImages = processedImages.slice(
    currentPage * pageSize,
    (currentPage + 1) * pageSize
  );

  // Calculate total containers for percentage
  const totalContainers = useMemo(
    () => images.reduce((sum, img) => sum + img.count, 0),
    [images]
  );

  // Handle sort
  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortOrder('desc');
    }
    setCurrentPage(0);
  };

  // Get registry badge styles
  const getRegistryBadge = (registry: string) => {
    const badges: Record<string, { bg: string; text: string }> = {
      'Docker Hub': { bg: 'bg-blue-500', text: 'text-white' },
      'ghcr.io': { bg: 'bg-blue-600', text: 'text-white' },
      'quay.io': { bg: 'bg-cyan-500', text: 'text-white' },
      'gcr.io': { bg: 'bg-blue-400', text: 'text-white' },
      'mcr.microsoft.com': { bg: 'bg-blue-500', text: 'text-white' },
    };

    const badge = badges[registry] || { bg: 'bg-gray-500', text: 'text-white' };

    return (
      <span
        className={`inline-block px-2 py-1 rounded text-xs font-semibold uppercase ${badge.bg} ${badge.text}`}
      >
        {registry === 'Docker Hub' ? 'Docker Hub' : registry.split('.')[0]}
      </span>
    );
  };

  // Sort indicator
  const SortIndicator = ({ column }: { column: SortColumn }) => {
    if (sortColumn !== column) return <span className="text-gray-400 ml-1">⇅</span>;
    return <span className="ml-1">{sortOrder === 'asc' ? '▲' : '▼'}</span>;
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6">
      <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
        {title}
      </h2>

      {/* Search and Info */}
      <div className="mb-4 flex flex-col sm:flex-row gap-4 justify-between items-start sm:items-center">
        <input
          type="text"
          placeholder="Search images..."
          value={searchTerm}
          onChange={(e) => {
            setSearchTerm(e.target.value);
            setCurrentPage(0);
          }}
          className="w-full sm:w-96 px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg
                   focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-gray-700 dark:text-white"
        />
        <div className="text-sm text-gray-600 dark:text-gray-400">
          Showing {currentPage * pageSize + 1}-
          {Math.min((currentPage + 1) * pageSize, processedImages.length)} of{' '}
          {processedImages.length} images
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead className="bg-gradient-to-r from-purple-600 to-blue-600">
            <tr>
              <th
                onClick={() => handleSort('name')}
                className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider
                         cursor-pointer hover:bg-white/10 transition-colors"
              >
                Image Name <SortIndicator column="name" />
              </th>
              <th
                onClick={() => handleSort('count')}
                className="px-6 py-3 text-right text-xs font-medium text-white uppercase tracking-wider
                         cursor-pointer hover:bg-white/10 transition-colors"
              >
                Container Count <SortIndicator column="count" />
              </th>
              <th
                onClick={() => handleSort('registry')}
                className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider
                         cursor-pointer hover:bg-white/10 transition-colors"
              >
                Registry <SortIndicator column="registry" />
              </th>
              <th
                onClick={() => handleSort('installations')}
                className="px-6 py-3 text-right text-xs font-medium text-white uppercase tracking-wider
                         cursor-pointer hover:bg-white/10 transition-colors"
              >
                Installations <SortIndicator column="installations" />
              </th>
              <th
                onClick={() => handleSort('percentage')}
                className="px-6 py-3 text-right text-xs font-medium text-white uppercase tracking-wider
                         cursor-pointer hover:bg-white/10 transition-colors"
              >
                % of Total <SortIndicator column="percentage" />
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
            {paginatedImages.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                  No images found
                </td>
              </tr>
            ) : (
              paginatedImages.map((img, idx) => {
                const percentage = totalContainers > 0
                  ? ((img.count / totalContainers) * 100).toFixed(2)
                  : '0';

                return (
                  <tr
                    key={`${img.image}-${idx}`}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <code className="text-sm font-medium text-gray-900 dark:text-gray-100">
                        {img.image}
                      </code>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm text-gray-900 dark:text-gray-100">
                      {img.count.toLocaleString()}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      {getRegistryBadge(img.registry)}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm text-gray-900 dark:text-gray-100">
                      {img.installation_count}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm text-gray-900 dark:text-gray-100">
                      {percentage}%
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="mt-6 flex justify-center items-center gap-4">
          <button
            onClick={() => setCurrentPage((p) => Math.max(0, p - 1))}
            disabled={currentPage === 0}
            className="px-4 py-2 bg-gradient-to-r from-purple-600 to-blue-600 text-white rounded-lg
                     disabled:bg-gray-300 disabled:cursor-not-allowed hover:shadow-lg transition-all
                     disabled:from-gray-300 disabled:to-gray-300"
          >
            ← Previous
          </button>
          <span className="text-sm text-gray-600 dark:text-gray-400">
            Page {currentPage + 1} of {totalPages}
          </span>
          <button
            onClick={() => setCurrentPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={currentPage >= totalPages - 1}
            className="px-4 py-2 bg-gradient-to-r from-purple-600 to-blue-600 text-white rounded-lg
                     disabled:bg-gray-300 disabled:cursor-not-allowed hover:shadow-lg transition-all
                     disabled:from-gray-300 disabled:to-gray-300"
          >
            Next →
          </button>
        </div>
      )}
    </div>
  );
}

'use client';

import { useEffect, useState, useMemo, useRef } from 'react';
import {
  getVulnerabilitySummary,
  getVulnerabilityScans,
  getVulnerabilityDetails,
  scanImage,
  scanAllImages,
  updateVulnerabilityDb,
} from '@/lib/api';
import type { VulnerabilitySummary, VulnerabilityScan, Vulnerability } from '@/types';

// Chart.js type declaration
declare const Chart: {
  new (ctx: CanvasRenderingContext2D, config: unknown): {
    destroy: () => void;
    update: () => void;
  };
};

function SeverityBadge({ severity, count }: { severity: string; count: number }) {
  const colors: Record<string, string> = {
    critical: 'bg-[#ff1744] text-white',
    high: 'bg-[#ff9800] text-white',
    medium: 'bg-[#ffc107] text-black',
    low: 'bg-[#4caf50] text-white',
  };

  return (
    <span className={`px-2 py-0.5 text-xs rounded font-medium ${colors[severity] || 'bg-gray-500 text-white'}`}>
      {count}
    </span>
  );
}

function StatCard({ label, value, icon, color = 'text-[var(--text-primary)]' }: { label: string; value: number | string; icon: string; color?: string }) {
  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
      <div className="flex items-center gap-2 mb-2">
        <span>{icon}</span>
        <span className="text-sm text-[var(--text-tertiary)]">{label}</span>
      </div>
      <div className={`text-3xl font-bold ${color}`}>{value}</div>
    </div>
  );
}

interface VulnerabilityDetailsModalProps {
  isOpen: boolean;
  onClose: () => void;
  imageId: string;
  imageName: string;
}

function VulnerabilityDetailsModal({ isOpen, onClose, imageId, imageName }: VulnerabilityDetailsModalProps) {
  const [loading, setLoading] = useState(true);
  const [scan, setScan] = useState<VulnerabilityScan | null>(null);
  const [vulnerabilities, setVulnerabilities] = useState<Vulnerability[]>([]);
  const [filter, setFilter] = useState('');
  const [severityFilter, setSeverityFilter] = useState('');

  useEffect(() => {
    if (isOpen && imageId) {
      setLoading(true);
      getVulnerabilityDetails(imageId)
        .then(data => {
          setScan(data.scan);
          setVulnerabilities(data.vulnerabilities || []);
        })
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [isOpen, imageId]);

  const filteredVulns = useMemo(() => {
    return vulnerabilities.filter(v => {
      const matchesSearch = filter === '' ||
        v.vulnerability_id.toLowerCase().includes(filter.toLowerCase()) ||
        v.pkg_name.toLowerCase().includes(filter.toLowerCase()) ||
        (v.title || '').toLowerCase().includes(filter.toLowerCase());
      const matchesSeverity = severityFilter === '' || v.severity.toLowerCase() === severityFilter.toLowerCase();
      return matchesSearch && matchesSeverity;
    });
  }, [vulnerabilities, filter, severityFilter]);

  // Compute counts from vulnerabilities array (most accurate)
  const counts = useMemo(() => {
    const c = { critical: 0, high: 0, medium: 0, low: 0, total: 0 };
    vulnerabilities.forEach(v => {
      c.total++;
      const sev = v.severity.toLowerCase();
      if (sev === 'critical') c.critical++;
      else if (sev === 'high') c.high++;
      else if (sev === 'medium') c.medium++;
      else if (sev === 'low') c.low++;
    });
    return c;
  }, [vulnerabilities]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-4xl max-h-[80vh] flex flex-col">
        <div className="p-4 border-b border-[var(--border)] flex justify-between items-center">
          <div>
            <h2 className="text-xl font-bold">Vulnerability Details</h2>
            <div className="text-sm text-[var(--text-tertiary)]">{imageName}</div>
          </div>
          <button onClick={onClose} className="text-2xl hover:opacity-70">×</button>
        </div>

        {loading ? (
          <div className="flex-1 flex items-center justify-center p-8">
            <div className="text-[var(--text-tertiary)]">Loading...</div>
          </div>
        ) : (
          <>
            {/* Summary with severity badges */}
            <div className="p-4 border-b border-[var(--border)] flex flex-wrap gap-3 items-center">
              <div className="flex items-center gap-2">
                <span className="text-sm text-[var(--text-tertiary)]">Critical:</span>
                <SeverityBadge severity="critical" count={counts.critical} />
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-[var(--text-tertiary)]">High:</span>
                <SeverityBadge severity="high" count={counts.high} />
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-[var(--text-tertiary)]">Medium:</span>
                <SeverityBadge severity="medium" count={counts.medium} />
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-[var(--text-tertiary)]">Low:</span>
                <SeverityBadge severity="low" count={counts.low} />
              </div>
              <span className="text-sm text-[var(--text-tertiary)] ml-4">
                Total: <strong>{counts.total}</strong>
              </span>
            </div>

            {/* Filters */}
            <div className="p-4 border-b border-[var(--border)] flex gap-4">
              <input
                type="text"
                placeholder="Search CVE, package, title..."
                value={filter}
                onChange={e => setFilter(e.target.value)}
                className="flex-1 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm focus:outline-none focus:border-[var(--accent)]"
              />
              <select
                value={severityFilter}
                onChange={e => setSeverityFilter(e.target.value)}
                className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm focus:outline-none focus:border-[var(--accent)]"
              >
                <option value="">All Severities</option>
                <option value="critical">Critical</option>
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
              </select>
            </div>

            {/* Vulnerabilities List */}
            <div className="flex-1 overflow-auto p-4">
              {filteredVulns.length === 0 ? (
                <div className="text-center py-8 text-[var(--text-tertiary)]">
                  {vulnerabilities.length === 0 ? 'No vulnerabilities found' : 'No matching vulnerabilities'}
                </div>
              ) : (
                <div className="space-y-2">
                  {filteredVulns.map((v, idx) => (
                    <div key={idx} className="bg-[var(--bg-tertiary)] rounded p-3">
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-1">
                            <SeverityBadge severity={v.severity.toLowerCase()} count={1} />
                            <code className="text-sm font-medium">{v.vulnerability_id}</code>
                          </div>
                          <div className="text-sm mb-1">{v.title || 'No title'}</div>
                          <div className="text-xs text-[var(--text-tertiary)]">
                            Package: {v.pkg_name} ({v.installed_version})
                            {v.fixed_version && <span> → Fix: {v.fixed_version}</span>}
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default function SecurityPage() {
  const [summary, setSummary] = useState<VulnerabilitySummary | null>(null);
  const [scans, setScans] = useState<VulnerabilityScan[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [severityFilter, setSeverityFilter] = useState('');
  const [actionLoading, setActionLoading] = useState(false);
  const [detailsModal, setDetailsModal] = useState<{ imageId: string; imageName: string } | null>(null);

  const severityChartRef = useRef<HTMLCanvasElement>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const severityChartInstance = useRef<any>(null);

  const loadData = async () => {
    try {
      const [summaryData, scansData] = await Promise.all([
        getVulnerabilitySummary().catch(() => null),
        getVulnerabilityScans(1000).catch(() => []),
      ]);
      setSummary(summaryData);
      setScans(scansData);
    } catch (error) {
      console.error('Failed to load security data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();

    // Load Chart.js from CDN
    const script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/chart.js@4.4.0';
    script.async = true;
    document.body.appendChild(script);

    const interval = setInterval(loadData, 30000);
    return () => {
      clearInterval(interval);
      if (script.parentNode) script.parentNode.removeChild(script);
    };
  }, []);

  // Helper to get severity count from scan (handles both nested and flat formats)
  const getSeverityCount = (scan: VulnerabilityScan, severity: 'critical' | 'high' | 'medium' | 'low'): number => {
    // Try nested severity_counts first (current API format)
    if (scan.severity_counts && typeof scan.severity_counts === 'object') {
      return scan.severity_counts[severity] || 0;
    }
    // Fallback to flat fields (legacy format)
    const legacyField = `${severity}_count` as keyof VulnerabilityScan;
    return (scan[legacyField] as number) || 0;
  };

  // Compute stats from scans (most reliable)
  const stats = useMemo(() => {
    let critical = 0, high = 0, medium = 0, low = 0, atRisk = 0;
    scans.forEach(scan => {
      if (scan.success) {
        critical += getSeverityCount(scan, 'critical');
        high += getSeverityCount(scan, 'high');
        medium += getSeverityCount(scan, 'medium');
        low += getSeverityCount(scan, 'low');
        if (scan.total_vulnerabilities > 0) atRisk++;
      }
    });
    return {
      total: scans.filter(s => s.success).length,
      critical,
      high,
      medium,
      low,
      atRisk,
    };
  }, [scans]);

  // Render severity distribution chart
  useEffect(() => {
    if (loading || !severityChartRef.current || typeof Chart === 'undefined') return;

    if (severityChartInstance.current) {
      severityChartInstance.current.destroy();
    }

    const ctx = severityChartRef.current.getContext('2d');
    if (!ctx) return;

    severityChartInstance.current = new Chart(ctx, {
      type: 'doughnut',
      data: {
        labels: ['Critical', 'High', 'Medium', 'Low'],
        datasets: [{
          data: [stats.critical, stats.high, stats.medium, stats.low],
          backgroundColor: ['#ff1744', '#ff9800', '#ffc107', '#4caf50'],
          borderWidth: 0,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'bottom',
            labels: { color: '#94a3b8', padding: 15 },
          },
        },
      },
    });

    return () => {
      if (severityChartInstance.current) {
        severityChartInstance.current.destroy();
      }
    };
  }, [loading, stats]);

  const filteredScans = useMemo(() => {
    return scans.filter(scan => {
      const matchesSearch = searchTerm === '' ||
        scan.image_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        scan.image_id.toLowerCase().includes(searchTerm.toLowerCase());

      let matchesStatus = true;
      if (statusFilter === 'scanned') {
        matchesStatus = scan.success === true;
      } else if (statusFilter === 'remote') {
        matchesStatus = scan.success !== true && Boolean(scan.error?.includes('not available') || scan.error?.includes('remote'));
      } else if (statusFilter === 'failed') {
        matchesStatus = scan.success !== true && !scan.error?.includes('not available');
      }

      let matchesSeverity = true;
      if (severityFilter === 'critical') {
        matchesSeverity = getSeverityCount(scan, 'critical') > 0;
      } else if (severityFilter === 'high') {
        matchesSeverity = getSeverityCount(scan, 'high') > 0;
      } else if (severityFilter === 'medium') {
        matchesSeverity = getSeverityCount(scan, 'medium') > 0;
      } else if (severityFilter === 'low') {
        matchesSeverity = getSeverityCount(scan, 'low') > 0;
      } else if (severityFilter === 'clean') {
        matchesSeverity = scan.total_vulnerabilities === 0 && scan.success === true;
      }

      return matchesSearch && matchesStatus && matchesSeverity;
    });
  }, [scans, searchTerm, statusFilter, severityFilter]);

  const handleScanAll = async () => {
    setActionLoading(true);
    try {
      await scanAllImages();
      await loadData();
    } catch (error) {
      console.error('Failed to scan all images:', error);
    } finally {
      setActionLoading(false);
    }
  };

  const handleUpdateDb = async () => {
    setActionLoading(true);
    try {
      await updateVulnerabilityDb();
      await loadData();
    } catch (error) {
      console.error('Failed to update vulnerability database:', error);
    } finally {
      setActionLoading(false);
    }
  };

  const handleScanImage = async (imageId: string) => {
    try {
      await scanImage(imageId);
      await loadData();
    } catch (error) {
      console.error('Failed to scan image:', error);
    }
  };

  // Determine status badge for a scan
  const getStatusBadge = (scan: VulnerabilityScan) => {
    if (!scan.success) {
      const isRemote = scan.error?.includes('not available') || scan.error?.includes('remote');
      if (isRemote) {
        return <span className="px-2 py-1 text-xs rounded bg-[var(--info)] text-white">🌐 Remote</span>;
      }
      return <span className="px-2 py-1 text-xs rounded bg-[var(--danger)] text-white" title={scan.error}>⚠️ Failed</span>;
    }
    if (scan.total_vulnerabilities === 0) {
      return <span className="px-2 py-1 text-xs rounded bg-[var(--success)] text-white">✓ Clean</span>;
    }
    if (getSeverityCount(scan, 'critical') > 0) {
      return <span className="px-2 py-1 text-xs rounded bg-[#ff1744] text-white">🚨 Critical</span>;
    }
    if (getSeverityCount(scan, 'high') > 0) {
      return <span className="px-2 py-1 text-xs rounded bg-[#ff9800] text-white">⚠️ High</span>;
    }
    return <span className="px-2 py-1 text-xs rounded bg-[#ffc107] text-black">⚡ Vuln</span>;
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-[var(--text-tertiary)]">Loading...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">🛡️ Security</h1>
          <p className="text-sm text-[var(--text-tertiary)]">Monitor and track security vulnerabilities across all container images</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleScanAll}
            disabled={actionLoading}
            className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:opacity-80 transition-opacity disabled:opacity-50"
          >
            {actionLoading ? '...' : '🔍 Scan All'}
          </button>
          <button
            onClick={handleUpdateDb}
            disabled={actionLoading}
            className="px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
          >
            {actionLoading ? '...' : '⬇️ Update DB'}
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard icon="📊" label="Scanned Images" value={stats.total} />
        <StatCard icon="🚨" label="Critical" value={stats.critical} color={stats.critical > 0 ? 'text-[#ff1744]' : ''} />
        <StatCard icon="⚠️" label="High" value={stats.high} color={stats.high > 0 ? 'text-[#ff9800]' : ''} />
        <StatCard icon="🛡️" label="At Risk Images" value={stats.atRisk} color={stats.atRisk > 0 ? 'text-[var(--warning)]' : ''} />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Severity Distribution Chart */}
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
          <h3 className="text-lg font-medium mb-2">Severity Distribution</h3>
          <p className="text-xs text-[var(--text-tertiary)] mb-4">Current vulnerability breakdown</p>
          <div className="h-64">
            <canvas ref={severityChartRef}></canvas>
          </div>
        </div>

        {/* Severity Counts */}
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
          <h3 className="text-lg font-medium mb-2">Vulnerability Counts</h3>
          <p className="text-xs text-[var(--text-tertiary)] mb-4">Total vulnerabilities by severity</p>
          <div className="space-y-4">
            <div className="flex items-center justify-between p-3 bg-[var(--bg-tertiary)] rounded">
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-[#ff1744]"></span>
                <span>Critical</span>
              </div>
              <span className="text-xl font-bold text-[#ff1744]">{stats.critical}</span>
            </div>
            <div className="flex items-center justify-between p-3 bg-[var(--bg-tertiary)] rounded">
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-[#ff9800]"></span>
                <span>High</span>
              </div>
              <span className="text-xl font-bold text-[#ff9800]">{stats.high}</span>
            </div>
            <div className="flex items-center justify-between p-3 bg-[var(--bg-tertiary)] rounded">
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-[#ffc107]"></span>
                <span>Medium</span>
              </div>
              <span className="text-xl font-bold text-[#ffc107]">{stats.medium}</span>
            </div>
            <div className="flex items-center justify-between p-3 bg-[var(--bg-tertiary)] rounded">
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-[#4caf50]"></span>
                <span>Low</span>
              </div>
              <span className="text-xl font-bold text-[#4caf50]">{stats.low}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Queue Status */}
      {summary?.queue_status && (summary.queue_status.in_progress > 0 || summary.queue_status.pending > 0) && (
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4 flex items-center gap-4">
          <span className="text-xl">⏳</span>
          <span className="text-sm">
            <strong>Scanning in progress:</strong> {summary.queue_status.in_progress} scanning, {summary.queue_status.pending} queued
          </span>
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <input
          type="text"
          placeholder="🔍 Search images..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="flex-1 min-w-[200px] bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg px-4 py-2 text-[var(--text-primary)] placeholder-[var(--text-tertiary)] focus:outline-none focus:border-[var(--accent)]"
        />
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg px-4 py-2 text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">All Status</option>
          <option value="scanned">Scanned Only</option>
          <option value="remote">Remote Only</option>
          <option value="failed">Failed Only</option>
        </select>
        <select
          value={severityFilter}
          onChange={(e) => setSeverityFilter(e.target.value)}
          className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg px-4 py-2 text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">All Severities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
          <option value="clean">Clean</option>
        </select>
      </div>

      {/* Scans Table */}
      {filteredScans.length === 0 ? (
        <div className="text-center py-12 text-[var(--text-tertiary)]">
          No vulnerability scans found
        </div>
      ) : (
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden">
          <div className="px-4 py-3 border-b border-[var(--border)] flex items-center justify-between">
            <h3 className="font-medium">Vulnerability Scans</h3>
            <span className="text-sm text-[var(--text-tertiary)]">{filteredScans.length} scans</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-[var(--bg-tertiary)]">
                  <th className="text-left px-4 py-3 text-sm font-medium">Image</th>
                  <th className="text-left px-4 py-3 text-sm font-medium">Status</th>
                  <th className="text-center px-4 py-3 text-sm font-medium">Critical</th>
                  <th className="text-center px-4 py-3 text-sm font-medium">High</th>
                  <th className="text-center px-4 py-3 text-sm font-medium">Medium</th>
                  <th className="text-center px-4 py-3 text-sm font-medium">Low</th>
                  <th className="text-center px-4 py-3 text-sm font-medium">Total</th>
                  <th className="text-left px-4 py-3 text-sm font-medium">Last Scan</th>
                  <th className="text-left px-4 py-3 text-sm font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredScans.map(scan => (
                  <tr
                    key={scan.image_id}
                    className="border-t border-[var(--border)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
                    onClick={() => setDetailsModal({ imageId: scan.image_id, imageName: scan.image_name })}
                  >
                    <td className="px-4 py-3">
                      <code className="text-sm">{scan.image_name}</code>
                    </td>
                    <td className="px-4 py-3">
                      {getStatusBadge(scan)}
                    </td>
                    <td className="px-4 py-3 text-center">
                      <SeverityBadge severity="critical" count={getSeverityCount(scan, 'critical')} />
                    </td>
                    <td className="px-4 py-3 text-center">
                      <SeverityBadge severity="high" count={getSeverityCount(scan, 'high')} />
                    </td>
                    <td className="px-4 py-3 text-center">
                      <SeverityBadge severity="medium" count={getSeverityCount(scan, 'medium')} />
                    </td>
                    <td className="px-4 py-3 text-center">
                      <SeverityBadge severity="low" count={getSeverityCount(scan, 'low')} />
                    </td>
                    <td className="px-4 py-3 text-center font-medium">{scan.total_vulnerabilities}</td>
                    <td className="px-4 py-3 text-sm text-[var(--text-tertiary)]">
                      {new Date(scan.scanned_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-3">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleScanImage(scan.image_id);
                        }}
                        className="px-2 py-1 text-sm rounded hover:bg-[var(--bg-secondary)] transition-colors"
                        title="Rescan"
                      >
                        🔄
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Details Modal */}
      {detailsModal && (
        <VulnerabilityDetailsModal
          isOpen={!!detailsModal}
          onClose={() => setDetailsModal(null)}
          imageId={detailsModal.imageId}
          imageName={detailsModal.imageName}
        />
      )}
    </div>
  );
}

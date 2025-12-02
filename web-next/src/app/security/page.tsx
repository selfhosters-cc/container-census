'use client';

import { useEffect, useState, useMemo } from 'react';
import {
  getVulnerabilitySummary,
  getVulnerabilityScans,
  getVulnerabilityDetails,
  scanImage,
  scanAllImages,
  updateVulnerabilityDb,
} from '@/lib/api';
import type { VulnerabilitySummary, VulnerabilityScan, Vulnerability } from '@/types';

function SeverityBadge({ severity, count }: { severity: string; count: number }) {
  const colors: Record<string, string> = {
    critical: 'bg-[#ff1744] text-white',
    high: 'bg-[#ff9800] text-white',
    medium: 'bg-[#ffc107] text-black',
    low: 'bg-[#4caf50] text-white',
  };

  return (
    <span className={`px-2 py-0.5 text-xs rounded ${colors[severity] || 'bg-gray-500 text-white'}`}>
      {count} {severity.toUpperCase()}
    </span>
  );
}

function StatCard({ label, value, color = 'text-[var(--text-primary)]' }: { label: string; value: number | string; color?: string }) {
  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
      <div className="text-sm text-[var(--text-tertiary)]">{label}</div>
      <div className={`text-2xl font-bold ${color}`}>{value}</div>
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
            {/* Summary */}
            {scan && (
              <div className="p-4 border-b border-[var(--border)] flex flex-wrap gap-2">
                <SeverityBadge severity="critical" count={scan.critical_count} />
                <SeverityBadge severity="high" count={scan.high_count} />
                <SeverityBadge severity="medium" count={scan.medium_count} />
                <SeverityBadge severity="low" count={scan.low_count} />
                <span className="text-sm text-[var(--text-tertiary)] ml-4">
                  Total: {scan.total_vulnerabilities}
                </span>
              </div>
            )}

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
  const [actionLoading, setActionLoading] = useState(false);
  const [detailsModal, setDetailsModal] = useState<{ imageId: string; imageName: string } | null>(null);

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
    const interval = setInterval(loadData, 30000);
    return () => clearInterval(interval);
  }, []);

  const filteredScans = useMemo(() => {
    return scans.filter(scan => {
      const matchesSearch = searchTerm === '' ||
        scan.image_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        scan.image_id.toLowerCase().includes(searchTerm.toLowerCase());

      let matchesStatus = true;
      if (statusFilter === 'critical') {
        matchesStatus = scan.critical_count > 0;
      } else if (statusFilter === 'high') {
        matchesStatus = scan.high_count > 0;
      } else if (statusFilter === 'clean') {
        matchesStatus = scan.total_vulnerabilities === 0;
      } else if (statusFilter === 'failed') {
        matchesStatus = !scan.success;
      }

      return matchesSearch && matchesStatus;
    });
  }, [scans, searchTerm, statusFilter]);

  const stats = useMemo(() => {
    // Handle both nested (summary.summary) and direct formats
    const s = summary?.summary || summary;
    const severityCounts = (s as { severity_counts?: Record<string, number> })?.severity_counts || {};
    const totalScanned = (s as { total_images_scanned?: number })?.total_images_scanned || scans.length;
    const atRiskCount = (s as { images_with_vulnerabilities?: number })?.images_with_vulnerabilities || scans.filter(sc => sc.total_vulnerabilities > 0).length;
    return {
      total: totalScanned,
      critical: severityCounts.critical || 0,
      high: severityCounts.high || 0,
      atRisk: atRiskCount,
    };
  }, [summary, scans]);

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
        <h1 className="text-2xl font-bold">Security</h1>
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
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Scanned Images" value={stats.total} />
        <StatCard label="Critical" value={stats.critical} color={stats.critical > 0 ? 'text-[#ff1744]' : ''} />
        <StatCard label="High" value={stats.high} color={stats.high > 0 ? 'text-[#ff9800]' : ''} />
        <StatCard label="At Risk Images" value={stats.atRisk} color={stats.atRisk > 0 ? 'text-[var(--warning)]' : ''} />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <input
          type="text"
          placeholder="Search images..."
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
          <option value="critical">Critical Issues</option>
          <option value="high">High Issues</option>
          <option value="clean">Clean</option>
          <option value="failed">Failed Scans</option>
        </select>
      </div>

      {/* Scans Table */}
      {filteredScans.length === 0 ? (
        <div className="text-center py-12 text-[var(--text-tertiary)]">
          No vulnerability scans found
        </div>
      ) : (
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="bg-[var(--bg-tertiary)]">
                <th className="text-left px-4 py-3 text-sm font-medium">Image</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Status</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Critical</th>
                <th className="text-left px-4 py-3 text-sm font-medium">High</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Medium</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Low</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Total</th>
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
                    {scan.success ? (
                      <span className="px-2 py-1 text-xs rounded bg-[var(--success)] text-white">✓</span>
                    ) : (
                      <span className="px-2 py-1 text-xs rounded bg-[var(--danger)] text-white" title={scan.error}>✗</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={scan.critical_count > 0 ? 'text-[#ff1744] font-bold' : ''}>
                      {scan.critical_count}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={scan.high_count > 0 ? 'text-[#ff9800] font-bold' : ''}>
                      {scan.high_count}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={scan.medium_count > 0 ? 'text-[#ffc107]' : ''}>
                      {scan.medium_count}
                    </span>
                  </td>
                  <td className="px-4 py-3">{scan.low_count}</td>
                  <td className="px-4 py-3">{scan.total_vulnerabilities}</td>
                  <td className="px-4 py-3 text-sm text-[var(--text-tertiary)]">
                    {new Date(scan.scanned_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleScanImage(scan.image_id);
                      }}
                      className="p-1.5 rounded hover:bg-[var(--bg-secondary)] transition-colors"
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

'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { getPluginTabs } from '@/lib/api';
import type { PluginTab } from '@/types';

export default function PluginPageClient() {
  const params = useParams();
  const pluginId = params?.pluginId as string;
  const [tab, setTab] = useState<PluginTab | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadPluginTab() {
      try {
        const tabs = await getPluginTabs();
        const matchingTab = tabs.find(t => t.id === pluginId);

        if (!matchingTab) {
          setError(`Plugin tab "${pluginId}" not found`);
          setLoading(false);
          return;
        }

        setTab(matchingTab);
        setLoading(false);

        // Map tab IDs to plugin IDs (since external plugins may have different IDs than their tabs)
        const tabToPluginMap: Record<string, string> = {
          'graph': 'graph-visualizer',
          'graph-visualizer': 'graph-visualizer',
        };

        const actualPluginId = tabToPluginMap[pluginId] || pluginId;

        // Load the plugin's JavaScript bundle with cache busting
        const script = document.createElement('script');
        script.src = `/api/p/${actualPluginId}/bundle.js?v=${Date.now()}`;
        script.async = true;
        script.onload = () => {
          console.log('[PluginPage] Plugin script loaded successfully');

          // Wait for DOM to be ready and get the container element
          const initPlugin = () => {
            const container = document.getElementById('plugin-container');
            if (!container) {
              console.error('[PluginPage] Plugin container not found');
              setError('Plugin container not found');
              return;
            }

            // Create a simple SDK for the plugin
            const sdk = {
              fetch: async (path: string, options?: RequestInit) => {
                // Proxy fetch calls through the plugin API (uses /api/p/ prefix)
                const url = `/api/p/${actualPluginId}${path}`;
                return fetch(url, options);
              },
              showToast: (message: string, type: string) => {
                console.log(`[Plugin Toast] ${type}: ${message}`);
                // TODO: Integrate with Census toast system
              }
            };

            // Call the plugin's init function
            const initFn = (window as any).initGraphVisualizer;
            if (typeof initFn === 'function') {
              console.log('[PluginPage] Calling plugin init function');
              initFn(container, sdk);
            } else {
              console.error('[PluginPage] Plugin init function not found');
              setError('Plugin initialization function not found');
            }
          };

          // Give React time to render the DOM
          setTimeout(initPlugin, 100);
        };
        script.onerror = () => {
          console.error('[PluginPage] Failed to load plugin script');
          setError('Failed to load plugin assets');
        };
        document.body.appendChild(script);

        return () => {
          // Clean up script tag on unmount
          if (document.body.contains(script)) {
            document.body.removeChild(script);
          }
        };
      } catch (err) {
        console.error('[PluginPage] Error loading plugin:', err);
        setError(err instanceof Error ? err.message : 'Unknown error');
        setLoading(false);
      }
    }

    if (pluginId) {
      loadPluginTab();
    }
  }, [pluginId]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-[var(--text-tertiary)]">Loading plugin...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-64">
        <div className="text-[var(--danger)] mb-4">{error}</div>
        <a href="/integrations" className="text-[var(--accent)] hover:underline">
          ← Back to Integrations
        </a>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <span className="text-2xl">{tab?.icon}</span>
        <h1 className="text-2xl font-bold">{tab?.label}</h1>
      </div>

      {/* Container where the plugin will render */}
      <div id="plugin-container" className="min-h-screen"></div>
    </div>
  );
}

import PluginPageClient from './PluginPageClient';

// Generate static params for known plugins at build time
export async function generateStaticParams() {
  // For static export, we need to pre-define known plugin routes
  // External plugins installed at runtime won't have pages pre-generated
  return [
    { pluginId: 'graph' },
    { pluginId: 'graph-visualizer' },
  ];
}

export default function PluginPage() {
  return <PluginPageClient />;
}

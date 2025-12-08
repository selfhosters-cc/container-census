'use client';

interface ViewToggleProps {
  view: 'cards' | 'table';
  onChange: (view: 'cards' | 'table') => void;
}

export default function ViewToggle({ view, onChange }: ViewToggleProps) {
  return (
    <div className="flex gap-1 bg-[var(--bg-secondary)] rounded-lg p-1 border border-[var(--border)]">
      <button
        onClick={() => onChange('cards')}
        className={`px-4 py-2 rounded-md transition-colors text-sm font-medium ${
          view === 'cards'
            ? 'bg-blue-600 text-white'
            : 'text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
        }`}
      >
        📊 Cards
      </button>
      <button
        onClick={() => onChange('table')}
        className={`px-4 py-2 rounded-md transition-colors text-sm font-medium ${
          view === 'table'
            ? 'bg-blue-600 text-white'
            : 'text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
        }`}
      >
        📋 Table
      </button>
    </div>
  );
}

import { useState } from 'react';
import { Activity } from 'lucide-react';
import DecisionTimeline from './components/DecisionTimeline';
import StatsView from './components/StatsView';
import SearchView from './components/SearchView';
import type { Decision } from './types';

type TabType = 'timeline' | 'stats' | 'search';

function App() {
  const [activeTab, setActiveTab] = useState<TabType>('timeline');
  const [selectedDecision, setSelectedDecision] = useState<Decision | null>(null);

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100">
      {/* Header */}
      <header className="bg-slate-800 border-b border-slate-700 shadow-lg">
        <div className="container mx-auto px-4 py-6">
          <div className="flex items-center gap-3">
            <Activity className="w-8 h-8 text-blue-400" />
            <h1 className="text-3xl font-bold text-slate-100">
              CryptoFunk Explainability Dashboard
            </h1>
          </div>
          <p className="text-slate-400 mt-2">
            Multi-Agent AI Trading System Decision Analysis
          </p>
        </div>
      </header>

      {/* Navigation Tabs */}
      <nav className="bg-slate-800 border-b border-slate-700">
        <div className="container mx-auto px-4">
          <div className="flex gap-1">
            <button
              onClick={() => setActiveTab('timeline')}
              className={`px-6 py-3 font-medium transition-colors ${
                activeTab === 'timeline'
                  ? 'text-blue-400 border-b-2 border-blue-400'
                  : 'text-slate-400 hover:text-slate-300'
              }`}
            >
              Timeline
            </button>
            <button
              onClick={() => setActiveTab('stats')}
              className={`px-6 py-3 font-medium transition-colors ${
                activeTab === 'stats'
                  ? 'text-blue-400 border-b-2 border-blue-400'
                  : 'text-slate-400 hover:text-slate-300'
              }`}
            >
              Statistics
            </button>
            <button
              onClick={() => setActiveTab('search')}
              className={`px-6 py-3 font-medium transition-colors ${
                activeTab === 'search'
                  ? 'text-blue-400 border-b-2 border-blue-400'
                  : 'text-slate-400 hover:text-slate-300'
              }`}
            >
              Search
            </button>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-6">
        {activeTab === 'timeline' && (
          <DecisionTimeline
            selectedDecision={selectedDecision}
            onSelectDecision={setSelectedDecision}
          />
        )}
        {activeTab === 'stats' && <StatsView />}
        {activeTab === 'search' && (
          <SearchView onSelectDecision={setSelectedDecision} />
        )}
      </main>
    </div>
  );
}

export default App;

import { useState } from 'react'
import { useGame } from './hooks/useGame'
import { TopBar } from './components/TopBar'
import { ErrorBanner } from './components/ErrorBanner'
import { OfficeSelect } from './components/OfficeSelect'
import { GameOverPanel } from './components/GameOverPanel'
import { OverviewPanel } from './components/OverviewPanel'
import { PackagesPanel } from './components/PackagesPanel'
import { EmployeesPanel } from './components/EmployeesPanel'
import { DeliveriesPanel } from './components/DeliveriesPanel'
import { FinancePanel } from './components/FinancePanel'
import './App.css'

type Tab = 'overview' | 'packages' | 'employees' | 'deliveries' | 'finance'

const TABS: { id: Tab; label: string }[] = [
  { id: 'overview', label: 'Flow' },
  { id: 'packages', label: 'Packages' },
  { id: 'employees', label: 'Employees' },
  { id: 'deliveries', label: 'Deliveries' },
  { id: 'finance', label: 'Finance' },
]

function App() {
  const game = useGame()
  const [tab, setTab] = useState<Tab>('overview')

  if (!game.loaded && !game.error) {
    return (
      <div className="app-loading">
        <p>Connecting to Delivery Office backend…</p>
      </div>
    )
  }

  if (!game.loaded && game.error) {
    return (
      <div className="app-loading">
        <p>Cannot reach the backend.</p>
        <p className="muted">{game.error.err.message}</p>
        <button type="button" className="btn btn-primary" onClick={() => void game.refresh()}>
          Retry
        </button>
        <ErrorBanner entry={game.error} onDismiss={game.dismissError} />
      </div>
    )
  }

  const office = game.state?.office ?? null

  return (
    <div className="app">
      <TopBar game={game} />

      {game.gameOver ? (
        <main className="app-main">
          <GameOverPanel game={game} />
        </main>
      ) : !office ? (
        <main className="app-main">
          <OfficeSelect game={game} />
        </main>
      ) : (
        <>
          <nav className="tabs" role="tablist">
            {TABS.map((item) => (
              <button
                key={item.id}
                type="button"
                role="tab"
                aria-selected={tab === item.id}
                className={`tab ${tab === item.id ? 'is-active' : ''}`}
                onClick={() => setTab(item.id)}
              >
                {item.label}
              </button>
            ))}
          </nav>
          <main className="app-main">
            {tab === 'overview' && <OverviewPanel game={game} />}
            {tab === 'packages' && <PackagesPanel game={game} />}
            {tab === 'employees' && <EmployeesPanel game={game} />}
            {tab === 'deliveries' && <DeliveriesPanel game={game} />}
            {tab === 'finance' && <FinancePanel game={game} />}
          </main>
        </>
      )}

      {game.error && <ErrorBanner entry={game.error} onDismiss={game.dismissError} />}
    </div>
  )
}

export default App

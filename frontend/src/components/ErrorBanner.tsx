import type { ErrorEntry } from '../hooks/useGame'

export function ErrorBanner({ entry, onDismiss }: { entry: ErrorEntry; onDismiss: () => void }) {
  const { err } = entry
  return (
    <div className="error-banner" role="alert">
      <div className="error-banner-body">
        <span className="error-code">{err.code}</span>
        <span className="error-message">{err.message}</span>
        {err.details !== undefined && (
          <details className="error-details">
            <summary>Details</summary>
            <pre>{JSON.stringify(err.details, null, 2)}</pre>
          </details>
        )}
      </div>
      <button type="button" className="error-dismiss" onClick={onDismiss} aria-label="Dismiss error">
        ×
      </button>
    </div>
  )
}

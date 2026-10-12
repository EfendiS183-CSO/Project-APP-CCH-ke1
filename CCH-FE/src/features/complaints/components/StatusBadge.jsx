import { statusMeta } from '../format.js'

export function StatusBadge({ code, label, size = 'md' }) {
  const meta = statusMeta(code)
  return (
    <span className={`status-badge status-${meta.tone} status-${size}`}>
      <span className="status-badge-dot" aria-hidden="true" />
      {label || meta.label}
    </span>
  )
}

export function StatusDot({ tone }) {
  return <span className={`status-dot status-dot-${tone}`} aria-hidden="true" />
}
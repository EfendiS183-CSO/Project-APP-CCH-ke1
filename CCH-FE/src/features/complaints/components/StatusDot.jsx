import { statusMeta } from '../format.js'

export function StatusDot({ code }) {
  const meta = statusMeta(code)
  return <span className={`status-dot status-dot-${meta.tone}`} aria-hidden="true" />
}
import { formatCount } from '../format.js'

export function MetricCard({ label, value, hint, tone = 'neutral', icon: Icon }) {
  return (
    <article className={`metric-card metric-${tone}`}>
      {Icon ? (
        <span className="metric-icon" aria-hidden="true">
          <Icon size={18} strokeWidth={2} />
        </span>
      ) : null}
      <div className="metric-body">
        <span className="metric-label">{label}</span>
        <strong className="metric-value">{formatCount(value)}</strong>
        {hint ? <span className="metric-hint">{hint}</span> : null}
      </div>
    </article>
  )
}
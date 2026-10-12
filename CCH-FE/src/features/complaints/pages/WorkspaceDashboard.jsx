import { useContext } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, ArrowRight, CalendarCheck, CheckCircle2, Inbox, Loader2, PackageCheck, Send, Ticket } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { complaintApi } from '../api/complaintApi.js'
import { MetricCard } from '../components/MetricCard.jsx'
import { Section } from '../components/Section.jsx'
import { ComplaintTable } from '../components/ComplaintTable.jsx'
import { EmptyState } from '../components/EmptyState.jsx'
import { useWorkspaceApi } from '../useWorkspaceApi.js'

function renderFetch({ loading, error, empty, children }) {
  if (loading) {
    return (
      <div className="queue-state">
        <Loader2 size={20} className="spin" />
        <span>Memuat…</span>
      </div>
    )
  }
  if (error) {
    return (
      <div className="alert alert-error" role="alert">
        <AlertCircle size={16} />
        <span>{error}</span>
      </div>
    )
  }
  if (empty) return empty
  return children
}

function useDashboard(token) {
  const { data, loading, error, refresh } = useWorkspaceApi(() => {
    if (!token) return Promise.resolve({ counts: null, open: [] })
    return Promise.all([
      complaintApi.countOpen(token),
      complaintApi.countReceivedOpen(token),
      complaintApi.countSentOpen(token),
      complaintApi.countCompleted(token),
      complaintApi.listOpen(token, { limit: 5 }),
    ]).then(([openCount, receivedCount, sentCount, completedCount, openList]) => ({
      counts: { open: openCount, received: receivedCount, sent: sentCount, completed: completedCount },
      open: openList,
    }))
  })

  return {
    counts: data?.counts || null,
    open: data?.open || [],
    loading,
    error,
    reload: refresh,
  }
}

export function WorkspaceDashboard() {
  const { token } = useContext(AuthContext)
  const { counts, open, loading, error, reload } = useDashboard(token)

  return (
    <div className="page-wide workspace-dashboard">
      <div className="page-heading workspace-heading">
        <div>
          <p className="eyebrow">Call Center Workspace</p>
          <h1>Ringkasan pengaduan</h1>
          <p className="page-heading-sub">
            Pantau tiket terbuka, diterima, dikirim, dan selesai sesuai kebijakan kantor Anda.
          </p>
        </div>
        <button type="button" className="btn btn-secondary" onClick={() => void reload()} disabled={loading}>
          <RefreshIcon spinning={loading} />
          Muat ulang
        </button>
      </div>

      {error ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      ) : null}

      <div className="metric-grid">
        <MetricCard label="Tiket terbuka" value={counts?.open} icon={Ticket} tone="brand" />
        <MetricCard label="Diterima" value={counts?.received} icon={Inbox} tone="sky" />
        <MetricCard label="Dikirim" value={counts?.sent} icon={Send} tone="indigo" />
        <MetricCard label="Selesai" value={counts?.completed} icon={CheckCircle2} tone="green" />
      </div>

      <Section
        eyebrow="Perlu perhatian"
        title="Tiket terbuka terbaru"
        actions={
          <Link to="/workspace/complaints/open" className="btn btn-secondary btn-small">
            Lihat semua <ArrowRight size={14} />
          </Link>
        }
      >
        {renderFetch({
          loading,
          error: '',
          empty: (
            <EmptyState
              icon={Inbox}
              title="Tidak ada tiket terbuka"
              description="Tiket baru akan muncul di sini sesuai scope kantor Anda."
            />
          ),
          children: <ComplaintTable items={open} variant="open" title="terbuka" />,
        })}
      </Section>

      <Section
        eyebrow="Kegiatan"
        title="Akses cepat"
      >
        <div className="quick-grid">
          <QuickLink to="/workspace/complaints/received-open" icon={Inbox} label="Diterima terbuka" />
          <QuickLink to="/workspace/complaints/sent-open" icon={Send} label="Dikirim terbuka" />
          <QuickLink to="/workspace/complaints/completed" icon={CheckCircle2} label="Selesai" />
          <QuickLink to="/workspace/follow-ups" icon={CalendarCheck} label="Follow-up" />
        </div>
      </Section>
    </div>
  )
}

function RefreshIcon({ spinning }) {
  return spinning ? <Loader2 size={15} className="spin" /> : <PackageCheck size={15} />
}

function QuickLink({ to, icon: Icon, label }) {
  return (
    <Link to={to} className="quick-link">
      <span className="quick-icon" aria-hidden="true"><Icon size={18} /></span>
      <span>{label}</span>
      <ArrowRight size={14} className="quick-arrow" />
    </Link>
  )
}
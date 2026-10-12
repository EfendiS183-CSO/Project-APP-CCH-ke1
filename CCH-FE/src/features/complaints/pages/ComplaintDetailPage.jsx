import { useContext, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { AuthContext } from '../../auth/auth-context.js'
import { complaintApi } from '../api/complaintApi.js'
import { followUpsApi } from '../../followups/api/followUpsApi.js'
import { ComplaintInfoPanel } from '../components/ComplaintInfoPanel.jsx'
import { TimelinePanel } from '../components/TimelinePanel.jsx'
import { FollowUpPanel } from '../components/FollowUpPanel.jsx'
import { DocumentsPanel } from '../components/DocumentsPanel.jsx'
import { StatusActionsPanel } from '../components/StatusActionsPanel.jsx'
import { DetailError, DetailLoading } from '../components/DetailStates.jsx'
import { documentApi } from '../../documents/api/documentApi.js'
import { useWorkspaceApi } from '../useWorkspaceApi.js'

export function ComplaintDetailPage({ completed = false }) {
  const { id } = useParams()
  const { token } = useContext(AuthContext)

  const [reloadKey, setReloadKey] = useState(0)
  const reload = () => setReloadKey((key) => key + 1)

  const detail = useWorkspaceApi(() => {
    if (!id || !token) return Promise.resolve(null)
    const getter = completed ? complaintApi.getCompleted : complaintApi.getOpen
    return getter(token, id)
  }, [id, token, completed])

  const history = useWorkspaceApi(() => {
    if (!id || !token) return Promise.resolve([])
    const historyGetter = completed ? complaintApi.listCompletedHistory : complaintApi.listOpenHistory
    return historyGetter(token, id, { limit: 100 })
  }, [id, token, reloadKey, completed])

  const followUps = useWorkspaceApi(() => {
    if (!id || !token) return Promise.resolve([])
    return followUpsApi.listForTicket(token, id)
  }, [id, token, reloadKey])

  const documents = useWorkspaceApi(() => {
    if (!id || !token) return Promise.resolve([])
    return documentApi.listForTicket(token, id)
  }, [id, token, reloadKey])

  if (detail.loading) return <DetailLoading />
  if (detail.error || !detail.data) return <DetailError message={detail.error} />

  const data = detail.data

  return (
    <div className="page-wide complaint-detail-page">
      <Link to={completed ? '/workspace/complaints/completed' : '/workspace/complaints/open'} className="back-link">
        <ArrowLeft size={14} />
        {completed ? 'Kembali ke tiket selesai' : 'Kembali ke tiket terbuka'}
      </Link>

      <div className="page-heading">
        <div>
          <p className="eyebrow">Call Center Workspace</p>
          <h1>{data.id}</h1>
          <p className="page-heading-sub">
            {data.summary || 'Detail tiket pengaduan'}
          </p>
        </div>
      </div>

      <div className="detail-layout">
        <div className="detail-main">
          <ComplaintInfoPanel detail={data} />
          <TimelinePanel
            items={history.data || []}
            loading={history.loading}
            error={history.error}
            reload={reload}
          />
        </div>
        <aside className="detail-side">
          {!completed ? (
            <StatusActionsPanel
              complaintId={data.id}
              statusCode={data.status_code}
              onTransitioned={reload}
            />
          ) : null}
          <FollowUpPanel
            items={followUps.data || []}
            loading={followUps.loading}
            error={followUps.error}
            reload={reload}
          />
          <DocumentsPanel items={documents.data || []} loading={documents.loading} error={documents.error} />
        </aside>
      </div>
    </div>
  )
}
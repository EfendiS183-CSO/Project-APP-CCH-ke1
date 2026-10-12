import { FileText, Loader2 } from 'lucide-react'
import { EmptyState } from './EmptyState.jsx'
import { formatDateTime } from '../format.js'

export function DocumentsPanel({ items, loading, error }) {
  return (
    <article className="panel documents-panel">
      <div className="panel-head">
        <div>
          <p className="section-eyebrow">Arsip dokumen</p>
          <h3>Dokumen tiket</h3>
        </div>
      </div>

      {loading ? (
        <div className="queue-state"><Loader2 size={20} className="spin" /><span>Memuat dokumen…</span></div>
      ) : error ? (
        <div className="alert alert-error" role="alert"><span>{error}</span></div>
      ) : !items || items.length === 0 ? (
        <EmptyState
          icon={FileText}
          title="Tidak ada dokumen"
          description="Metadata dokumen terpasang pada riwayat akan tampil di sini."
        />
      ) : (
        <ul className="document-list">
          {items.map((document) => (
            <li key={`${document.history_id}-${document.document_type_code}`} className="document-item">
              <span className="document-icon" aria-hidden="true"><FileText size={16} /></span>
              <div className="document-body">
                <span className="document-type">{document.document_type_code || document.document_type}</span>
                <span className="document-meta">
                  {document.office_code ? `Kantor ${document.office_code} · ` : ''}
                  {document.added_at ? formatDateTime(document.added_at) : ''}
                  {document.file_count ? ` · ${document.file_count} berkas` : ''}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}
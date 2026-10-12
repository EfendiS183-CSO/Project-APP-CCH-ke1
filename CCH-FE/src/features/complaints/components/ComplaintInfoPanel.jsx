import { Building2, Mail, MapPin, Package, Phone, User } from 'lucide-react'
import { StatusBadge } from './StatusBadge.jsx'
import { formatDateTime } from '../format.js'

function Field({ icon: Icon, label, value, mono = false }) {
  return (
    <div className="detail-field">
      {Icon ? (
        <span className="detail-field-icon" aria-hidden="true">
          <Icon size={15} />
        </span>
      ) : null}
      <div className="detail-field-text">
        <span className="detail-field-label">{label}</span>
        <span className={`detail-field-value${mono ? ' mono' : ''}`}>{value || '—'}</span>
      </div>
    </div>
  )
}

export function ComplaintInfoPanel({ detail }) {
  if (!detail) return null

  return (
    <article className="panel info-panel">
      <div className="info-panel-head">
        <div>
          <p className="section-eyebrow">Informasi tiket</p>
          <h3>{detail.id}</h3>
        </div>
        <StatusBadge code={detail.status_code} label={detail.status_description} />
      </div>

      <div className="detail-grid">
        <Field icon={User} label="Pelanggan" value={detail.customer_name} />
        <Field icon={Phone} label="Telepon" value={detail.customer_phone} mono />
        <Field icon={Mail} label="Email" value={detail.customer_email} />
        <Field icon={MapPin} label="Alamat" value={detail.customer_address} />
        <Field icon={Building2} label="Kantor asal" value={detail.origin_office_code} mono />
        <Field icon={Building2} label="Kantor tujuan" value={detail.destination_office_code} mono />
        <Field icon={Building2} label="Tembusan" value={detail.copy_office_codes} mono />
        <Field icon={Package} label="Kiriman" value={detail.shipment_number} mono />
      </div>

      <div className="detail-summary">
        <p className="section-eyebrow">Ringkasan</p>
        <p>{detail.summary || 'Tidak ada ringkasan.'}</p>
        {detail.description ? <p className="detail-description">{detail.description}</p> : null}
      </div>

      <div className="detail-footer">
        <span>Status sejak {formatDateTime(detail.status_date)}</span>
        <span>Update kantor {detail.status_office_code || '—'}</span>
      </div>
    </article>
  )
}
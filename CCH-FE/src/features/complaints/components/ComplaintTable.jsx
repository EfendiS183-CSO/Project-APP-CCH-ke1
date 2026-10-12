import { Link } from 'react-router-dom'
import { ArrowRight, CalendarClock, Building2 } from 'lucide-react'
import { StatusBadge } from './StatusBadge.jsx'
import { formatDateTime } from '../format.js'

const OPEN_PATH = '/workspace/complaints/open'
const CLOSED_PATH = '/workspace/complaints/completed'

function complaintTarget(item, variant) {
  if (variant === 'open') return `${OPEN_PATH}/${item.id}`
  if (variant === 'closed') return `${CLOSED_PATH}/${item.id}`
  if (variant === 'received') return '/workspace/complaints/received-open/'
  if (variant === 'sent') return '/workspace/complaints/sent-open/'
  return `${OPEN_PATH}/${item.id}`
}

export function ComplaintTable({ items, variant, title }) {
  if (!items || items.length === 0) {
    return (
      <div className="table-empty">
        <p>Tidak ada tiket {title || 'untuk tampilan ini'}.</p>
      </div>
    )
  }

  return (
    <div className="table-scroll">
      <table className="complaint-table">
        <thead>
          <tr>
            <th>ID tiket</th>
            <th>Asal / tujuan</th>
            <th>Kiriman</th>
            <th>Status</th>
            <th>Status sejak</th>
            <th aria-label="Buka detail" />
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.id || item.complaint_id}>
              <td>
                <Link to={complaintTarget(item, variant)} className="table-ticket-id">
                  <span className="ticket-id-text">{item.id || item.complaint_id}</span>
                  <ArrowRight size={13} />
                </Link>
              </td>
              <td>
                <div className="cell-office">
                  <Building2 size={13} />
                  <span>
                    {item.origin_office_code || item.origin_office_update_code}
                    {item.destination_office_code || item.shipment_destination_office_code
                      ? ` → ${item.destination_office_code || item.shipment_destination_office_code}`
                      : ''}
                  </span>
                </div>
              </td>
              <td>
                <span className="cell-monospace">{item.shipment_number || '—'}</span>
              </td>
              <td>
                <StatusBadge code={item.status_code || item.latest_status_code} />
              </td>
              <td>
                <span className="cell-date">
                  <CalendarClock size={13} />
                  {formatDateTime(item.status_date || item.latest_status_at)}
                </span>
              </td>
              <td className="table-action-cell">
                <Link
                  to={complaintTarget(item, variant)}
                  className="table-action"
                  aria-label={`Buka tiket ${item.id || item.complaint_id}`}
                >
                  <ArrowRight size={15} />
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
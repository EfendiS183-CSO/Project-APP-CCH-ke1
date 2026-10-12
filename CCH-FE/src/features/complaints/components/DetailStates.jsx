import { AlertCircle, Loader2 } from 'lucide-react'

export function DetailError({ message }) {
  return (
    <div className="page-wide">
      <div className="alert alert-error" role="alert">
        <AlertCircle size={16} />
        <span>{message || 'Tiket tidak ditemukan atau tidak dapat diakses.'}</span>
      </div>
    </div>
  )
}

export function DetailLoading() {
  return (
    <div className="page-wide queue-state">
      <Loader2 size={22} className="spin" />
      <span>Memuat tiket…</span>
    </div>
  )
}
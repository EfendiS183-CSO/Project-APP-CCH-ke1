import { ArrowLeft, CheckCircle2, Loader2, Phone, Send, ShieldCheck } from 'lucide-react'

const SUMMARY_FIELDS = [
  ['Jenis keluhan', 'Jenis_Pengaduan'],
  ['Layanan kiriman', 'Jenis_Kiriman'],
  ['Nomor kiriman', 'Nomor_Kiriman'],
  ['Kantor asal kiriman', 'Kantor_Asal_Kiriman'],
  ['Kantor tujuan kiriman', 'Kantor_Tujuan_Kiriman'],
]

export function SubmissionReviewStep({
  payload,
  summary,
  onSummaryChange,
  consent,
  onConsentChange,
  token,
  submitting,
  submitError,
  onBack,
}) {
  return (
    <section className="form-section">
      <div className="form-section-head">
        <span className="icon-tile"><ShieldCheck size={18} /></span>
        <div>
          <h2>Ringkasan & persetujuan</h2>
          <p>Periksa sekali lagi sebelum mengirim. Ringkasan wajib diisi sebagai jejak pengajuan.</p>
        </div>
      </div>
      <dl className="summary-list">
        <div className="summary-row"><dt>Nama pelanggan</dt><dd>{payload.Nama_Pelanggan || '—'}</dd></div>
        <div className="summary-row">
          <dt>Kontak</dt>
          <dd><Phone size={13} /> {payload.Nomor_HP_Pelanggan || '—'}{payload.Email_Pelanggan ? ` · ${payload.Email_Pelanggan}` : ''}</dd>
        </div>
        {SUMMARY_FIELDS.map(([label, key]) => (
          <div className="summary-row" key={label}><dt>{label}</dt><dd>{payload[key] || '—'}</dd></div>
        ))}
        <div className="summary-row"><dt>Uraian pengaduan</dt><dd className="summary-description">{payload.Uraian_Pengaduan || '—'}</dd></div>
      </dl>
      <label className="field field-wide">
        <span>Ringkasan pengaduan *</span>
        <input type="text" maxLength={100} value={summary} onChange={onSummaryChange} placeholder="Ringkasan singkat (maks. 100 karakter)" />
      </label>
      <label className={`consent-box ${consent ? 'is-checked' : ''}`}>
        <input type="checkbox" checked={consent} onChange={onConsentChange} />
        <span className="consent-box-mark"><CheckCircle2 size={16} /></span>
        <span>
          <strong>Persetujuan eksplisit</strong>
          Saya menyetujui data keluhan saya diproses untuk pembuatan tiket penanganan keluhan.
        </span>
      </label>
      {submitError ? (
        <div className="alert alert-error" role="alert">
          <span aria-hidden="true">!</span>
          <span>{submitError}</span>
        </div>
      ) : null}
      <div className="form-actions">
        <button type="button" className="btn btn-secondary" onClick={onBack} disabled={submitting}>
          <ArrowLeft size={16} />
          Kembali
        </button>
        <button type="submit" className="btn btn-primary" disabled={submitting || !token || !consent || !summary.trim()}>
          {submitting ? <Loader2 size={16} className="spin" /> : <Send size={16} />}
          {submitting ? 'Mengirim…' : 'Kirim pengajuan'}
        </button>
      </div>
    </section>
  )
}

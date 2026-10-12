import { AlertCircle, Loader2, ShieldCheck, Ticket } from 'lucide-react'

const CORRECTIONS = [
  ['Kantor tujuan', 'Kantor_Tujuan'],
  ['Kantor asal kiriman', 'Kantor_Asal_Kiriman'],
  ['Kantor tujuan kiriman', 'Kantor_Tujuan_Kiriman'],
]

export function TicketCreationPanel({
  draft,
  selected,
  corrections,
  onCorrectionsChange,
  identityVerified,
  onIdentityVerifiedChange,
  creating,
  createError,
  onCreate,
}) {
  return (
    <section className="panel review-panel create-panel">
      <div className="panel-heading">
        <div>
          <h2>Verifikasi & buat tiket</h2>
          <p className="panel-sub">Koreksi kantor terbatas pada tiga field berikut; isi hanya jika perlu diubah dari pengajuan.</p>
        </div>
        <span className="icon-tile"><Ticket size={18} /></span>
      </div>

      <div className="form-grid">
        {CORRECTIONS.map(([label, key]) => (
          <label className="field" key={key}>
            <span>{label}</span>
            <input
              type="text"
              maxLength={7}
              value={corrections[key]}
              onChange={(event) => onCorrectionsChange({ ...corrections, [key]: event.target.value })}
              placeholder={draft.payload?.[key] || 'Kode kantor'}
            />
          </label>
        ))}
      </div>

      <div className="review-attestation">
        <ShieldCheck size={16} />
        <span>Identitas yang dipilih akan dikirim sebagai attestation staf dan divalidasi bersama sesi JWT oleh backend.</span>
      </div>

      <label className="consent-box">
        <input type="checkbox" checked={identityVerified} onChange={onIdentityVerifiedChange} />
        <span className="consent-box-mark"><ShieldCheck size={16} /></span>
        <span><strong>Verifikasi identitas selesai</strong>Saya sudah memastikan pelanggan sesuai dengan profil CCH yang dipilih.</span>
      </label>

      {createError ? (
        <div className="alert alert-error" role="alert"><AlertCircle size={15} /><span>{createError}</span></div>
      ) : null}

      <div className="form-actions">
        <button type="button" className="btn btn-primary" disabled={!selected || !identityVerified || creating} onClick={onCreate}>
          {creating ? <Loader2 size={16} className="spin" /> : <Ticket size={16} />}
          {creating ? 'Membuat tiket…' : 'Buat tiket setelah verifikasi'}
        </button>
      </div>
    </section>
  )
}

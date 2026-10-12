import { ArrowLeft, ArrowRight, ClipboardCheck } from 'lucide-react'

export function ShipmentDetailsStep({ payload, setField, onBack, onNext }) {
  return (
    <section className="form-section">
      <div className="form-section-head">
        <span className="icon-tile"><ClipboardCheck size={18} /></span>
        <div>
          <h2>Detail kiriman</h2>
          <p>Informasi layanan dan routing membantu backend memvalidasi referensi master.</p>
        </div>
      </div>
      <div className="form-grid">
        <label className="field">
          <span>Kode jenis keluhan CCH *</span>
          <input type="number" min={1} max={255} value={payload.Jenis_Pengaduan} onChange={setField('Jenis_Pengaduan')} placeholder="Kode aktif dari daftar CCH" />
        </label>
        <label className="field">
          <span>Kode layanan kiriman CCH *</span>
          <input type="text" maxLength={5} value={payload.Jenis_Kiriman} onChange={setField('Jenis_Kiriman')} placeholder="Kode layanan dari CCH" />
        </label>
        <label className="field field-wide">
          <span>Nomor kiriman / resi *</span>
          <input type="text" maxLength={255} value={payload.Nomor_Kiriman} onChange={setField('Nomor_Kiriman')} placeholder="Nomor resi yang dilaporkan" />
        </label>
        <label className="field">
          <span>Kantor asal kiriman *</span>
          <input type="text" maxLength={7} value={payload.Kantor_Asal_Kiriman} onChange={setField('Kantor_Asal_Kiriman')} placeholder="Kode kantor (contoh: 40005)" />
        </label>
        <label className="field">
          <span>Kantor tujuan kiriman *</span>
          <input type="text" maxLength={7} value={payload.Kantor_Tujuan_Kiriman} onChange={setField('Kantor_Tujuan_Kiriman')} placeholder="Kode kantor tujuan" />
        </label>
      </div>
      <div className="form-actions">
        <button type="button" className="btn btn-secondary" onClick={onBack}>
          <ArrowLeft size={16} />
          Kembali
        </button>
        <button type="button" className="btn btn-primary" onClick={onNext}>
          Lanjut ke ringkasan
          <ArrowRight size={16} />
        </button>
      </div>
    </section>
  )
}

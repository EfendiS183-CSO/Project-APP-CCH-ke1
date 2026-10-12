import { ArrowRight, UserRound } from 'lucide-react'

export function CustomerDetailsStep({ payload, setField, onNext }) {
  return (
    <section className="form-section">
      <div className="form-section-head">
        <span className="icon-tile"><UserRound size={18} /></span>
        <div>
          <h2>Data pelanggan</h2>
          <p>Identitas dipakai backend untuk mencocokkan profil CCH secara tepat.</p>
        </div>
      </div>
      <div className="form-grid">
        <label className="field">
          <span>Nama lengkap pelanggan *</span>
          <input type="text" maxLength={50} value={payload.Nama_Pelanggan} onChange={setField('Nama_Pelanggan')} placeholder="Nama sesuai identitas" />
        </label>
        <label className="field">
          <span>Nomor HP pelanggan *</span>
          <input type="tel" maxLength={100} value={payload.Nomor_HP_Pelanggan} onChange={setField('Nomor_HP_Pelanggan')} placeholder="08xxxxxxxxxx" />
        </label>
        <label className="field">
          <span>Email (opsional)</span>
          <input type="email" maxLength={100} value={payload.Email_Pelanggan} onChange={setField('Email_Pelanggan')} placeholder="nama@domain.com" />
        </label>
        <label className="field field-wide">
          <span>Alamat lengkap *</span>
          <textarea rows={3} maxLength={5000} value={payload.Alamat_Pelanggan} onChange={setField('Alamat_Pelanggan')} placeholder="Alamat lengkap pelanggan" />
        </label>
        <label className="field field-wide">
          <span>Uraian pengaduan *</span>
          <textarea rows={5} maxLength={5000} value={payload.Uraian_Pengaduan} onChange={setField('Uraian_Pengaduan')} placeholder="Jelaskan kronologi keluhan dengan jelas…" />
        </label>
      </div>
      <div className="form-actions">
        <button type="button" className="btn btn-primary" onClick={onNext}>
          Lanjut ke detail kiriman
          <ArrowRight size={16} />
        </button>
      </div>
    </section>
  )
}

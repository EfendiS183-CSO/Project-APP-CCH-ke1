const FIELDS = [
  ['Nama pelanggan', 'Nama_Pelanggan'],
  ['Email pelanggan', 'Email_Pelanggan'],
  ['Nomor HP', 'Nomor_HP_Pelanggan'],
  ['Alamat', 'Alamat_Pelanggan'],
  ['Kantor tujuan', 'Kantor_Tujuan'],
  ['Jenis pengaduan', 'Jenis_Pengaduan'],
  ['Layanan kiriman', 'Jenis_Kiriman'],
  ['Nomor kiriman', 'Nomor_Kiriman'],
  ['Kantor asal kiriman', 'Kantor_Asal_Kiriman'],
  ['Kantor tujuan kiriman', 'Kantor_Tujuan_Kiriman'],
  ['Uraian pengaduan', 'Uraian_Pengaduan'],
]

export function DraftPayloadPanel({ draft }) {
  const rows = FIELDS
    .map(([label, key]) => ({ label, value: String(draft.payload?.[key] ?? '—') }))
    .filter((row) => row.value !== '—')

  return (
    <section className="panel review-panel">
      <div className="panel-heading">
        <h2>Data pengajuan</h2>
        <span className="count-chip">{rows.length} field</span>
      </div>
      <dl className="summary-list compact">
        {rows.map((row) => (
          <div className="summary-row" key={row.label}>
            <dt>{row.label}</dt>
            <dd>{row.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

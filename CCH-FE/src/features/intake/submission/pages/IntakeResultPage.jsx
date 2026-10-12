import { useLocation, Link } from 'react-router-dom'
import {
  ArrowRight,
  CheckCircle2,
  Clock3,
  FileQuestion,
  Inbox,
  Ticket,
  Undo2,
} from 'lucide-react'

function ResultCard({ title, detail, state }) {
  const tone = state === 'created' ? 'is-success' : state === 'pending_cs' ? 'is-warn' : 'is-info'
  return (
    <section className={`panel result-card ${tone}`}>
      <span className="icon-tile">
        {state === 'created' ? <Ticket size={20} /> : <Clock3 size={20} />}
      </span>
      <div>
        <h1>{title}</h1>
        <p>{detail}</p>
      </div>
    </section>
  )
}

export default function IntakeResultPage() {
  const location = useLocation()
  const result = location.state?.result

  if (!result) {
    return (
      <div className="page-narrow">
        <section className="panel result-card is-info">
          <span className="icon-tile"><FileQuestion size={20} /></span>
          <div>
            <h1>Belum ada hasil pengajuan</h1>
            <p>Mulai pengajuan dari halaman intake untuk melihat hasilnya di sini.</p>
          </div>
        </section>
        <div className="result-actions">
          <Link to="/intake" className="btn btn-primary">
            <Undo2 size={16} />
            Kembali ke intake
          </Link>
        </div>
      </div>
    )
  }

  const isCreated = result.status === 'created'

  return (
    <div className="page-narrow">
      <div className="page-heading">
        <div>
          <p className="eyebrow">Status pengajuan</p>
          <h1>Hasil pengajuan</h1>
          <p className="page-heading-sub">
            Backend memproses submission Anda dan mengembalikan status final.
          </p>
        </div>
        {isCreated ? <CheckCircle2 size={26} className="result-success-icon" /> : <Clock3 size={26} />}
      </div>

      {isCreated ? (
        <ResultCard
          state="created"
          title="Tiket berhasil dibuat"
          detail={`Pengaduan Anda tercatat dengan nomor tiket ${result.ticket_id ?? ''}. Simpan nomor ini untuk pengecekan lanjutan.`}
        />
      ) : (
        <ResultCard
          state={result.status === 'pending_cs' ? 'pending_cs' : 'creating'}
          title={result.status === 'pending_cs' ? 'Menunggu review Call Center' : 'Pengajuan sedang diproses'}
          detail={
            result.status === 'pending_cs'
              ? 'Profil Anda belum dapat dicocokkan secara pasti, sehingga pengajuan diteruskan ke petugas Call Center untuk ditinjau.'
              : 'Pengajuan masih dalam proses rekonsiliasi; menyegarkan halaman ini dapat mengubah status.'
          }
        />
      )}

      {!isCreated && result.reason ? (
        <div className="alert alert-info">
          <Inbox size={16} />
          <span>Alasan antrean: <code>{result.reason}</code></span>
        </div>
      ) : null}

      <div className="result-actions">
        <Link to="/" className="btn btn-secondary">
          <ArrowRight size={16} />
          Kembali ke beranda
        </Link>
        <Link to="/intake" className="text-link">
          Laporkan keluhan lain
        </Link>
      </div>
    </div>
  )
}
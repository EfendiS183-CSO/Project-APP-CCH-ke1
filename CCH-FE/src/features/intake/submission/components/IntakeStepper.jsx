import { CheckCircle2 } from 'lucide-react'

const STEPS = [
  ['Data pelanggan', 'identitas & uraian'],
  ['Detail kiriman', 'routing & layanan'],
  ['Ringkasan & persetujuan', 'cek sebelum kirim'],
]

export function IntakeStepper({ step }) {
  return (
    <ol className="stepper" aria-label="Langkah pengajuan">
      {STEPS.map(([title, detail], index) => {
        const number = index + 1
        return (
          <li className={`stepper-item ${step >= number ? 'is-done' : ''}`} key={title}>
            <span className="stepper-dot">
              {step > number || step === number && number === 3 ? <CheckCircle2 size={14} /> : number}
            </span>
            <span>
              {title}
              <small>{detail}</small>
            </span>
          </li>
        )
      })}
    </ol>
  )
}

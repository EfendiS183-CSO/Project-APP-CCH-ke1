import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertCircle, CheckCircle2, Loader2 } from 'lucide-react'
import { createIntakeSession, submitIntakeSubmission } from '../api/submissionApi.js'
import { CustomerDetailsStep } from '../components/CustomerDetailsStep.jsx'
import { IntakeStepper } from '../components/IntakeStepper.jsx'
import { ShipmentDetailsStep } from '../components/ShipmentDetailsStep.jsx'
import { SubmissionReviewStep } from '../components/SubmissionReviewStep.jsx'
import { buildIntakeSubmission, validateIntakeStep } from '../validation.js'

const EMPTY_PAYLOAD = {
  Nama_Pelanggan: '',
  Email_Pelanggan: '',
  Nomor_HP_Pelanggan: '',
  Alamat_Pelanggan: '',
  Kantor_Tujuan: '',
  Jenis_Pengaduan: '',
  Jenis_Kiriman: '',
  Nomor_Kiriman: '',
  Kantor_Asal_Kiriman: '',
  Kantor_Tujuan_Kiriman: '',
  Uraian_Pengaduan: '',
}

function useIntakeSession() {
  const [session, setSession] = useState(null)
  const [sessionError, setSessionError] = useState('')

  const retry = useCallback(() => {
    setSessionError('')
    createIntakeSession()
      .then(setSession)
      .catch((error) => {
        setSessionError(error.message || 'Gagal memulai sesi intake.')
        setSession(null)
      })
  }, [])

  useEffect(() => {
    let cancelled = false
    createIntakeSession()
      .then((issued) => {
        if (!cancelled) setSession(issued)
      })
      .catch((error) => {
        if (!cancelled) setSessionError(error.message || 'Gagal memulai sesi intake.')
      })
    return () => {
      cancelled = true
    }
  }, [])

  return { session, sessionError, retry }
}

export default function GuestIntakePage() {
  const navigate = useNavigate()
  const { session, sessionError, retry } = useIntakeSession()
  const [step, setStep] = useState(1)
  const [consent, setConsent] = useState(false)
  const [summary, setSummary] = useState('')
  const [payload, setPayload] = useState(EMPTY_PAYLOAD)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')
  const [validation, setValidation] = useState([])
  const token = session?.token

  const setField = useCallback(
    (key) => (event) => setPayload((current) => ({ ...current, [key]: event.target.value })),
    [],
  )

  function validateStep() {
    const errors = validateIntakeStep(step, payload)
    setValidation(errors)
    if (errors.length) window.scrollTo({ top: 0, behavior: 'smooth' })
    return errors.length === 0
  }

  function moveTo(nextStep) {
    if (nextStep > step && !validateStep()) return
    setValidation([])
    setStep(nextStep)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  async function handleSubmit(event) {
    event.preventDefault()
    if (!token || !validateStep()) return
    setSubmitting(true)
    setSubmitError('')
    try {
      const result = await submitIntakeSubmission(buildIntakeSubmission(consent, summary, payload), token)
      void navigate('/intake/result', { state: { result } })
    } catch (error) {
      setSubmitError(error.message || 'Pengajuan gagal dikirim.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="page-wide intake-page">
      <div className="page-heading">
        <div>
          <p className="eyebrow">Lapor keluhan langsung</p>
          <h1>Intake pelanggan</h1>
          <p className="page-heading-sub">
            Data dikirim sekali dengan persetujuan; tiket dibuat oleh backend hanya jika profil cocok.
          </p>
        </div>
        <span className={`session-badge ${token ? 'is-live' : ''}`}>
          {token ? <CheckCircle2 size={14} /> : <Loader2 size={14} className="spin" />}
          {token ? 'Sesi aktif' : 'Menyiapkan sesi…'}
        </span>
      </div>

      {sessionError ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <div><strong>Sesi intake tidak bisa dimulai.</strong><span>{sessionError}</span></div>
          <button type="button" className="btn btn-secondary btn-small" onClick={retry}>Coba lagi</button>
        </div>
      ) : null}

      <IntakeStepper step={step} />
      {validation.length ? (
        <div className="alert alert-error" role="alert">
          <AlertCircle size={16} />
          <ul className="alert-list">{validation.map((message) => <li key={message}>{message}</li>)}</ul>
        </div>
      ) : null}

      <form onSubmit={handleSubmit} className="intake-form">
        {step === 1 ? <CustomerDetailsStep payload={payload} setField={setField} onNext={() => moveTo(2)} /> : null}
        {step === 2 ? (
          <ShipmentDetailsStep
            payload={payload}
            setField={setField}
            onBack={() => moveTo(1)}
            onNext={() => moveTo(3)}
          />
        ) : null}
        {step === 3 ? (
          <SubmissionReviewStep
            payload={payload}
            summary={summary}
            onSummaryChange={(event) => setSummary(event.target.value)}
            consent={consent}
            onConsentChange={(event) => setConsent(event.target.checked)}
            token={token}
            submitting={submitting}
            submitError={submitError}
            onBack={() => moveTo(2)}
          />
        ) : null}
        <p className="form-legal">
          <Link to="/">Kembali ke beranda</Link>
          <span>Pengajuan diproses sesuai kontrak CCH API v1.</span>
        </p>
      </form>
    </div>
  )
}

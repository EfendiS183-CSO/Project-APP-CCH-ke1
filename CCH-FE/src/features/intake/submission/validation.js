const REQUIRED_FIELDS = {
  1: [
    ['Nama_Pelanggan', 'Nama pelanggan wajib diisi.'],
    ['Nomor_HP_Pelanggan', 'Nomor HP pelanggan wajib diisi.'],
    ['Alamat_Pelanggan', 'Alamat pelanggan wajib diisi.'],
    ['Uraian_Pengaduan', 'Uraian pengaduan wajib diisi.'],
  ],
  2: [
    ['Jenis_Pengaduan', 'Jenis keluhan wajib dipilih.'],
    ['Jenis_Kiriman', 'Layanan kiriman wajib diisi.'],
    ['Nomor_Kiriman', 'Nomor kiriman wajib diisi.'],
    ['Kantor_Asal_Kiriman', 'Kantor asal kiriman wajib diisi.'],
    ['Kantor_Tujuan_Kiriman', 'Kantor tujuan kiriman wajib diisi.'],
  ],
}

function isPlausibleEmail(value) {
  const at = value.indexOf('@')
  if (at <= 0 || at !== value.lastIndexOf('@') || at === value.length - 1) return false
  const domain = value.slice(at + 1)
  const dot = domain.lastIndexOf('.')
  return dot > 0 && dot < domain.length - 1 && !value.includes(' ')
}

export function validateIntakeStep(step, payload) {
  const errors = (REQUIRED_FIELDS[step] || [])
    .filter(([key]) => !String(payload[key] ?? '').trim())
    .map(([, message]) => message)

  const email = String(payload.Email_Pelanggan ?? '').trim()
  if (step === 1 && email && !isPlausibleEmail(email)) {
    errors.push('Format email tidak valid.')
  }

  return errors
}

export function buildIntakeSubmission(consent, summary, payload) {
  const cleanPayload = { ...payload }
  cleanPayload.Email_Pelanggan = cleanPayload.Email_Pelanggan.trim() || undefined
  cleanPayload.Kantor_Tujuan = cleanPayload.Kantor_Tujuan.trim() || undefined
  cleanPayload.Jenis_Pengaduan = Number(cleanPayload.Jenis_Pengaduan)

  return {
    consent: Boolean(consent),
    summary: summary.trim(),
    payload: cleanPayload,
  }
}

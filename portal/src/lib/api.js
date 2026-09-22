/*
 * Pembungkus fetch untuk API portal (base path /api/v1).
 *
 * Semua panggilan ke backend lewat sini supaya cookie sesi selalu ikut
 * (credentials: same-origin) dan bentuk error-nya seragam:
 *   Error dengan .status (angka) dan .code (string dari field "error").
 *
 * Jawaban 204 (login/logout) tidak punya body, jadi hasilnya null.
 */

const BASE = '/api/v1'

function fail(message, status, code) {
  const e = new Error(message)
  e.status = status
  e.code = code
  return e
}

async function req(method, path, body) {
  const opt = {
    method,
    credentials: 'same-origin',
    headers: { Accept: 'application/json' }
  }
  if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json'
    opt.body = JSON.stringify(body)
  }

  let res
  try {
    res = await fetch(BASE + path, opt)
  } catch {
    throw fail('Tidak dapat menghubungi server.', 0, 'network')
  }

  return bacaJawaban(res)
}

/*
 * Membaca jawaban server dan mengubah galat HTTP jadi Error yang seragam.
 * Dipisah dari req() supaya unggahan berkas multipart memakai bentuk yang
 * sama.
 */
async function bacaJawaban(res) {
  if (res.status === 204) return null

  let data = null
  const text = await res.text()
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }

  if (!res.ok) {
    const code = data && typeof data.error === 'string' ? data.error : ''
    /*
     * Server mengirim kalimat siap tampil di field "message" (mis. subdomain
     * bentrok). Pesannya ditaruh di err.message supaya pemanggil bisa
     * menampilkannya apa adanya, dan ditandai lewat serverMessage supaya
     * pesanGagal() tidak menampilkan kode galat mentah seperti "invalid".
     */
    const pesan = data && typeof data.message === 'string' ? data.message.trim() : ''
    const err = fail(pesan || code || `Permintaan gagal (HTTP ${res.status}).`, res.status, code)
    if (pesan) err.serverMessage = pesan
    /* 409 "already_pending" menyertakan klaim yang sudah ada. */
    if (data && data.claim) err.claim = data.claim
    throw err
  }

  if (data === null) throw fail('Jawaban server tidak terbaca.', res.status, '')
  return data
}

const enc = encodeURIComponent

/* ------------------------------------------------------------ pelanggan -- */

export function payInfo(token) {
  return req('GET', `/pay/${enc(token)}`)
}

export function payClaim(token, planCode) {
  return req('POST', `/pay/${enc(token)}/claim`, { plan_code: planCode })
}

/* ---------------------------------------------------------------- admin -- */

export function adminMe() {
  return req('GET', '/admin/me')
}

export function adminLogin(password) {
  return req('POST', '/admin/login', { password })
}

export function adminLogout() {
  return req('POST', '/admin/logout')
}

export function adminOverview() {
  return req('GET', '/admin/overview')
}

/*
 * Unggah gambar QRIS. Ini panggilan multipart, bukan JSON: req() selalu
 * mengirim Content-Type: application/json, sedangkan di sini header itu harus
 * dibiarkan kosong supaya browser menuliskan boundary multipart-nya sendiri.
 */
export async function adminUploadQRIS(file) {
  const fd = new FormData()
  fd.append('file', file)

  let res
  try {
    res = await fetch(BASE + '/admin/qris', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
      body: fd
    })
  } catch {
    throw fail('Tidak dapat menghubungi server.', 0, 'network')
  }

  return bacaJawaban(res)
}

export function adminDeleteQRIS() {
  return req('DELETE', '/admin/qris')
}

/** action: "approve" | "reject" */
export function adminClaim(id, action) {
  return req('POST', `/admin/claims/${enc(id)}/${action}`)
}

/** action: "suspend" | "activate" */
export function adminCustomer(id, action) {
  return req('POST', `/admin/customers/${enc(id)}/${action}`)
}

/* ------------------------------------------------- kelola pelanggan ----- */

export function adminPlans() {
  return req('GET', '/admin/plans')
}

export function adminInstances() {
  return req('GET', '/admin/instances')
}

/** body: {name, kind} — kind: "shared" | "dedicated" */
export function adminCreateInstance(body) {
  return req('POST', '/admin/instances', body)
}

/** Ditolak server kalau panelnya masih menangani pelanggan. */
export function adminDeleteInstance(id) {
  return req('DELETE', `/admin/instances/${encodeURIComponent(id)}`)
}

/** body: {name, institution, wa, session_name, instance_id, plan_code} */
export function adminCreateCustomer(body) {
  return req('POST', '/admin/customers', body)
}

/** body: {name, institution, wa, session_name, instance_id} */
export function adminUpdateCustomer(id, body) {
  return req('PATCH', `/admin/customers/${enc(id)}`, body)
}

export function adminExtend(id, months) {
  return req('POST', `/admin/customers/${enc(id)}/extend`, { months })
}

export function adminDeleteCustomer(id) {
  return req('DELETE', `/admin/customers/${enc(id)}`)
}

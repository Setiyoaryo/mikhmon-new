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
    const err = fail(code || `Permintaan gagal (HTTP ${res.status}).`, res.status, code)
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

/** action: "approve" | "reject" */
export function adminClaim(id, action) {
  return req('POST', `/admin/claims/${enc(id)}/${action}`)
}

/** action: "suspend" | "activate" */
export function adminCustomer(id, action) {
  return req('POST', `/admin/customers/${enc(id)}/${action}`)
}

/*
 * Pemformat tampilan dan pesan kesalahan.
 * Dipakai bersama oleh halaman pelanggan, admin, dan form login.
 */

export function fmtRupiah(n) {
  return 'Rp ' + (n || 0).toLocaleString('id-ID')
}

export function fmtDate(v) {
  const d = v instanceof Date ? v : new Date(v)
  if (!(d instanceof Date && !isNaN(d))) return '-'
  const bulan = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
  return `${d.getDate()} ${bulan[d.getMonth()]} ${d.getFullYear()}`
}

export const stateLabel = {
  active: 'Aktif',
  warning: 'Segera berakhir',
  grace: 'Masa tenggang',
  expired: 'Berakhir',
  pending: 'Menunggu verifikasi'
}

/** ID instalasi: 12 karakter, ditampilkan berkelompok. */
export function prettyId(id) {
  return String(id || '').replace(/^(.{4})(.{4})(.{4})$/, '$1-$2-$3')
}

/** Kalimat kesalahan berbahasa Indonesia untuk Error dari api.js. */
export function pesanGagal(err, fallback = 'Terjadi kesalahan. Coba lagi.') {
  if (!err) return fallback
  if (err.status === 0) return 'Tidak dapat menghubungi server. Periksa koneksi lalu coba lagi.'
  if (err.status === 401) return 'Sesi berakhir. Silakan masuk lagi.'
  /* Penjelasan dari server (biasanya 400 "invalid") dipakai apa adanya. */
  if (err.serverMessage) return err.serverMessage
  if (err.status === 404) return 'Data tidak ditemukan. Tautan mungkin sudah tidak berlaku.'
  if (err.status === 409) return 'Klaim pembayaran sudah menunggu verifikasi.'
  return fallback
}

/*
 * Nama sesi/subdomain yang aman, cermin dari NormalizeSessionName di server:
 * huruf kecil, angka, dan tanda hubung saja. Dipakai untuk mengisi kolom
 * Subdomain otomatis dari nama usaha, jadi hasilnya sama dengan yang
 * disimpan server.
 */
export function subdomainDari(teks) {
  let out = ''
  let dash = false
  for (const c of String(teks || '').toLowerCase().trim()) {
    if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
      out += c
      dash = false
    } else if ((c === '-' || c === ' ' || c === '_' || c === '.') && !dash && out.length > 0) {
      out += '-'
      dash = true
    }
  }
  out = out.replace(/^-+|-+$/g, '')
  if (out.length > 40) out = out.slice(0, 40).replace(/-+$/g, '')
  return out
}

/** Nomor WhatsApp jadi digit untuk tautan wa.me (08xx dan 8xx jadi 628xx). */
export function waNomor(wa) {
  let d = String(wa || '').replace(/[^0-9]/g, '')
  if (!d) return ''
  if (d.startsWith('0')) d = '62' + d.slice(1)
  else if (d.startsWith('8')) d = '62' + d
  return d
}

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
  if (err.status === 404) return 'Data tidak ditemukan. Tautan mungkin sudah tidak berlaku.'
  if (err.status === 409) return 'Klaim pembayaran sudah menunggu verifikasi.'
  return fallback
}

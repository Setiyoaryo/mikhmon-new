/*
 * Data contoh untuk mockup.
 *
 * Bentuk datanya sengaja dibuat sama dengan skema yang akan dipakai portal
 * sungguhan (SQLite), supaya saat backend-nya dibuat tinggal ganti sumbernya:
 *
 *   customers    id, name, institution, wa, created_at
 *   instances    id (12 hex), token, customer_id, router_name, last_seen, version
 *   plans        code, label, months, price
 *   payments     id, customer_id, plan_code, amount, status, created_at, approved_at
 *   ledger       catatan perubahan status (audit)
 */

export const plans = [
  { code: 'P1M', label: '1 Bulan', months: 1, price: 50000 },
  { code: 'P3M', label: '3 Bulan', months: 3, price: 135000, note: 'hemat 10%' },
  { code: 'P6M', label: '6 Bulan', months: 6, price: 250000, note: 'hemat 17%' },
  { code: 'P12M', label: '12 Bulan', months: 12, price: 450000, note: 'hemat 25%' }
]

export const dayMs = 86400000

export function fmtRupiah(n) {
  return 'Rp ' + (n || 0).toLocaleString('id-ID')
}

export function fmtDate(d) {
  const bulan = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
  return `${d.getDate()} ${bulan[d.getMonth()]} ${d.getFullYear()}`
}

export function addMonths(date, months) {
  const d = new Date(date.getTime())
  const hari = d.getDate()
  d.setMonth(d.getMonth() + months)
  // 31 Jan + 1 bulan jangan jadi 3 Maret
  if (d.getDate() !== hari) d.setDate(0)
  return d
}

/** sisa hari kalender, dihitung dari tengah malam supaya tidak terpengaruh jam */
export function daysLeft(expires) {
  const a = new Date()
  a.setHours(0, 0, 0, 0)
  const b = new Date(expires.getTime())
  b.setHours(0, 0, 0, 0)
  return Math.round((b - a) / dayMs)
}

export function stateOf(expires) {
  const d = daysLeft(expires)
  if (d < 0) return 'expired'
  if (d <= 7) return 'warning'
  return 'active'
}

export const stateLabel = {
  active: 'Aktif',
  warning: 'Segera berakhir',
  expired: 'Berakhir',
  pending: 'Menunggu verifikasi'
}

/** ID instalasi: 12 karakter, ditampilkan berkelompok. */
export function prettyId(id) {
  return id.replace(/^(.{4})(.{4})(.{4})$/, '$1-$2-$3')
}

function daysFromNow(n) {
  const d = new Date()
  d.setHours(12, 0, 0, 0)
  return new Date(d.getTime() + n * dayMs)
}

/**
 * Pelanggan contoh untuk halaman pembayaran.
 * `variant` cuma untuk mockup: aktif / warning / expired / pending.
 */
export function demoCustomer(variant = 'warning') {
  const expires = {
    active: daysFromNow(96),
    warning: daysFromNow(4),
    expired: daysFromNow(-12),
    pending: daysFromNow(4)
  }[variant]
  const started = addMonths(expires, -1)

  return {
    name: 'Taufiq',
    institution: 'Taufiq.net',
    wa: '6285139495106',
    instanceId: 'B83DE163FDCB',
    routerName: 'CHR-HOTSPOT',
    version: 'NOCIFY 3.20',
    payToken: 'NOC-8F3A-2C71',
    planCode: 'P1M',
    startedAt: started,
    expiresAt: expires,
    pending: variant === 'pending'
  }
}

/* ------------------------------------------------------------ data admin -- */

export const adminStats = {
  mrr: 2850000,
  mrrNote: 'bulan ini'
}

export const customers = [
  { id: 'c1', name: 'Taufiq', institution: 'Taufiq.net', instance: 'B83DE163FDCB', plan: 'P1M', days: 4, status: 'warning', lastSeen: '6 menit lalu', monthly: 50000 },
  { id: 'c2', name: 'Warung Kopi Barokah', institution: 'Barokah Hotspot', instance: '77AC41E90B12', plan: 'P3M', days: 61, status: 'active', lastSeen: '12 menit lalu', monthly: 45000 },
  { id: 'c3', name: 'Pak Hendra', institution: 'Hendra Net', instance: '2F09CD3311AA', plan: 'P1M', days: 0, status: 'warning', lastSeen: '1 jam lalu', monthly: 50000 },
  { id: 'c4', name: 'Kos Melati', institution: 'Melati WiFi', instance: 'A1B2C3D4E5F6', plan: 'P6M', days: 142, status: 'active', lastSeen: '3 menit lalu', monthly: 41666 },
  { id: 'c5', name: 'Bu Rina', institution: 'RinaNet', instance: '5C7E0A9B1234', plan: 'P1M', days: -9, status: 'expired', lastSeen: '2 hari lalu', monthly: 50000 },
  { id: 'c6', name: 'Musholla Al-Ikhlas', institution: 'Al-Ikhlas Free WiFi', instance: '9D2B44F01A83', plan: 'P12M', days: 208, status: 'active', lastSeen: '25 menit lalu', monthly: 37500 },
  { id: 'c7', name: 'Pak Yudi', institution: 'Yudi Cell', instance: '33EF88120C4D', plan: 'P1M', days: -31, status: 'expired', lastSeen: '19 hari lalu', monthly: 50000 }
]

export const claims = [
  { id: 'p1021', customer: 'Taufiq', institution: 'Taufiq.net', plan: 'P1M', amount: 50000, at: '8 menit lalu', ref: 'NOC-8F3A-2C71' },
  { id: 'p1022', customer: 'Pak Hendra', institution: 'Hendra Net', plan: 'P3M', amount: 135000, at: '1 jam lalu', ref: 'NOC-4B11-9902' },
  { id: 'p1023', customer: 'Bu Rina', institution: 'RinaNet', plan: 'P1M', amount: 50000, at: '5 jam lalu', ref: 'NOC-71D0-3E45' }
]

export const activity = [
  { at: '8 menit lalu', text: 'Taufiq mengirim klaim pembayaran Rp 50.000 (1 Bulan).' },
  { at: '2 jam lalu', text: 'Portal memberi tahu 7 panel: masih aktif. Tidak ada yang gagal.' },
  { at: '1 hari lalu', text: 'Kos Melati diperpanjang 6 bulan sampai 12 Apr 2026.' },
  { at: '3 hari lalu', text: 'Bu Rina ditangguhkan otomatis karena lewat 9 hari.' }
]

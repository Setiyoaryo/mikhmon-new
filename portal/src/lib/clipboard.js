/*
 * Menyalin teks ke papan klip tanpa pustaka tambahan.
 *
 * Clipboard API hanya ada di konteks aman (https atau localhost); kalau tidak
 * tersedia atau ditolak, dipakai cara lama lewat textarea tersembunyi supaya
 * tombol salin tetap berfungsi saat portal diakses dari alamat http biasa.
 */
export async function salinTeks(teks) {
  const isi = String(teks || '')
  if (!isi) return false

  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(isi)
      return true
    } catch {
      /* jatuh ke cara lama di bawah */
    }
  }

  try {
    const ta = document.createElement('textarea')
    ta.value = isi
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '-1000px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}

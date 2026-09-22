import { writable } from 'svelte/store'

/*
 * Router hash sederhana.
 *
 * Portal ini satu halaman yang disajikan sebagai SPA di belakang satu lokasi
 * nginx (`portal.nocify.id`) dan di-embed di binary Go, jadi tidak ada server
 * yang bisa menulis ulang path. Hash membuat seluruh navigasi jalan tanpa
 * konfigurasi tambahan, termasuk saat file-nya dibuka langsung dari disk.
 */

function readPath() {
  const h = window.location.hash.replace(/^#/, '')
  return h === '' ? '/' : h
}

export const path = writable(readPath())

window.addEventListener('hashchange', () => path.set(readPath()))

export function go(to) {
  window.location.hash = to
}

/** Cocokkan "/pay/ABC" dengan pola "/pay/:token". */
export function match(pattern, actual) {
  const p = pattern.split('/')
  const a = actual.split('/')
  if (p.length !== a.length) return null
  const params = {}
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith(':')) {
      params[p[i].slice(1)] = decodeURIComponent(a[i])
    } else if (p[i] !== a[i]) {
      return null
    }
  }
  return params
}

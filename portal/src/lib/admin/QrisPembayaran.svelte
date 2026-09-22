<script>
  import { adminDeleteQRIS, adminUploadQRIS } from '../api.js'
  import { pesanGagal } from '../format.js'

  /*
   * Kartu pengaturan QRIS: satu gambar merchant dipakai semua halaman
   * pembayaran, jadi unggahan baru langsung mengganti gambar yang lama.
   * Unggahannya multipart, bukan JSON, karena itu lewat adminUploadQRIS().
   */
  let {
    qrisUrl = '',
    onpesan = () => {},
    onmuatulang = () => {},
    on401 = () => {}
  } = $props()

  /* Nilai bawaan backend, ditampilkan sebagai keterangan gambar. */
  const merchant = 'NOCIFY, SOFTWARE'
  const nmid = 'ID1026599320839'

  const MAKS = 2 * 1024 * 1024
  const JENIS = ['image/png', 'image/jpeg', 'image/webp']

  let berkas = $state(null)
  let busy = $state(false)
  let galat = $state('')
  let fileEl = $state(null)

  let ada = $derived(typeof qrisUrl === 'string' && qrisUrl.length > 0)

  /* Saring di sisi klien dulu; server tetap memeriksa isi berkasnya sendiri. */
  function pilihBerkas(e) {
    galat = ''
    const el = e.currentTarget
    const f = el.files && el.files[0]
    if (!f) {
      berkas = null
      return
    }
    if (JENIS.indexOf(f.type) === -1) {
      berkas = null
      el.value = ''
      galat = 'Berkas itu bukan gambar PNG, JPG, atau WEBP.'
      return
    }
    if (f.size > MAKS) {
      berkas = null
      el.value = ''
      galat = 'Berkasnya terlalu besar. Maksimal 2 MB.'
      return
    }
    berkas = f
  }

  function kosongkanBerkas() {
    berkas = null
    if (fileEl) fileEl.value = ''
  }

  async function unggah() {
    if (busy || !berkas) return
    busy = true
    galat = ''
    try {
      await adminUploadQRIS(berkas)
      kosongkanBerkas()
      onpesan('Gambar QRIS diperbarui.')
      await onmuatulang()
    } catch (e) {
      if (e.status === 401) {
        on401()
        return
      }
      galat = pesanGagal(e, 'Gambar QRIS gagal diunggah.')
    } finally {
      busy = false
    }
  }

  async function hapus() {
    if (busy || !ada) return
    const lanjut = confirm(
      'Hapus gambar QRIS? Halaman pembayaran tidak akan menampilkan kode QR sampai gambar baru diunggah.'
    )
    if (!lanjut) return

    busy = true
    galat = ''
    try {
      await adminDeleteQRIS()
      onpesan('Gambar QRIS dihapus.')
      await onmuatulang()
    } catch (e) {
      if (e.status === 401) {
        on401()
        return
      }
      galat = pesanGagal(e, 'Gambar QRIS gagal dihapus.')
    } finally {
      busy = false
    }
  }
</script>

<div class="card mt-3">
  <div class="card-head">
    <i class="fa fa-qrcode"></i>
    <h3>Pembayaran QRIS</h3>
    <div class="grow"></div>
    {#if ada}
      <span class="badge neutral">terpasang</span>
    {:else}
      <span class="badge warning">belum ada</span>
    {/if}
  </div>

  <div class="card-body">
    {#if ada}
      <div class="row wrap">
        <div
          style="background:#fff;border:1px solid #d8dde2;border-radius:var(--radius);padding:10px;max-width:220px"
        >
          <img
            src={qrisUrl}
            alt="Gambar QRIS merchant {merchant}"
            style="display:block;width:100%;height:auto"
          />
        </div>
        <div class="grow" style="min-width:240px">
          <div class="small">
            Kode QR merchant <b>{merchant}</b> dengan NMID <span class="mono">{nmid}</span>.
          </div>
          <div class="tiny muted mt-1">
            Gambar inilah yang dilihat pelanggan di halaman pembayaran mereka. Mengunggah
            gambar baru langsung menggantikan yang lama.
          </div>
        </div>
      </div>
    {:else}
      <div class="box warning">
        <i class="fa fa-exclamation-triangle"></i>
        Halaman pembayaran sedang tidak menampilkan QRIS. Pelanggan belum bisa membayar
        sampai gambar QRIS diunggah.
      </div>
    {/if}

    <div class="row wrap mt-3">
      <div style="flex:1 1 260px;max-width:340px">
        <input
          class="input"
          type="file"
          accept="image/png,image/jpeg,image/webp,.png,.jpg,.jpeg,.webp"
          bind:this={fileEl}
          onchange={pilihBerkas}
          disabled={busy}
        />
      </div>
      <button class="btn btn-primary" onclick={unggah} disabled={busy || !berkas}>
        {#if busy}
          <i class="fa fa-spinner"></i> Mengunggah…
        {:else}
          <i class="fa fa-upload"></i> Unggah QRIS
        {/if}
      </button>
      <button class="btn btn-danger" onclick={hapus} disabled={busy || !ada}>
        <i class="fa fa-trash"></i> Hapus gambar
      </button>
      <div class="grow"></div>
      <span class="tiny muted">PNG, JPG, atau WEBP. Maksimal 2 MB.</span>
    </div>

    {#if galat}
      <div class="box danger mt-2">
        <i class="fa fa-exclamation-triangle"></i>
        {galat}
      </div>
    {/if}
  </div>
</div>

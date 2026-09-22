<script>
  import { adminCreatePlan, adminDeletePlan, adminUpdatePlan } from '../api.js'
  import { fmtRupiah, pesanGagal } from '../format.js'

  /*
   * Kartu paket & harga.
   *
   * Harga yang disimpan di sini yang dilihat pelanggan di halaman pembayaran
   * dan yang dipakai kolom "Rp X/bln" di daftar pelanggan, jadi tiap perubahan
   * memanggil onchanged() supaya induknya memuat ulang ringkasan sekaligus
   * daftar pelanggannya.
   *
   * Daftar paketnya datang dari induk lewat prop plans: form pelanggan memakai
   * daftar yang sama, jadi tidak ada dua sumber data yang bisa berbeda.
   *
   * planError diisi induk kalau daftar paket dan panel gagal dimuat (satu
   * permintaan untuk keduanya). Kotak galatnya cuma muncul kalau memang belum
   * ada baris yang bisa ditampilkan — supaya kalimat yang sama tidak tampil dua
   * kali di halaman, karena PanelDeploy memakai pesan itu juga.
   */
  let {
    plans = [],
    planError = '',
    planMemuat = false,
    onchanged = () => {},
    on401 = () => {},
    onmuatulang = () => {}
  } = $props()

  /*
   * Batas kotak isian disamakan dengan batas server (internal/portal/manage.go,
   * validatePlan: durasi 1-36 bulan, nama dan catatan maksimal 40 huruf).
   * Server tetap yang memutuskan; ini hanya supaya admin tidak mengetik nilai
   * yang pasti ditolak.
   */
  const MAKS_BULAN = 36
  const MAKS_DIGIT_HARGA = 12
  const MAKS_CATATAN = 40
  const MAKS_NAMA = 40

  const KOSONG = { sibuk: false, sukses: '', galat: '' }

  let sunting = $state({}) // kode -> {label, bulan, harga, catatan, dasar, baku}
  let status = $state({}) // kode -> {sibuk, sukses, galat}
  let fokus = $state('') // kode yang kolom harganya sedang dibuka
  let suksesUmum = $state('')

  /* Form paket baru. */
  let bukaForm = $state(false)
  let fBulan = $state('')
  let fKode = $state('')
  let fNama = $state('')
  let fHarga = $state('')
  let fCatatan = $state('')
  let kodeManual = $state(false) // kode sudah disentuh sendiri, jangan ditimpa
  let namaManual = $state(false) // nama sudah disentuh sendiri, jangan ditimpa
  let fSibuk = $state(false)
  let fGalat = $state('')

  /* Nilai satu paket sebagai teks, supaya "ada perubahan" tinggal dibandingkan. */
  function salinan(p) {
    return {
      label: String(p.label ?? ''),
      bulan: String(p.months ?? ''),
      harga: String(p.price ?? ''),
      catatan: String(p.note ?? '')
    }
  }

  let baris = $derived(
    plans.map((p) => {
      const d = sunting[p.code]
      const nilai = d ?? salinan(p)
      const dasar = d ? d.dasar : salinan(p)
      const st = status[p.code] ?? KOSONG
      const bulan = Number(nilai.bulan)
      const harga = Number(nilai.harga)
      return {
        kode: p.code,
        label: nilai.label,
        bulan: nilai.bulan,
        harga: nilai.harga,
        catatan: nilai.catatan,
        pelanggan: Number(p.customers || 0),
        hargaAsal: dasar.harga,
        sibuk: st.sibuk,
        sukses: st.sukses,
        galat: st.galat,
        sahBulan: /^[0-9]{1,3}$/.test(nilai.bulan) && bulan >= 1 && bulan <= MAKS_BULAN,
        sahHarga: /^[0-9]{1,12}$/.test(nilai.harga) && harga > 0,
        kotor:
          nilai.label !== dasar.label ||
          nilai.bulan !== dasar.bulan ||
          nilai.harga !== dasar.harga ||
          nilai.catatan !== dasar.catatan
      }
    })
  )

  let fSahBulan = $derived(
    /^[0-9]{1,3}$/.test(fBulan) && Number(fBulan) >= 1 && Number(fBulan) <= MAKS_BULAN
  )
  let fSahHarga = $derived(/^[0-9]{1,12}$/.test(fHarga) && Number(fHarga) > 0)
  let fSah = $derived(fKode.trim() !== '' && fSahBulan && fSahHarga && !fSibuk)

  /*
   * Lapisan sunting dibuang begitu nilai dari server sudah sama dengan yang
   * diketik, atau kalau barisnya memang belum diubah sama sekali. Dengan begitu
   * nilai server tetap yang menang, tetapi barisnya tidak berkedip balik ke
   * harga lama selagi induknya memuat ulang daftarnya.
   */
  $effect(() => {
    const buang = []
    for (const kode of Object.keys(sunting)) {
      const p = plans.find((x) => x.code === kode)
      if (!p) {
        buang.push(kode)
        continue
      }
      const d = sunting[kode]
      const s = salinan(p)
      const samaServer =
        d.label === s.label &&
        d.bulan === s.bulan &&
        d.harga === s.harga &&
        d.catatan === s.catatan
      const belumDiubah =
        d.label === d.dasar.label &&
        d.bulan === d.dasar.bulan &&
        d.harga === d.dasar.harga &&
        d.catatan === d.dasar.catatan
      if (samaServer || (!d.baku && belumDiubah)) buang.push(kode)
    }
    if (buang.length === 0) return
    const sisa = { ...sunting }
    for (const kode of buang) delete sisa[kode]
    sunting = sisa
  })

  function setStatus(kode, ubah) {
    status = { ...status, [kode]: { ...KOSONG, ...status[kode], ...ubah } }
  }

  function buangBaris(kode) {
    const s = { ...sunting }
    const t = { ...status }
    delete s[kode]
    delete t[kode]
    sunting = s
    status = t
    if (fokus === kode) fokus = ''
  }

  function ubah(kode, bidang, nilai) {
    const p = plans.find((x) => x.code === kode)
    if (!p) return
    const lama = sunting[kode]
    const dasar = lama ? lama.dasar : salinan(p)
    sunting = { ...sunting, [kode]: { ...salinan(p), ...lama, [bidang]: nilai, dasar } }
    if (status[kode] && (status[kode].sukses || status[kode].galat)) {
      setStatus(kode, { sukses: '', galat: '' })
    }
  }

  /* Angka polos: titik, spasi, dan "Rp" hasil tempel dibuang di sini. */
  function digitSaja(e, maks) {
    const el = e.currentTarget
    const bersih = el.value.replace(/[^0-9]/g, '').slice(0, maks)
    if (el.value !== bersih) el.value = bersih
    return bersih
  }

  function ubahHarga(e, kode) {
    ubah(kode, 'harga', digitSaja(e, MAKS_DIGIT_HARGA))
  }

  function ubahBulan(e, kode) {
    ubah(kode, 'bulan', digitSaja(e, 3))
  }

  function tutupHarga(kode) {
    if (fokus === kode) fokus = ''
  }

  /*
   * Begitu kolom harganya dibuka, seluruh angkanya ditandai: angka baru yang
   * diketik (atau "Rp 100.000" yang ditempel) langsung menggantikan harga lama,
   * bukan tersambung di belakangnya. Klik sekali lagi di dalam kotak menaruh
   * kursor seperti biasa kalau yang diubah cuma beberapa digit.
   */
  function mulaiFokus(node) {
    requestAnimationFrame(() => {
      node.focus()
      try {
        node.setSelectionRange(0, node.value.length)
      } catch {
        /* jenis masukan yang tidak punya seleksi: biarkan saja */
      }
    })
  }

  /* Angka polos untuk dibaca, dipakai harga yang belum sah juga. */
  function teksHarga(digit) {
    return digit === '' ? 'belum diisi' : fmtRupiah(Number(digit))
  }

  /*
   * Kalimat galat dari server dipakai apa adanya (api.js menaruhnya di
   * err.serverMessage). pesanGagal() hanya untuk galat yang tidak membawa
   * kalimat siap tampil — kecuali 409, yang di sini selalu soal paket terpakai,
   * bukan klaim pembayaran yang menunggu verifikasi.
   */
  function galatAksi(e, cadangan) {
    if (e && e.serverMessage) return e.serverMessage
    if (e && e.status === 409) return cadangan
    return pesanGagal(e, cadangan)
  }

  function bukaHarga(kode) {
    fokus = kode
  }

  async function simpan(r) {
    if (r.sibuk || !r.kotor || !r.sahBulan || !r.sahHarga) return
    const body = {
      label: r.label.trim() || `${Number(r.bulan)} Bulan`,
      months: Number(r.bulan),
      price: Number(r.harga),
      note: r.catatan.trim()
    }

    setStatus(r.kode, { sibuk: true, sukses: '', galat: '' })
    try {
      const res = await adminUpdatePlan(r.kode, body)
      const p = res && res.plan ? res.plan : null
      /*
       * Baris ditahan pada jawaban server, bukan pada nilai yang diketik, sampai
       * induk selesai memuat ulang daftarnya.
       */
      const nilai = p
        ? salinan(p)
        : { label: body.label, bulan: String(body.months), harga: String(body.price), catatan: body.note }
      sunting = { ...sunting, [r.kode]: { ...nilai, dasar: nilai, baku: true } }
      setStatus(r.kode, {
        sibuk: false,
        sukses:
          String(body.price) === r.hargaAsal
            ? `Paket ${r.kode} disimpan.`
            : `Harga ${r.kode} disimpan.`,
        galat: ''
      })
      await onchanged()
    } catch (e) {
      if (e.status === 401) {
        on401()
        return
      }
      setStatus(r.kode, {
        sibuk: false,
        sukses: '',
        galat: galatAksi(e, `Paket ${r.kode} gagal disimpan.`)
      })
    } finally {
      if (status[r.kode]) setStatus(r.kode, { sibuk: false })
    }
  }

  async function hapus(r) {
    if (r.sibuk) return
    const tanya =
      r.pelanggan > 0
        ? `Hapus paket ${r.kode} (${r.label})? Paket ini masih dipakai ${r.pelanggan} pelanggan, jadi portal akan menolaknya selama pelanggan itu belum dipindahkan.`
        : `Hapus paket ${r.kode} (${r.label})? Paket ini tidak bisa dikembalikan.`
    if (!window.confirm(tanya)) return

    setStatus(r.kode, { sibuk: true, sukses: '', galat: '' })
    try {
      await adminDeletePlan(r.kode)
      buangBaris(r.kode)
      suksesUmum = `Paket ${r.kode} dihapus.`
      await onchanged()
    } catch (e) {
      if (e.status === 401) {
        on401()
        return
      }
      /*
       * 409 "in_use" membawa kalimat dari server; barisnya dibiarkan utuh supaya
       * paketnya masih bisa dipakai memindahkan pelanggan dulu.
       */
      setStatus(r.kode, {
        sibuk: false,
        sukses: '',
        galat: galatAksi(
          e,
          `Paket ${r.kode} masih dipakai pelanggan. Pindahkan dulu pelanggannya ke paket lain.`
        )
      })
    } finally {
      if (status[r.kode]) setStatus(r.kode, { sibuk: false })
    }
  }

  /* -------------------------------------------------------- paket baru ---- */

  function kosongkanForm() {
    fBulan = ''
    fKode = ''
    fNama = ''
    fHarga = ''
    fCatatan = ''
    kodeManual = false
    namaManual = false
    fGalat = ''
  }

  function tutupForm() {
    bukaForm = false
    kosongkanForm()
  }

  function tombolForm() {
    suksesUmum = ''
    if (bukaForm) {
      tutupForm()
      return
    }
    kosongkanForm()
    bukaForm = true
  }

  /* Durasi mengisi kode (P{n}M) dan nama (<n> Bulan) selama belum disentuh. */
  function ubahBulanBaru(e) {
    const bersih = digitSaja(e, 3)
    fBulan = bersih
    const bulan = Number(bersih)
    if (!kodeManual) fKode = bersih && bulan >= 1 ? `P${bulan}M` : ''
    if (!namaManual) fNama = bersih && bulan >= 1 ? `${bulan} Bulan` : ''
  }

  function ubahKodeBaru(e) {
    kodeManual = true
    fKode = e.currentTarget.value.toUpperCase().replace(/\s+/g, '')
  }

  function ubahNamaBaru(e) {
    namaManual = true
    fNama = e.currentTarget.value
  }

  function ubahHargaBaru(e) {
    fHarga = digitSaja(e, MAKS_DIGIT_HARGA)
  }

  async function buatPaket(e) {
    e.preventDefault()
    if (!fSah) return

    const bulan = Number(fBulan)
    const body = {
      code: fKode.trim(),
      label: fNama.trim() || `${bulan} Bulan`,
      months: bulan,
      price: Number(fHarga),
      note: fCatatan.trim()
    }

    fSibuk = true
    fGalat = ''
    try {
      const res = await adminCreatePlan(body)
      const kode = res && res.plan ? res.plan.code : body.code
      tutupForm()
      suksesUmum = `Paket ${kode} ditambahkan.`
      await onchanged()
    } catch (err) {
      if (err.status === 401) {
        on401()
        return
      }
      fGalat = galatAksi(err, 'Paket baru gagal ditambahkan.')
    } finally {
      fSibuk = false
    }
  }
</script>

<div class="card mt-3">
  <div class="card-head bungkus">
    <i class="fa fa-tags"></i>
    <h3>Paket &amp; Harga</h3>
    <span class="tiny muted">Harga inilah yang dilihat pelanggan di halaman pembayaran.</span>
    <div class="grow"></div>
    <span class="badge neutral">{plans.length} paket</span>
    <button class="btn btn-primary btn-sm" onclick={tombolForm}>
      {#if bukaForm}
        <i class="fa fa-times"></i> Tutup form
      {:else}
        <i class="fa fa-plus"></i> Tambah paket
      {/if}
    </button>
  </div>

  {#if bukaForm}
    <!-- ----------------------------------------------------- form paket baru --- -->
    <form class="formpanel panel-masuk" onsubmit={buatPaket}>
      <div class="tiny muted">PAKET BARU</div>
      <h3>Tambah paket harga</h3>

      <div class="grid cols-2 mt-2">
        <div>
          <label class="flabel" for="ph-bulan">Durasi (bulan)</label>
          <input
            id="ph-bulan"
            class="input"
            inputmode="numeric"
            value={fBulan}
            oninput={ubahBulanBaru}
            placeholder="mis. 2"
            autocomplete="off"
          />
          <div class="tiny muted mt-1">Menentukan lama langganan yang dibeli pelanggan.</div>
        </div>

        <div>
          <label class="flabel" for="ph-kode">Kode paket</label>
          <input
            id="ph-kode"
            class="input mono"
            value={fKode}
            oninput={ubahKodeBaru}
            placeholder="P2M"
            maxlength="12"
            spellcheck="false"
            autocomplete="off"
          />
          <div class="tiny muted mt-1">
            Terisi sendiri dari durasi (<span class="mono">P2M</span> untuk 2 bulan), boleh ditimpa.
          </div>
        </div>

        <div>
          <label class="flabel" for="ph-nama">Nama paket</label>
          <input
            id="ph-nama"
            class="input"
            value={fNama}
            oninput={ubahNamaBaru}
            placeholder="2 Bulan"
            maxlength={MAKS_NAMA}
          />
          <div class="tiny muted mt-1">Nama ini yang dibaca pelanggan di halaman pembayaran.</div>
        </div>

        <div>
          <label class="flabel" for="ph-harga">Harga (Rp)</label>
          <input
            id="ph-harga"
            class="input"
            inputmode="numeric"
            value={fHarga}
            oninput={ubahHargaBaru}
            placeholder="mis. 100000"
            autocomplete="off"
          />
          <div class="tiny muted mt-1">Angka saja; titik pemisah tidak perlu.</div>
        </div>

        <div>
          <label class="flabel" for="ph-catatan">Catatan <span class="muted">(opsional)</span></label>
          <input
            id="ph-catatan"
            class="input"
            value={fCatatan}
            oninput={(e) => (fCatatan = e.currentTarget.value)}
            placeholder="mis. sudah termasuk PPN"
            maxlength={MAKS_CATATAN}
          />
          <div class="tiny muted mt-1">
            Cuma keterangan untuk Anda sendiri, tidak ditampilkan ke pelanggan.
          </div>
        </div>
      </div>

      {#if fGalat}
        <div class="box solid-danger mt-3" role="alert">
          <i class="fa fa-exclamation-triangle"></i>
          {fGalat}
        </div>
      {/if}

      <div class="row wrap mt-3">
        <button class="btn btn-primary" type="submit" disabled={!fSah}>
          {#if fSibuk}
            <i class="fa fa-spinner"></i> Menyimpan…
          {:else}
            <i class="fa fa-save"></i> Simpan paket
          {/if}
        </button>
        <button class="btn btn-ghost" type="button" onclick={tutupForm}>Batal</button>
        <div class="grow"></div>
        <span class="tiny muted">
          {#if fKode.trim() === ''}
            Kode paket wajib diisi.
          {:else if !fSahBulan}
            Durasi 1–{MAKS_BULAN} bulan.
          {:else if !fSahHarga}
            Harga harus lebih dari 0.
          {:else}
            Kode paket dipakai di tagihan dan tidak bisa diubah setelah dibuat.
          {/if}
        </span>
      </div>
    </form>
  {/if}

  {#if suksesUmum}
    <div class="card-body tight">
      <div class="box solid-success" role="status">
        <i class="fa fa-check-circle"></i>
        {suksesUmum}
        <button
          class="btn btn-ghost btn-sm"
          style="float:right;margin-top:-3px"
          onclick={() => (suksesUmum = '')}>tutup</button
        >
      </div>
    </div>
  {/if}

  {#if planMemuat && plans.length === 0}
    <div class="card-body">
      <div class="muted small">Memuat paket…</div>
    </div>
  {:else if plans.length === 0}
    <div class="card-body">
      {#if planError}
        <div class="box solid-danger" role="alert">
          <i class="fa fa-exclamation-triangle"></i>
          {planError}
        </div>
        <button class="btn btn-ghost btn-sm mt-2" onclick={onmuatulang}>
          <i class="fa fa-rotate-right"></i> Coba muat lagi
        </button>
      {:else}
        <div class="box info">
          <i class="fa fa-tags"></i>
          Belum ada paket harga. Tanpa paket, pelanggan tidak bisa memilih lama langganan di
          halaman pembayarannya.
        </div>
        {#if !bukaForm}
          <button class="btn btn-primary mt-2" onclick={tombolForm}>
            <i class="fa fa-plus"></i> Tambah paket
          </button>
        {/if}
      {/if}
    </div>
  {:else}
    <div class="tablewrap">
      <table class="table">
        <thead>
          <tr>
            <th style="width:70px">Kode</th>
            <th style="width:190px">Nama paket</th>
            <th style="width:105px">Durasi (bulan)</th>
            <th style="width:170px">Harga</th>
            <th>Catatan</th>
            <th class="right" style="width:185px">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each baris as r, i (r.kode)}
            <tr class="baris-masuk" style="animation-delay:{(i > 8 ? 8 : i) * 28}ms">
              <td>
                <span class="mono tiny">{r.kode}</span>
              </td>

              <td>
                <input
                  class="input"
                  aria-label="Nama paket {r.kode}"
                  maxlength={MAKS_NAMA}
                  value={r.label}
                  oninput={(e) => ubah(r.kode, 'label', e.currentTarget.value)}
                  disabled={r.sibuk}
                />
                <div class="tiny muted mt-1">
                  {#if r.pelanggan > 0}
                    dipakai {r.pelanggan} pelanggan
                  {:else}
                    belum dipakai
                  {/if}
                </div>
              </td>

              <td>
                <input
                  class="input"
                  inputmode="numeric"
                  aria-label="Durasi paket {r.kode} dalam bulan"
                  value={r.bulan}
                  oninput={(e) => ubahBulan(e, r.kode)}
                  disabled={r.sibuk}
                  autocomplete="off"
                />
                {#if !r.sahBulan}
                  <div class="tiny cl-danger mt-1">Durasi 1–{MAKS_BULAN} bulan.</div>
                {/if}
              </td>

              <td>
                {#if fokus === r.kode}
                  <input
                    class="input mono"
                    inputmode="numeric"
                    aria-label="Harga paket {r.kode} dalam rupiah"
                    value={r.harga}
                    oninput={(e) => ubahHarga(e, r.kode)}
                    onblur={() => tutupHarga(r.kode)}
                    onkeydown={(e) => {
                      if (e.key === 'Escape' || e.key === 'Enter') tutupHarga(r.kode)
                    }}
                    use:mulaiFokus
                    disabled={r.sibuk}
                    autocomplete="off"
                  />
                {:else}
                  <button
                    class="input harga"
                    type="button"
                    title="Klik untuk mengubah harga"
                    aria-label="Harga paket {r.kode}: {teksHarga(r.harga)}. Ubah harga"
                    onclick={() => bukaHarga(r.kode)}
                    disabled={r.sibuk}
                  >
                    {#if r.harga === ''}
                      <span class="muted">Belum diisi</span>
                    {:else}
                      {fmtRupiah(Number(r.harga))}
                    {/if}
                  </button>
                {/if}
                {#if !r.sahHarga}
                  <div class="tiny cl-danger mt-1">
                    {#if r.harga === ''}
                      Harga belum diisi.
                    {:else}
                      Harga harus lebih dari 0.
                    {/if}
                  </div>
                {/if}
              </td>

              <td>
                <input
                  class="input"
                  aria-label="Catatan paket {r.kode}"
                  maxlength={MAKS_CATATAN}
                  value={r.catatan}
                  oninput={(e) => ubah(r.kode, 'catatan', e.currentTarget.value)}
                  disabled={r.sibuk}
                />
              </td>

              <td class="right nowrap">
                <div class="aksi">
                  <button
                    class="btn btn-primary btn-sm simpan"
                    disabled={!r.kotor || !r.sahBulan || !r.sahHarga || r.sibuk}
                    onclick={() => simpan(r)}
                  >
                    {#if r.sibuk}
                      <i class="fa fa-spinner"></i> Menyimpan…
                    {:else}
                      <i class="fa fa-save"></i> Simpan
                    {/if}
                  </button>
                  <button
                    class="btn btn-danger btn-sm"
                    aria-label="Hapus paket {r.kode}"
                    title="Hapus paket"
                    disabled={r.sibuk}
                    onclick={() => hapus(r)}
                  >
                    <i class="fa fa-trash"></i>
                  </button>
                </div>
              </td>
            </tr>

            {#if r.galat || r.sukses}
              <tr class="subrow">
                <td colspan="6">
                  {#if r.galat}
                    <div class="box solid-danger" role="alert">
                      <i class="fa fa-exclamation-triangle"></i>
                      {r.galat}
                    </div>
                  {:else}
                    <div class="box solid-success" role="status">
                      <i class="fa fa-check-circle"></i>
                      {r.sukses}
                    </div>
                  {/if}
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  /* Tombol harga dibuat sama bentuknya dengan kotak .input supaya barisnya
     tidak bergeser saat kolom harganya dibuka. */
  .harga {
    display: block;
    text-align: left;
    cursor: text;
    line-height: 1.15;
  }
  .aksi .simpan {
    min-width: 104px;
  }
</style>

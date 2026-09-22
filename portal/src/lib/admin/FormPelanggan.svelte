<script>
  import { adminCreateCustomer, adminUpdateCustomer } from '../api.js'
  import { fmtRupiah, pesanGagal, prettyId, stateLabel, subdomainDari, waNomor } from '../format.js'
  import { salinTeks } from '../clipboard.js'

  /*
   * Domain panel (<nama>.<domain>) diturunkan dari alamat portal yang sedang
   * dibuka, bukan ditulis di kode: portal ini dilayani di control.nocify.id
   * waktu produksi dan di alamat lain waktu diuji. Satu label pertama dibuang,
   * jadi control.nocify.id menjadi nocify.id. Kalau nanti domain panel
   * dipisah dari domain portal, ganti HANYA di sini.
   */
  const DOMAIN_PANEL = (() => {
    const host = typeof location !== 'undefined' ? location.hostname : ''
    const titik = host.indexOf('.')
    return titik === -1 ? host : host.slice(titik + 1)
  })()

  /*
   * Form tambah/ubah pelanggan.
   *
   * Dipakai dua mode: "baru" (POST, lalu menampilkan panel hasil berisi
   * tautan panel dan tautan pembayaran) dan "edit" (PATCH, langsung ditutup
   * setelah tersimpan). Subdomain diisi otomatis dari nama usaha sampai
   * pemakainya menyentuh kolom itu sendiri.
   */
  let {
    instances = [],
    plans = [],
    awal = null,
    instError = '',
    onsimpan = () => {},
    ontutup = () => {},
    on401 = () => {},
    onbuatpanel = () => {},
    onmuatulang = () => {}
  } = $props()

  /*
   * Komponen selalu dipasang ulang lewat {#key} setiap kali form dibuka, jadi
   * nilai awal cukup disalin sekali di sini — tidak ada isian yang perlu ikut
   * berubah di tengah pengisian.
   */
  function nilaiAwal() {
    return {
      edit: awal != null,
      name: awal ? awal.name : '',
      institution: awal ? awal.institution : '',
      wa: awal ? awal.wa : '',
      session: awal ? awal.session_name : '',
      instance: awal ? awal.instance_id : ''
    }
  }
  const mula = nilaiAwal()
  const edit = mula.edit

  let name = $state(mula.name)
  let institution = $state(mula.institution)
  let wa = $state(mula.wa)
  let session = $state(mula.session)
  let manual = $state(edit) // subdomain sudah diisi sendiri, jangan ditimpa
  let instanceID = $state(mula.instance)
  let planCode = $state('')

  /* Daftar panel baru tiba setelah halaman dimuat, jadi pilihan awal diisi di sini. */
  $effect(() => {
    if (!instanceID && instances.length > 0) instanceID = instances[0].id
  })


  let busy = $state(false)
  let galat = $state('')
  let hasil = $state(null) // pelanggan hasil POST
  let tersalin = $state('')
  let timer = 0

  let sub = $derived(subdomainDari(session))
  let rapi = $derived(session !== sub)
  let panel = $derived(instances.find((i) => i.id === instanceID) ?? null)
  let paket = $derived(plans.find((p) => p.code === planCode) ?? null)
  let subBerubah = $derived(edit && sub !== awal.session_name)
  let panelBerubah = $derived(edit && instanceID !== awal.instance_id)
  let lengkap = $derived(name.trim() !== '' && institution.trim() !== '' && sub !== '' && instanceID !== '')
  let bisaSimpan = $derived(lengkap && !busy)

  let teksWA = $derived.by(() => {
    if (!hasil) return ''
    return (
      `Halo ${hasil.name}, panel Mikhmon untuk ${hasil.institution} sudah disiapkan.\n\n` +
      `Alamat panel : https://${hasil.session_name}.${DOMAIN_PANEL}\n` +
      `Sesi panel   : ${hasil.session_name}\n` +
      `Tautan bayar : ${hasil.pay_url}\n\n` +
      `Tautan itu berisi QRIS untuk mengaktifkan langganan. Hubungi kami kalau ada yang perlu dibantu.`
    )
  })
  let waTujuan = $derived(hasil ? waNomor(hasil.wa) : '')
  let waHref = $derived(`https://wa.me/${waTujuan}?text=${encodeURIComponent(teksWA)}`)
  let panelHasil = $derived(
    hasil ? (instances.find((i) => i.id === hasil.instance_id) ?? null) : null
  )

  function ubahUsaha(e) {
    institution = e.currentTarget.value
    if (!manual) session = subdomainDari(institution)
  }

  function ubahSub(e) {
    manual = true
    session = e.currentTarget.value
  }

  function reset() {
    name = ''
    institution = ''
    wa = ''
    session = ''
    manual = false
    planCode = ''
    hasil = null
    galat = ''
  }

  async function simpan(e) {
    e.preventDefault()
    if (!bisaSimpan) return

    busy = true
    galat = ''
    const isi = {
      name: name.trim(),
      institution: institution.trim(),
      wa: wa.trim(),
      session_name: sub,
      instance_id: instanceID
    }

    try {
      if (edit) {
        const res = await adminUpdateCustomer(awal.id, isi)
        onsimpan(res.customer, 'edit')
        ontutup()
      } else {
        const res = await adminCreateCustomer({ ...isi, plan_code: planCode })
        hasil = res.customer
        onsimpan(res.customer, 'baru')
      }
    } catch (err) {
      if (err.status === 401) {
        on401()
        return
      }
      galat = pesanGagal(err, edit ? 'Perubahan gagal disimpan.' : 'Pelanggan gagal dibuat.')
    } finally {
      busy = false
    }
  }

  async function salin(teks, tanda) {
    const ok = await salinTeks(teks)
    if (!ok) {
      galat = 'Tidak bisa menyalin otomatis. Salin teksnya secara manual.'
      return
    }
    galat = ''
    tersalin = tanda
    clearTimeout(timer)
    timer = setTimeout(() => (tersalin = ''), 2000)
  }
</script>

{#if hasil}
  <!-- ------------------------------------------------------ hasil tambah --- -->
  <div class="formpanel panel-masuk">
    <div class="row between wrap">
      <div>
        <div class="tiny muted">PELANGGAN BARU</div>
        <h2>{hasil.institution}</h2>
        <div class="small muted">
          {hasil.name}
          {#if panelHasil}&middot; panel {panelHasil.name}{/if}
        </div>
      </div>
      <span class="badge {hasil.status}">{stateLabel[hasil.status] ?? hasil.status}</span>
    </div>

    <div class="grid cols-2 mt-3">
      <div>
        <div class="tiny muted">ALAMAT PANEL</div>
        <div class="mono" style="font-size:15px">https://{hasil.session_name}.{DOMAIN_PANEL}</div>
        <div class="row wrap mt-2">
          <button class="btn btn-ghost btn-sm" onclick={() => salin(`https://${hasil.session_name}.${DOMAIN_PANEL}`, 'panel')}>
            <i class="fa {tersalin === 'panel' ? 'fa-check' : 'fa-copy'}"></i>
            {#if tersalin === 'panel'}Tersalin{:else}Salin alamat{/if}
          </button>
          <span class="tiny muted">Sesi di panel harus bernama <b class="mono">{hasil.session_name}</b>.</span>
        </div>
      </div>

      <div>
        <div class="tiny muted">TAUTAN PEMBAYARAN</div>
        <div class="mono tiny" style="word-break:break-all">{hasil.pay_url}</div>
        <div class="row wrap mt-2">
          <button class="btn btn-ghost btn-sm" onclick={() => salin(hasil.pay_url, 'bayar')}>
            <i class="fa {tersalin === 'bayar' ? 'fa-check' : 'fa-copy'}"></i>
            {#if tersalin === 'bayar'}Tersalin{:else}Salin tautan{/if}
          </button>
          <a class="btn btn-success btn-sm" href={waHref} target="_blank" rel="noopener">
            <i class="fa fa-whatsapp"></i> Kirim via WhatsApp
          </a>
        </div>
        {#if !waTujuan}
          <div class="tiny muted mt-1">
            Nomor WhatsApp belum diisi, jadi kontaknya dipilih di aplikasi WhatsApp.
          </div>
        {/if}
      </div>
    </div>

    {#if hasil.status === 'expired'}
      <div class="box solid-warning mt-3">
        <i class="fa fa-info-circle"></i>
        Pelanggan ini belum punya masa berlaku. Panelnya terkunci sampai paket dibayar, atau
        sampai Anda menekan <b>Perpanjang</b> di tabel pelanggan.
      </div>
    {:else}
      <div class="box solid-success mt-3">
        <i class="fa fa-check-circle"></i>
        Paket {hasil.plan_label} langsung aktif. Sisa {hasil.days_left} hari dari hari ini.
      </div>
    {/if}

    {#if galat}
      <div class="box danger mt-3">
        <i class="fa fa-exclamation-triangle"></i>
        {galat}
      </div>
    {/if}

    <div class="row wrap mt-3">
      <button class="btn btn-primary" onclick={reset}><i class="fa fa-plus"></i> Tambah lagi</button>
      <button class="btn btn-ghost" onclick={ontutup}>Tutup</button>
      <div class="grow"></div>
      <span class="tiny muted">
        Pelanggan sudah masuk tabel. Tautan pembayaran tetap bisa disalin dari baris tabelnya.
      </span>
    </div>
  </div>
{:else}
  <!-- -------------------------------------------------- form tambah/edit --- -->
  <form class="formpanel panel-masuk" onsubmit={simpan}>
    <div class="row between wrap">
      <div>
        <div class="tiny muted">{edit ? 'UBAH DATA PELANGGAN' : 'PELANGGAN BARU'}</div>
        <h3>{edit ? awal.institution : 'Tambah pelanggan'}</h3>
        {#if edit}
          <div class="tiny muted">
            ID instalasi <span class="mono">{prettyId(awal.instance_id)}</span> &middot; sesi panel
            <span class="mono">{awal.session_name}</span>
          </div>
        {/if}
      </div>
      <button class="btn btn-ghost btn-sm" type="button" onclick={ontutup}>
        <i class="fa fa-times"></i> Batal
      </button>
    </div>

    {#if instances.length === 0}
      {#if instError}
        <div class="box solid-danger mt-3">
          <i class="fa fa-exclamation-triangle"></i>
          {instError}
          <div class="row wrap mt-2">
            <button class="btn btn-ghost btn-sm" type="button" onclick={onmuatulang}>Coba muat lagi</button>
          </div>
        </div>
      {:else}
        <div class="box solid-warning mt-3">
          <i class="fa fa-info-circle"></i>
          Belum ada panel terpasang, jadi pelanggan belum bisa ditambahkan. Buat panelnya dulu,
          lalu tempelkan berkas <span class="mono">instance.php</span> di VPS panel.
          <div class="row wrap mt-2">
            <button class="btn btn-primary btn-sm" type="button" onclick={onbuatpanel}>
              <i class="fa fa-plus"></i> Tambah panel
            </button>
          </div>
        </div>
      {/if}
    {/if}

    <div class="grid cols-2 mt-3">
      <div>
        <label class="flabel" for="fp-nama">Nama pelanggan</label>
        <input
          id="fp-nama"
          class="input"
          bind:value={name}
          placeholder="mis. Taufiq"
          maxlength="60"
          required
        />
        <div class="tiny muted mt-1">Nama orang yang menghubungi Anda.</div>
      </div>

      <div>
        <label class="flabel" for="fp-usaha">Nama usaha</label>
        <input
          id="fp-usaha"
          class="input"
          value={institution}
          oninput={ubahUsaha}
          placeholder="mis. Taufiq.net"
          maxlength="60"
          required
        />
        <div class="tiny muted mt-1">Dipakai juga untuk mengisi subdomain di bawah.</div>
      </div>

      <div>
        <label class="flabel" for="fp-wa">WhatsApp <span class="muted">(opsional)</span></label>
        <input id="fp-wa" class="input" type="tel" bind:value={wa} placeholder="mis. 0851 3949 5106" />
        <div class="tiny muted mt-1">Tujuan tombol kirim tautan pembayaran.</div>
      </div>

      <div>
        <label class="flabel" for="fp-sub">Subdomain</label>
        <input
          id="fp-sub"
          class="input mono"
          value={session}
          oninput={ubahSub}
          placeholder="mis. taufiq-net"
          maxlength="60"
          spellcheck="false"
          required
        />
        <div class="tiny mt-1">
          {#if sub}
            <span class="mono cl-primary">{sub}</span><span class="muted mono">.{DOMAIN_PANEL}</span>
          {:else}
            <span class="muted">Isi subdomain untuk melihat alamat panelnya.</span>
          {/if}
        </div>
        {#if rapi && sub}
          <div class="tiny muted">
            Disimpan sebagai <span class="mono">{sub}</span> &mdash; spasi dan tanda baca jadi tanda hubung.
          </div>
        {:else if session && !sub}
          <div class="tiny cl-danger">
            Subdomain belum sah. Pakai huruf kecil, angka, dan tanda hubung.
          </div>
        {/if}
      </div>

      <div>
        <label class="flabel" for="fp-panel">Panel</label>
        <select id="fp-panel" class="input" bind:value={instanceID} disabled={instances.length === 0}>
          {#if instances.length === 0}
            <option value="">Belum ada panel</option>
          {/if}
          {#each instances as ins (ins.id)}
            <option value={ins.id}>{ins.name} &middot; {prettyId(ins.id)}</option>
          {/each}
        </select>
        <div class="tiny muted mt-1">
          {#if panel}
            Panel ini sekarang menangani <b>{panel.customers}</b> pelanggan.
            {#if panel.customers === 0}Belum ada sesi di dalamnya.{/if}
          {:else}
            Pilih panel tempat sesi pelanggan ini dibuat.
          {/if}
        </div>
      </div>

      <div>
        {#if edit}
          <div class="flabel">Paket berjalan</div>
          <div class="input" style="border-style:dashed">
            {awal.plan_label} &middot; sisa {awal.days_left} hari
          </div>
          <div class="tiny muted mt-1">
            Paket tidak ikut berubah dari sini. Pakai <b>Perpanjang</b> di tabel pelanggan.
          </div>
        {:else}
          <label class="flabel" for="fp-paket">Paket awal</label>
          <select id="fp-paket" class="input" bind:value={planCode}>
            <option value="">Belum aktif &mdash; tanpa masa berlaku</option>
            {#each plans as p (p.code)}
              <option value={p.code}>
                {p.label} &mdash; langsung aktif {p.months} bulan &middot; {fmtRupiah(p.price)}
              </option>
            {/each}
          </select>
          <div class="tiny muted mt-1">
            {#if paket}
              Masa berlaku mulai hari ini, {paket.months} bulan ke depan.
            {:else}
              Panel pelanggan tetap terkunci sampai paket dibayar.
            {/if}
          </div>
        {/if}
      </div>
    </div>

    {#if subBerubah}
      <div class="box solid-warning mt-3">
        <i class="fa fa-exclamation-triangle"></i>
        Subdomain berubah dari <span class="mono">{awal.session_name}</span> jadi
        <span class="mono">{sub}</span>. Sesi di panel harus diganti namanya jadi
        <span class="mono">{sub}</span> juga &mdash; kalau tidak, panel tidak akan melapor ke portal
        ini. Perubahan tetap bisa disimpan sekarang.
      </div>
    {/if}

    {#if panelBerubah}
      <div class="box solid-warning mt-3">
        <i class="fa fa-exclamation-triangle"></i>
        Panel pelanggan ini dipindah ke {panel ? panel.name : 'panel lain'}. Buat sesi bernama
        <span class="mono">{sub}</span> di panel yang baru, kalau tidak tidak ada yang melapor
        ke portal.
      </div>
    {/if}

    {#if galat}
      <div class="box solid-danger mt-3">
        <i class="fa fa-exclamation-triangle"></i>
        {galat}
      </div>
    {/if}

    <div class="row wrap mt-3">
      <button class="btn btn-primary" type="submit" disabled={!bisaSimpan}>
        {#if busy}
          <i class="fa fa-spinner"></i> Menyimpan…
        {:else}
          <i class="fa fa-save"></i> {edit ? 'Simpan perubahan' : 'Simpan pelanggan'}
        {/if}
      </button>
      <button class="btn btn-ghost" type="button" onclick={ontutup}>Batal</button>
      <div class="grow"></div>
      {#if !lengkap}
        <span class="tiny muted">Nama, nama usaha, dan subdomain wajib diisi.</span>
      {/if}
    </div>
  </form>
{/if}

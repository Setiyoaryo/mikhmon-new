<script>
  import Login from './Login.svelte'
  import FormPelanggan from '../admin/FormPelanggan.svelte'
  import PanelDeploy from '../admin/PanelDeploy.svelte'
  import { salinTeks } from '../clipboard.js'
  import {
    adminOverview,
    adminMe,
    adminLogout,
    adminClaim,
    adminCustomer,
    adminPlans,
    adminInstances,
    adminExtend,
    adminDeleteCustomer
  } from '../api.js'
  import { fmtRupiah, fmtDate, prettyId, pesanGagal, stateLabel } from '../format.js'

  let authed = $state(null) // null = belum diketahui, true/false = hasil pemeriksaan
  let user = $state('admin')
  let data = $state(null)
  let loading = $state(true)
  let error = $state('')
  let busyId = $state('')
  let flash = $state('')
  let q = $state('')

  /* Data pendukung pengelolaan pelanggan. */
  let plans = $state([])
  let instances = $state([])
  let instError = $state('')
  let form = $state(null) // {mode: 'baru'|'edit', row, key}
  let formSeq = 0
  let formPanel = $state(false) // form "Tambah panel" di bagian bawah
  let extendFor = $state('') // id pelanggan yang pemilih perpanjangannya terbuka
  let copied = $state('') // id pelanggan yang tautannya baru disalin
  let copyTimer = 0

  let stats = $derived(data ? data.stats : { revenue_month: 0, customers: 0, active: 0, warning: 0, expired: 0 })
  let claims = $derived(data ? data.claims : [])
  let total = $derived(data ? data.customers.length : 0)

  /* Pencarian tetap di sisi klien, atas data pelanggan yang sudah diambil. */
  let filtered = $derived(
    (data ? data.customers : []).filter((r) => {
      const s = q.trim().toLowerCase()
      if (!s) return true
      return (
        (r.name || '').toLowerCase().includes(s) ||
        (r.institution || '').toLowerCase().includes(s) ||
        (r.instance_id || '').toLowerCase().includes(s)
      )
    })
  )

  $effect(() => {
    boot()
  })

  async function boot() {
    loading = true
    error = ''
    try {
      data = await adminOverview()
      authed = true
      flash = ''
      await muatReferensi()
      try {
        const me = await adminMe()
        if (me && me.user) user = me.user
      } catch {
        /* nama user cuma pemanis, abaikan kalau gagal */
      }
    } catch (e) {
      data = null
      if (e.status === 401) authed = false
      else error = pesanGagal(e, 'Gagal memuat data admin.')
    } finally {
      loading = false
    }
  }

  async function refresh() {
    try {
      data = await adminOverview()
      authed = true
      error = ''
      await muatReferensi()
    } catch (e) {
      if (e.status === 401) authed = false
      else error = pesanGagal(e, 'Gagal memuat ulang data admin.')
    }
  }

  /* Daftar panel dan paket dipakai form pelanggan; gagalnya tidak mematikan halaman. */
  async function muatReferensi() {
    try {
      const [p, i] = await Promise.all([adminPlans(), adminInstances()])
      plans = p.plans || []
      instances = i.instances || []
      instError = ''
    } catch (e) {
      if (e.status === 401) {
        authed = false
        return
      }
      instError = pesanGagal(e, 'Gagal memuat daftar panel dan paket.')
    }
  }

  function gagal(e) {
    if (e.status === 401) {
      authed = false
      return
    }
    flash = ''
    error = pesanGagal(e, 'Aksi gagal dijalankan. Coba lagi.')
  }

  function sesiHabis() {
    authed = false
  }

  async function approve(c) {
    busyId = c.id
    try {
      const res = await adminClaim(c.id, 'approve')
      const lanjut =
        res && res.expires_at
          ? ` Paket aktif sampai ${fmtDate(res.expires_at)}.`
          : ' Panel pelanggannya otomatis menerima status aktif pada heartbeat berikutnya.'
      flash = `Pembayaran ${c.customer} (${fmtRupiah(c.amount)}) disetujui.${lanjut}`
      await refresh()
    } catch (e) {
      gagal(e)
    } finally {
      busyId = ''
    }
  }

  async function reject(c) {
    busyId = c.id
    try {
      await adminClaim(c.id, 'reject')
      flash = `Klaim ${c.customer} ditolak. Tandai sudah dibaca.`
      await refresh()
    } catch (e) {
      gagal(e)
    } finally {
      busyId = ''
    }
  }

  async function toggle(r) {
    busyId = r.id
    const suspend = r.status !== 'expired'
    try {
      await adminCustomer(r.id, suspend ? 'suspend' : 'activate')
      flash = suspend
        ? `${r.institution} ditangguhkan. Panel-nya terkunci di heartbeat berikutnya.`
        : `${r.institution} diaktifkan kembali.`
      await refresh()
    } catch (e) {
      gagal(e)
    } finally {
      busyId = ''
    }
  }

  /* ------------------------------------------------------ kelola pelanggan -- */

  function tambah() {
    extendFor = ''
    form = { mode: 'baru', row: null, key: ++formSeq }
  }

  function edit(r) {
    extendFor = ''
    form = { mode: 'edit', row: r, key: ++formSeq }
  }

  function tutupForm() {
    form = null
  }

  async function simpanPelanggan(c, mode) {
    error = ''
    flash =
      mode === 'baru'
        ? `Pelanggan ${c.institution} dibuat. Sesi panelnya bernama ${c.session_name}.`
        : `Data ${c.institution} diperbarui. Sesi di panel harus bernama ${c.session_name}.`
    await refresh()
  }

  async function perpanjang(r, bulan) {
    busyId = r.id
    try {
      const res = await adminExtend(r.id, bulan)
      extendFor = ''
      flash = `Langganan ${r.institution} diperpanjang ${bulan} bulan, sampai ${fmtDate(res.expires_at)}.`
      await refresh()
    } catch (e) {
      gagal(e)
    } finally {
      busyId = ''
    }
  }

  async function hapus(r) {
    const lanjut = confirm(
      `Hapus pelanggan ${r.institution} (sesi ${r.session_name})? Tautan pembayarannya ikut hilang dan tidak bisa dikembalikan.`
    )
    if (!lanjut) return

    busyId = r.id
    try {
      await adminDeleteCustomer(r.id)
      if (form && form.row && form.row.id === r.id) form = null
      flash = `Pelanggan ${r.institution} dihapus.`
      await refresh()
    } catch (e) {
      gagal(e)
    } finally {
      busyId = ''
    }
  }

  async function salinTautan(r) {
    const ok = await salinTeks(r.pay_url)
    if (!ok) {
      flash = ''
      error = `Tautan tidak bisa disalin otomatis. Salin manual: ${r.pay_url}`
      return
    }
    error = ''
    copied = r.id
    flash = `Tautan pembayaran ${r.institution} disalin.`
    clearTimeout(copyTimer)
    copyTimer = setTimeout(() => (copied = ''), 2000)
  }

  /* Dari form pelanggan yang belum punya panel: buka form panel di bawah. */
  function kePanel() {
    formPanel = true
    const el = document.getElementById('panel-terpasang')
    if (!el) return
    const kurangi = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    el.scrollIntoView({ behavior: kurangi ? 'auto' : 'smooth', block: 'start' })
  }

  async function panelBaru(inst) {
    instances = instances.concat([inst])
    error = ''
    flash = `${inst.name} direkam. Salin berkas include/instance.php ke VPS panel.`
    formPanel = false
  }

  async function logout() {
    try {
      await adminLogout()
    } catch {
      /* walau server menolak, portal tetap kembali ke form login */
    }
    data = null
    flash = ''
    error = ''
    q = ''
    form = null
    extendFor = ''
    authed = false
  }

  function sisa(d) {
    if (d < 0) return `lewat ${Math.abs(d)} hari`
    if (d === 0) return 'berakhir hari ini'
    return `${d} hari`
  }
</script>

<div class="topbar">
  <div class="brand">
    <img src="./logo-nocify-on-dark.png" alt="NOCIFY" class="logo" />
    <span class="sub">Billing Mikhmon</span>
  </div>
  <div class="grow"></div>
  {#if authed}
    <span class="tiny muted nowrap">Masuk sebagai <b style="color:var(--text)">{user}</b> (admin)</span>
    <button class="btn btn-ghost btn-sm" onclick={logout}><i class="fa fa-sign-out"></i> Keluar</button>
  {/if}
</div>

<div class="wrap">
  {#if authed === false}
    <Login onlogin={boot} />
  {:else if loading && !data}
    <div class="card">
      <div class="card-body">
        <div class="muted small">Memuat…</div>
      </div>
    </div>
  {:else if error && !data}
    <div class="card">
      <div class="card-body">
        <div class="box danger">
          <i class="fa fa-exclamation-triangle"></i>
          {error}
        </div>
        <button class="btn btn-ghost mt-3" onclick={boot}>
          <i class="fa fa-rotate-right"></i> Coba lagi
        </button>
      </div>
    </div>
  {:else if data}
    {#if flash}
      <div class="box solid-success mb-2">
        <i class="fa fa-check-circle"></i>
        {flash}
        <button
          class="btn btn-ghost btn-sm"
          style="float:right;margin-top:-3px"
          onclick={() => (flash = '')}>tutup</button
        >
      </div>
    {/if}

    {#if error}
      <div class="box danger mb-2">
        <i class="fa fa-exclamation-triangle"></i>
        {error}
        <button
          class="btn btn-ghost btn-sm"
          style="float:right;margin-top:-3px"
          onclick={() => (error = '')}>tutup</button
        >
      </div>
    {/if}

    <!-- ----------------------------------------------------------- stat --- -->
    <div class="grid cols-4">
      <div class="stat is-money">
        <div class="label">Pendapatan bulan ini</div>
        <div class="value">{fmtRupiah(stats.revenue_month)}</div>
        <div class="hint">{stats.customers} pelanggan berlangganan</div>
      </div>
      <div class="stat is-active">
        <div class="label">Aktif</div>
        <div class="value">{stats.active}</div>
        <div class="hint">panel berjalan normal</div>
      </div>
      <div class="stat is-warning">
        <div class="label">Berakhir &le; 7 hari</div>
        <div class="value">{stats.warning}</div>
        <div class="hint">waktunya menagih</div>
      </div>
      <div class="stat is-expired">
        <div class="label">Kedaluwarsa</div>
        <div class="value">{stats.expired}</div>
        <div class="hint">panel terkunci</div>
      </div>
    </div>

    <!-- ------------------------------------------------------ perlu aksi --- -->
    <div class="card mt-3">
      <div class="card-head">
        <i class="fa fa-bell"></i>
        <h3>Perlu tindakan</h3>
        <div class="grow"></div>
        <span class="badge pending">{claims.length} klaim pembayaran</span>
      </div>

      {#if claims.length === 0}
        <div class="card-body">
          <div class="box success">
            <i class="fa fa-check"></i> Semua klaim sudah diproses.
          </div>
        </div>
      {:else}
        <table class="table">
          <thead>
            <tr>
              <th>Pelanggan</th>
              <th>Paket</th>
              <th>Nominal</th>
              <th>Referensi</th>
              <th>Dikirim</th>
              <th class="right">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each claims as c (c.id)}
              <tr>
                <td>
                  <div class="name">{c.customer}</div>
                  <div class="tiny muted">{c.institution}</div>
                </td>
                <td>{c.plan_label}</td>
                <td><b>{fmtRupiah(c.amount)}</b></td>
                <td class="mono tiny">{c.ref}</td>
                <td class="muted small nowrap">{c.age}</td>
                <td class="right nowrap">
                  <button class="btn btn-success btn-sm" onclick={() => approve(c)} disabled={busyId === c.id}>
                    <i class="fa fa-check"></i> Setujui
                  </button>
                  <button class="btn btn-danger btn-sm" onclick={() => reject(c)} disabled={busyId === c.id}>Tolak</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>

    <!-- -------------------------------------------------------- pelanggan -- -->
    <div class="card mt-3">
      <div class="card-head bungkus">
        <i class="fa fa-users"></i>
        <h3>Pelanggan</h3>
        <div class="grow"></div>
        <div class="search">
          <i class="fa fa-search"></i>
          <input class="input" placeholder="Cari nama / ID instalasi" bind:value={q} />
        </div>
        <button class="btn btn-primary btn-sm" onclick={tambah} disabled={form && form.mode === 'baru'}>
          <i class="fa fa-plus"></i> Tambah pelanggan
        </button>
      </div>

      {#if form}
        {#key form.key}
          <FormPelanggan
            instances={instances}
            plans={plans}
            instError={instError}
            awal={form.row}
            onsimpan={simpanPelanggan}
            ontutup={tutupForm}
            on401={sesiHabis}
            onbuatpanel={kePanel}
            onmuatulang={muatReferensi}
          />
        {/key}
      {/if}

      {#if total === 0}
        <div class="card-body">
          <div class="box info">
            <i class="fa fa-users"></i>
            Belum ada pelanggan yang terdaftar. Tambah pelanggan pertama untuk membuat sesi
            panelnya beserta tautan pembayaran.
          </div>
          <button class="btn btn-primary mt-2" onclick={tambah}>
            <i class="fa fa-plus"></i> Tambah pelanggan
          </button>
        </div>
      {:else}
        <div class="tablewrap">
          <table class="table">
            <thead>
              <tr>
                <th>Pelanggan</th>
                <th>ID Instalasi</th>
                <th>Paket</th>
                <th>Sisa</th>
                <th>Heartbeat</th>
                <th>Status</th>
                <th class="right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#each filtered as r, i (r.id)}
                <tr class="baris-masuk" style="animation-delay:{(i > 8 ? 8 : i) * 28}ms">
                  <td>
                    <div class="name">{r.institution}</div>
                    <div class="tiny muted">{r.name}</div>
                  </td>
                  <td>
                    <div class="mono tiny">{prettyId(r.instance_id)}</div>
                    <div class="tiny muted">sesi <span class="mono">{r.session_name}</span></div>
                  </td>
                  <td>
                    {r.plan_label}
                    {#if r.monthly}
                      <div class="tiny muted">{fmtRupiah(r.monthly)}/bln</div>
                    {/if}
                  </td>
                  <td class="nowrap">{sisa(r.days_left)}</td>
                  <td class="muted small nowrap">{r.last_seen}</td>
                  <td><span class="badge {r.status}">{stateLabel[r.status] ?? r.status}</span></td>
                  <td class="right nowrap">
                    <div class="aksi">
                      <button
                        class="btn btn-ghost btn-sm"
                        title="Salin tautan pembayaran"
                        aria-label="Salin tautan pembayaran {r.institution}"
                        onclick={() => salinTautan(r)}
                      >
                        <i class="fa {copied === r.id ? 'fa-check' : 'fa-copy'}"></i>
                        {#if copied === r.id}<span class="tiny">tersalin</span>{/if}
                      </button>
                      <button
                        class="btn btn-ghost btn-sm"
                        title="Ubah data pelanggan"
                        aria-label="Ubah data {r.institution}"
                        onclick={() => edit(r)}
                      >
                        <i class="fa fa-pencil"></i>
                      </button>
                      <button
                        class="btn btn-ghost btn-sm"
                        title="Perpanjang langganan"
                        aria-label="Perpanjang langganan {r.institution}"
                        aria-expanded={extendFor === r.id}
                        onclick={() => (extendFor = extendFor === r.id ? '' : r.id)}
                      >
                        <i class="fa fa-clock-o"></i>
                      </button>
                      <button
                        class="btn btn-ghost btn-sm"
                        title="Hapus pelanggan"
                        aria-label="Hapus pelanggan {r.institution}"
                        onclick={() => hapus(r)}
                        disabled={busyId === r.id}
                      >
                        <i class="fa fa-trash"></i>
                      </button>
                      <button class="btn btn-ghost btn-sm" onclick={() => toggle(r)} disabled={busyId === r.id}>
                        {#if r.status === 'expired'}
                          <i class="fa fa-play"></i> Aktifkan
                        {:else}
                          <i class="fa fa-pause"></i> Tangguhkan
                        {/if}
                      </button>
                    </div>
                  </td>
                </tr>
                {#if extendFor === r.id}
                  <tr class="subrow">
                    <td colspan="7">
                      <div class="row wrap">
                        <span class="small nowrap">Perpanjang <b>{r.institution}</b>:</span>
                        {#each [1, 3, 6, 12] as bulan}
                          <button
                            class="btn btn-ghost btn-sm"
                            onclick={() => perpanjang(r, bulan)}
                            disabled={busyId === r.id}
                          >
                            {bulan} bulan
                          </button>
                        {/each}
                        {#if busyId === r.id}
                          <span class="small muted"><i class="fa fa-spinner"></i> Menyimpan…</span>
                        {/if}
                        <button
                          class="btn btn-ghost btn-sm"
                          onclick={() => (extendFor = '')}
                          disabled={busyId === r.id}
                        >
                          Batal
                        </button>
                        <div class="grow"></div>
                        <span class="tiny muted">
                          Sisa sekarang {sisa(r.days_left)}. Waktu yang belum habis tidak hangus.
                        </span>
                      </div>
                    </td>
                  </tr>
                {/if}
              {/each}
              {#if filtered.length === 0}
                <tr><td colspan="7" class="center muted">Tidak ada yang cocok.</td></tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    </div>

    <!-- ----------------------------------------------------- panel terpasang -- -->
    <PanelDeploy
      {instances}
      {instError}
      bind:bukaForm={formPanel}
      onpanelbaru={panelBaru}
      on401={sesiHabis}
      onmuatulang={muatReferensi}
    />

    <!-- -------------------------------------------------------- aktivitas -- -->
    <div class="grid cols-2 mt-3">
      <div class="card">
        <div class="card-head"><i class="fa fa-list-ul"></i><h3>Catatan terakhir</h3></div>
        <div class="card-body">
          <ul style="margin:0;padding-left:16px">
            {#each data.activity as a}
              <li class="small" style="margin-bottom:7px">
                <span class="muted">{a.at}</span><br />{a.text}
              </li>
            {/each}
          </ul>
        </div>
      </div>

      <div class="card">
        <div class="card-head"><i class="fa fa-shield"></i><h3>Yang tidak bisa dilakukan pelanggan</h3></div>
        <div class="card-body small">
          <ul style="margin:0;padding-left:16px;line-height:1.8">
            <li>Mengganti gambar atau nominal QRIS &mdash; QRIS tidak pernah ada di server mereka.</li>
            <li>Memalsukan lisensi &mdash; tidak ada file lisensi atau kunci di server mereka.</li>
            <li>Menghindari penangguhan &mdash; status selalu ditanyakan ke portal ini.</li>
          </ul>
          <div class="box warning mt-2">
            <i class="fa fa-info-circle"></i>
            Pelanggan yang pandai tetap bisa menambal PHP-nya supaya tidak bertanya ke portal.
            Yang benar-benar terlindungi adalah uangnya.
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>

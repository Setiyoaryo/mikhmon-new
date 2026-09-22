<script>
  import Login from './Login.svelte'
  import { adminOverview, adminMe, adminLogout, adminClaim, adminCustomer } from '../api.js'
  import { fmtRupiah, fmtDate, prettyId, pesanGagal } from '../format.js'

  let authed = $state(null) // null = belum diketahui, true/false = hasil pemeriksaan
  let user = $state('admin')
  let data = $state(null)
  let loading = $state(true)
  let error = $state('')
  let busyId = $state('')
  let flash = $state('')
  let q = $state('')

  let stats = $derived(data ? data.stats : { revenue_month: 0, customers: 0, active: 0, warning: 0, expired: 0 })
  let claims = $derived(data ? data.claims : [])

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
    } catch (e) {
      if (e.status === 401) authed = false
      else error = pesanGagal(e, 'Gagal memuat ulang data admin.')
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
    <span class="mark">N</span> NOCIFY
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
      <div class="card-head">
        <i class="fa fa-users"></i>
        <h3>Pelanggan</h3>
        <div class="grow"></div>
        <div class="search">
          <i class="fa fa-search"></i>
          <input class="input" placeholder="Cari nama / ID instalasi" bind:value={q} />
        </div>
      </div>

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
          {#each filtered as r (r.id)}
            <tr>
              <td>
                <div class="name">{r.institution}</div>
                <div class="tiny muted">{r.name}</div>
              </td>
              <td class="mono tiny">{prettyId(r.instance_id)}</td>
              <td>{r.plan_label}</td>
              <td class="nowrap">{sisa(r.days_left)}</td>
              <td class="muted small nowrap">{r.last_seen}</td>
              <td><span class="badge {r.status}">{r.status === 'active' ? 'Aktif' : r.status === 'warning' ? 'Segera berakhir' : 'Berakhir'}</span></td>
              <td class="right nowrap">
                <button class="btn btn-ghost btn-sm" onclick={() => toggle(r)} disabled={busyId === r.id}>
                  {#if r.status === 'expired'}
                    <i class="fa fa-play"></i> Aktifkan
                  {:else}
                    <i class="fa fa-pause"></i> Tangguhkan
                  {/if}
                </button>
              </td>
            </tr>
          {/each}
          {#if filtered.length === 0}
            <tr><td colspan="7" class="center muted">Tidak ada yang cocok.</td></tr>
          {/if}
        </tbody>
      </table>
    </div>

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

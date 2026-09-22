<script>
  import { customers, claims as seedClaims, activity, adminStats, fmtRupiah, prettyId } from '../data.js'

  let q = $state('')
  let claims = $state(seedClaims.map((c) => ({ ...c })))
  let rows = $state(customers.map((c) => ({ ...c })))
  let flash = $state('')
  let approvedTotal = $state(0)

  let filtered = $derived(
    rows.filter((r) => {
      const s = q.trim().toLowerCase()
      if (!s) return true
      return (
        r.name.toLowerCase().includes(s) ||
        r.institution.toLowerCase().includes(s) ||
        r.instance.toLowerCase().includes(s)
      )
    })
  )

  let counts = $derived({
    active: rows.filter((r) => r.status === 'active').length,
    warning: rows.filter((r) => r.status === 'warning').length,
    expired: rows.filter((r) => r.status === 'expired').length
  })

  function approve(c) {
    claims = claims.filter((x) => x.id !== c.id)
    approvedTotal += c.amount
    const row = rows.find((r) => r.institution === c.institution)
    if (row) row.status = 'active'
    rows = rows
    flash = `Pembayaran ${c.customer} (${fmtRupiah(c.amount)}) disetujui. Panel pelanggannya otomatis menerima status aktif pada heartbeat berikutnya.`
  }

  function reject(c) {
    claims = claims.filter((x) => x.id !== c.id)
    flash = `Klaim ${c.customer} ditolak. Tandai sudah dibaca.`
  }

  function toggle(r) {
    r.status = r.status === 'expired' ? 'active' : 'expired'
    r.days = r.status === 'expired' ? -1 : 30
    rows = rows
    flash =
      r.status === 'expired'
        ? `${r.institution} ditangguhkan. Panel-nya terkunci di heartbeat berikutnya.`
        : `${r.institution} diaktifkan kembali.`
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
  <span class="tiny muted nowrap">Masuk sebagai <b style="color:var(--text)">Setiyo</b> (admin)</span>
  <button class="btn btn-ghost btn-sm"><i class="fa fa-sign-out"></i> Keluar</button>
</div>

<div class="wrap">
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

  <!-- ----------------------------------------------------------- stat --- -->
  <div class="grid cols-4">
    <div class="stat is-money">
      <div class="label">Pendapatan {adminStats.mrrNote}</div>
      <div class="value">{fmtRupiah(adminStats.mrr + approvedTotal)}</div>
      <div class="hint">{rows.length} pelanggan berlangganan</div>
    </div>
    <div class="stat is-active">
      <div class="label">Aktif</div>
      <div class="value">{counts.active}</div>
      <div class="hint">panel berjalan normal</div>
    </div>
    <div class="stat is-warning">
      <div class="label">Berakhir &le; 7 hari</div>
      <div class="value">{counts.warning}</div>
      <div class="hint">waktunya menagih</div>
    </div>
    <div class="stat is-expired">
      <div class="label">Kedaluwarsa</div>
      <div class="value">{counts.expired}</div>
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
              <td>{c.plan}</td>
              <td><b>{fmtRupiah(c.amount)}</b></td>
              <td class="mono tiny">{c.ref}</td>
              <td class="muted small nowrap">{c.at}</td>
              <td class="right nowrap">
                <button class="btn btn-success btn-sm" onclick={() => approve(c)}>
                  <i class="fa fa-check"></i> Setujui
                </button>
                <button class="btn btn-danger btn-sm" onclick={() => reject(c)}>Tolak</button>
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
            <td class="mono tiny">{prettyId(r.instance)}</td>
            <td>{r.plan}</td>
            <td class="nowrap">{sisa(r.days)}</td>
            <td class="muted small nowrap">{r.lastSeen}</td>
            <td><span class="badge {r.status}">{r.status === 'active' ? 'Aktif' : r.status === 'warning' ? 'Segera berakhir' : 'Berakhir'}</span></td>
            <td class="right nowrap">
              <button class="btn btn-ghost btn-sm" onclick={() => toggle(r)}>
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
          {#each activity as a}
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
</div>

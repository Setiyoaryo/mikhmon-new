<script>
  import { adminCreateInstance, adminDeleteInstance } from '../api.js'
  import { pesanGagal, prettyId } from '../format.js'
  import { salinTeks } from '../clipboard.js'

  /*
   * Bagian pemasangan panel: daftar panel yang sudah melapor ke portal,
   * formulir panel baru, dan berkas include/instance.php untuk disalin ke VPS
   * panel. Token hanya ada di berkas itu, jadi jangan sampai tersebar.
   */
  let {
    instances = [],
    instError = '',
    bukaForm = $bindable(false),
    onpanelbaru = () => {},
    on401 = () => {},
    onmuatulang = () => {}
  } = $props()

  const jenis = [
    { value: 'shared', label: 'Panel bersama — beberapa pelanggan dalam satu VPS' },
    { value: 'dedicated', label: 'Panel khusus — satu pelanggan per VPS' }
  ]

  let nama = $state('')
  let kind = $state('shared')
  let busy = $state(false)
  let galat = $state('')
  let baru = $state(null) // instance hasil POST
  let lihat = $state('') // id instance yang berkasnya dibuka dari tabel
  let tersalin = $state('')
  let timer = 0

  let origin = $derived(typeof window !== 'undefined' ? window.location.origin : '')
  let terbuka = $derived(baru ?? instances.find((i) => i.id === lihat) ?? null)

  function berkas(inst) {
    return (
      '<?php\n' +
      '$mikhmon_instance = array(\n' +
      `  'id'     => '${inst.id}',\n` +
      `  'token'  => '${inst.token}',\n` +
      `  'portal' => '${origin}',\n` +
      ');\n'
    )
  }

  /*
   * Menghapus panel dari portal. Kalau masih ada pelanggan di dalamnya,
   * server yang menolak dan alasannya ditampilkan apa adanya.
   */
  async function hapusPanel(ins) {
    if (busy) return
    const tanya =
      `Hapus panel ${ins.name} (${prettyId(ins.id)})?\n\n` +
      'Panelnya sendiri tetap berjalan, tetapi tidak lagi dikenali portal: ' +
      'tidak akan bisa melapor, dan tidak akan pernah terkunci.'
    if (!window.confirm(tanya)) return

    busy = true
    galat = ''
    try {
      await adminDeleteInstance(ins.id)
      if (lihat === ins.id) lihat = ''
      onmuatulang()
    } catch (e) {
      if (e.status === 401) {
        on401()
        return
      }
      galat = pesanGagal(e, 'Gagal menghapus panel.')
    } finally {
      busy = false
    }
  }

  async function buatPanel(e) {
    e.preventDefault()
    if (busy || nama.trim() === '') return

    busy = true
    galat = ''
    try {
      const res = await adminCreateInstance({ name: nama.trim(), kind })
      baru = res.instance
      lihat = ''
      nama = ''
      kind = 'shared'
      bukaForm = false
      onpanelbaru(res.instance)
    } catch (err) {
      if (err.status === 401) {
        on401()
        return
      }
      galat = pesanGagal(err, 'Panel gagal dibuat.')
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

  function buka(id) {
    lihat = lihat === id ? '' : id
    tersalin = ''
  }
</script>

<div class="card mt-3" id="panel-terpasang" style="scroll-margin-top:64px">
  <div class="card-head">
    <i class="fa fa-server"></i>
    <h3>Panel terpasang</h3>
    <div class="grow"></div>
    <span class="badge neutral">{instances.length} panel</span>
    <button
      class="btn btn-primary btn-sm"
      onclick={() => {
        bukaForm = !bukaForm
        galat = ''
      }}
    >
      {#if bukaForm}
        <i class="fa fa-times"></i> Tutup form
      {:else}
        <i class="fa fa-plus"></i> Tambah panel
      {/if}
    </button>
  </div>

  {#if bukaForm}
    <!-- ---------------------------------------------------- form panel baru --- -->
    <form class="formpanel panel-masuk" onsubmit={buatPanel}>
      <div class="tiny muted">PANEL BARU</div>
      <h3>Pasang panel Mikhmon</h3>

      <div class="grid cols-2 mt-2">
        <div>
          <label class="flabel" for="pp-nama">Nama panel</label>
          <input
            id="pp-nama"
            class="input"
            bind:value={nama}
            placeholder="mis. Panel NOCIFY Jakarta"
            maxlength="60"
            required
          />
          <div class="tiny muted mt-1">Nama ini hanya untuk Anda, tidak dipakai panel.</div>
        </div>
        <div>
          <label class="flabel" for="pp-jenis">Jenis</label>
          <select id="pp-jenis" class="input" bind:value={kind}>
            {#each jenis as j (j.value)}
              <option value={j.value}>{j.label}</option>
            {/each}
          </select>
          <div class="tiny muted mt-1">
            Jenis hanya penanda cara Anda menagih pelanggan di panel itu.
          </div>
        </div>
      </div>

      {#if galat}
        <div class="box solid-danger mt-3">
          <i class="fa fa-exclamation-triangle"></i>
          {galat}
        </div>
      {/if}

      <div class="row wrap mt-3">
        <button class="btn btn-primary" type="submit" disabled={busy || nama.trim() === ''}>
          {#if busy}
            <i class="fa fa-spinner"></i> Membuat…
          {:else}
            <i class="fa fa-save"></i> Buat panel
          {/if}
        </button>
        <button class="btn btn-ghost" type="button" onclick={() => (bukaForm = false)}>Batal</button>
        <div class="grow"></div>
        <span class="tiny muted">
          Setelah dibuat, salin berkas <span class="mono">instance.php</span> ke VPS panel.
        </span>
      </div>
    </form>
  {/if}

  {#if terbuka}
    <!-- -------------------------------------------- berkas include/instance --- -->
    <div class="formpanel panel-masuk">
      <div class="row between wrap">
        <div>
          <div class="tiny muted">{baru ? 'PANEL BARU DIREKAM' : 'BERKAS PANEL'}</div>
          <h3>{terbuka.name}</h3>
          <div class="tiny muted">
            ID panel <span class="mono">{prettyId(terbuka.id)}</span>
          </div>
        </div>
        {#if baru}
          <button class="btn btn-ghost btn-sm" onclick={() => (baru = null)}>Selesai</button>
        {/if}
      </div>

      <div class="box solid-success mt-3">
        <i class="fa fa-file-code-o"></i>
        Tempel isi berkas ini ke <span class="mono">include/instance.php</span> di VPS yang
        menjalankan panelnya. Nama berkasnya harus persis itu, dan tidak ada berkas lain yang
        perlu diubah.
      </div>

      <pre class="snippet mt-2">{berkas(terbuka)}</pre>

      <div class="row wrap mt-2">
        <button class="btn btn-primary btn-sm" onclick={() => salin(berkas(terbuka), terbuka.id)}>
          <i class="fa {tersalin === terbuka.id ? 'fa-check' : 'fa-clipboard'}"></i>
          {#if tersalin === terbuka.id}Tersalin{:else}Salin berkas{/if}
        </button>
        <span class="tiny muted">
          alamat portal diambil dari halaman yang sedang Anda buka ({origin}).
        </span>
      </div>

      <div class="box solid-warning mt-3">
        <i class="fa fa-exclamation-triangle"></i>
        Token di berkas itu jangan dibagikan ke siapa pun. Siapa pun yang memegangnya bisa
        melapor ke portal ini atas nama panel tersebut, termasuk mengirim heartbeat palsu.
        Panel yang tokennya bocor sebaiknya dibuat ulang.
      </div>

      {#if galat}
        <div class="box danger mt-3">
          <i class="fa fa-exclamation-triangle"></i>
          {galat}
        </div>
      {/if}
    </div>
  {/if}

  {#if instError}
    <div class="card-body">
      <div class="box solid-danger">
        <i class="fa fa-exclamation-triangle"></i>
        {instError}
      </div>
      <button class="btn btn-ghost btn-sm mt-2" onclick={onmuatulang}>
        <i class="fa fa-rotate-right"></i> Coba muat lagi
      </button>
    </div>
  {:else if instances.length === 0}
    <div class="card-body">
      <div class="box info">
        <i class="fa fa-server"></i>
        Belum ada panel yang terpasang. Buat panel dulu, lalu salin berkas
        <span class="mono">instance.php</span> ke VPS panel supaya panelnya bisa melapor ke portal
        ini.
      </div>
      {#if !bukaForm}
        <button class="btn btn-primary mt-2" onclick={() => (bukaForm = true)}>
          <i class="fa fa-plus"></i> Tambah panel
        </button>
      {/if}
    </div>
  {:else}
    <div class="tablewrap">
      <table class="table">
        <thead>
          <tr>
            <th>Panel</th>
            <th>Jenis</th>
            <th>ID panel</th>
            <th>Pelanggan</th>
            <th>Terakhir lapor</th>
            <th class="right">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#each instances as ins, i (ins.id)}
            <tr class="baris-masuk" style="animation-delay:{(i > 8 ? 8 : i) * 28}ms">
              <td>
                <div class="name">{ins.name}</div>
                <div class="tiny muted">
                  {ins.router_name ? `${ins.router_name} · Mikhmon ${ins.version}` : 'Router belum terhubung'}
                </div>
              </td>
              <td>
                <span class="badge neutral">{ins.kind === 'dedicated' ? 'Khusus' : 'Bersama'}</span>
              </td>
              <td class="mono tiny">{prettyId(ins.id)}</td>
              <td class="nowrap">{ins.customers} pelanggan</td>
              <td class="muted small nowrap">{ins.last_seen || 'belum pernah'}</td>
              <td class="right nowrap">
                <div class="aksi">
                  <button
                    class="btn btn-ghost btn-sm"
                    aria-label="Berkas instance.php panel {ins.name}"
                    aria-expanded={lihat === ins.id}
                    title="Berkas include/instance.php"
                    onclick={() => buka(ins.id)}
                  >
                    <i class="fa fa-file-code-o"></i>
                    {#if lihat === ins.id}Tutup berkas{:else}Berkas{/if}
                  </button>
                  <button
                    class="btn btn-danger btn-sm"
                    aria-label="Hapus panel {ins.name}"
                    title="Hapus panel"
                    disabled={busy}
                    onclick={() => hapusPanel(ins)}
                  >
                    <i class="fa fa-trash"></i>
                  </button>
                </div>
              </td>
            </tr>
            {#if lihat === ins.id && !baru}
              <tr class="subrow">
                <td colspan="6">
                  <div class="row between wrap">
                    <span class="small">
                      <b>include/instance.php</b> &middot; tempel di VPS panel ini
                    </span>
                    <button class="btn btn-ghost btn-sm" onclick={() => salin(berkas(ins), ins.id)}>
                      <i class="fa {tersalin === ins.id ? 'fa-check' : 'fa-clipboard'}"></i>
                      {#if tersalin === ins.id}Tersalin{:else}Salin berkas{/if}
                    </button>
                  </div>
                  <pre class="snippet mt-2">{berkas(ins)}</pre>
                  <div class="tiny muted mt-2">
                    Token ini jangan dibagikan. Portal diisi <span class="mono">{origin}</span>.
                  </div>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

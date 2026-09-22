<script>
  import QrisCard from '../QrisCard.svelte'
  import {
    plans,
    demoCustomer,
    fmtRupiah,
    fmtDate,
    daysLeft,
    stateOf,
    stateLabel,
    prettyId
  } from '../data.js'

  let { variant = 'warning' } = $props()

  let c = $derived(demoCustomer(variant))
  let state = $derived(stateOf(c.expiresAt))
  let days = $derived(daysLeft(c.expiresAt))

  let picked = $state('P3M')
  let chosen = $derived(plans.find((p) => p.code === picked))
  let submitted = $state(false)

  let progress = $derived.by(() => {
    const total = c.expiresAt - c.startedAt
    const used = Date.now() - c.startedAt
    return Math.max(2, Math.min(100, Math.round((used / total) * 100)))
  })

  let waText = $derived(
    encodeURIComponent(
      `Halo NOCIFY, saya sudah bayar perpanjangan Mikhmon.\n\n` +
        `Instalasi : ${c.institution} (${prettyId(c.instanceId)})\n` +
        `Paket     : ${chosen.label}\n` +
        `Nominal   : ${fmtRupiah(chosen.price)}\n\n` +
        `Mohon dibantu aktivasi. Terima kasih.`
    )
  )
</script>

<div class="topbar">
  <div class="brand">
    <span class="mark">N</span> NOCIFY
    <span class="sub">Billing Mikhmon</span>
  </div>
  <div class="grow"></div>
  <span class="tiny muted mono nowrap">portal.nocify.id</span>
</div>

<div class="wrap narrow">
  <!-- ------------------------------------------------------- status ----- -->
  <div class="card">
    <div class="card-head">
      <i class="fa fa-wifi"></i>
      <h2>Langganan Mikhmon</h2>
      <div class="grow"></div>
      <span class="badge {state}">
        {#if state === 'active'}<i class="fa fa-check"></i>{/if}
        {#if state === 'warning'}<i class="fa fa-clock-o"></i>{/if}
        {#if state === 'expired'}<i class="fa fa-lock"></i>{/if}
        {stateLabel[state]}
      </span>
    </div>
    <div class="card-body">
      <div class="row between wrap">
        <div>
          <h1>{c.institution}</h1>
          <div class="muted small">
            Router <span class="mono">{c.routerName}</span> &middot; {c.version}
          </div>
        </div>
        <div class="right">
          <div class="tiny muted">ID INSTALASI</div>
          <div class="mono" style="font-size:15px">{prettyId(c.instanceId)}</div>
        </div>
      </div>

      <div class="grid cols-4 mt-3">
        <div>
          <div class="tiny muted">PAKET</div>
          <div style="font-weight:600">{plans.find((p) => p.code === c.planCode).label}</div>
        </div>
        <div>
          <div class="tiny muted">MULAI</div>
          <div style="font-weight:600">{fmtDate(c.startedAt)}</div>
        </div>
        <div>
          <div class="tiny muted">BERLAKU SAMPAI</div>
          <div style="font-weight:600">{fmtDate(c.expiresAt)}</div>
        </div>
        <div>
          <div class="tiny muted">SISA WAKTU</div>
          <div style="font-weight:600" class:cl-danger={state === 'expired'}>
            {#if days >= 0}{days} hari lagi{:else}Lewat {Math.abs(days)} hari{/if}
          </div>
        </div>
      </div>

      <div class="mt-3">
        <div class="progress {state === 'expired' ? 'expired' : state === 'warning' ? 'warning' : ''}">
          <span style="width:{state === 'expired' ? 100 : progress}%"></span>
        </div>
      </div>

      {#if state === 'expired'}
        <div class="box solid-danger mt-2">
          <i class="fa fa-lock"></i>
          Panel Mikhmon Anda <b>dikunci</b> sejak {fmtDate(c.expiresAt)}. Panel akan terbuka
          sendiri begitu pembayaran dikonfirmasi — tidak perlu instal ulang apa pun.
        </div>
      {:else if state === 'warning'}
        <div class="box solid-warning mt-2">
          <i class="fa fa-exclamation-triangle"></i>
          Langganan berakhir <b>{days} hari lagi</b>. Perpanjang sekarang supaya panel tidak
          terkunci saat sedang dipakai.
        </div>
      {/if}
    </div>
  </div>

  <!-- ------------------------------------------------------ pembayaran --- -->
  <div class="card mt-3">
    <div class="card-head">
      <i class="fa fa-qrcode"></i>
      <h3>Perpanjang Langganan</h3>
    </div>

    {#if submitted}
      <!-- --------------------------------------------- menunggu verifikasi -->
      <div class="card-body">
        <div class="box solid-success">
          <i class="fa fa-check-circle"></i>
          <b>Terima kasih. Klaim pembayaran Anda sudah masuk.</b>
        </div>

        <div class="grid cols-2 mt-3">
          <div>
            <div class="tiny muted">NOMOR REFERENSI</div>
            <div class="mono" style="font-size:17px;font-weight:700">{c.payToken}</div>
            <div class="tiny muted mt-2">
              Sebutkan nomor ini kalau menghubungi NOCIFY, supaya lebih cepat dicari.
            </div>
          </div>
          <div>
            <div class="tiny muted">YANG DIPESAN</div>
            <div style="font-weight:600">{chosen.label} &middot; {fmtRupiah(chosen.price)}</div>
            <div class="tiny muted mt-2">
              Biasanya dikonfirmasi dalam beberapa menit pada jam kerja. Setelah
              dikonfirmasi, tanggal berakhir di atas bertambah otomatis.
            </div>
          </div>
        </div>

        <div class="box warning mt-3">
          <i class="fa fa-info-circle"></i>
          Kirim bukti transfer lewat WhatsApp supaya verifikasinya lebih cepat.
        </div>

        <div class="row wrap mt-2">
          <a class="btn btn-success" href="https://wa.me/6285139495106?text={waText}" target="_blank" rel="noopener">
            <i class="fa fa-whatsapp"></i> Kirim bukti via WhatsApp
          </a>
          <button class="btn btn-ghost" onclick={() => (submitted = false)}>
            <i class="fa fa-rotate-left"></i> Ganti paket
          </button>
        </div>
      </div>
    {:else}
      <!-- ------------------------------------------------------ pilih paket -->
      <div class="card-body">
        <div class="muted small mb-2"><b>1.</b> Pilih lama perpanjangan</div>
        <div class="plans">
          {#each plans as p (p.code)}
            <button class="plan" class:selected={picked === p.code} onclick={() => (picked = p.code)}>
              <i class="fa fa-check-circle tick"></i>
              <div class="t">{p.label}</div>
              <div class="p">{fmtRupiah(p.price)}</div>
              <div class="d">{p.note ? p.note : `Rp ${Math.round(p.price / p.months).toLocaleString('id-ID')}/bln`}</div>
            </button>
          {/each}
        </div>

        <div class="muted small mb-2 mt-3"><b>2.</b> Bayar lewat QRIS</div>
        <div class="grid cols-2">
          <div>
            <QrisCard amount={chosen.price} />
          </div>

          <div>
            <div class="amount">
              <div class="tiny muted">NOMINAL YANG HARUS DIBAYAR</div>
              <div class="big">{fmtRupiah(chosen.price)}</div>
              <div class="tiny muted mt-1">
                Masukkan nominal <b>persis</b> sebesar itu di aplikasi GoPay / bank Anda.
              </div>
            </div>

            <div class="mt-3">
              <button class="btn btn-primary btn-block" onclick={() => (submitted = true)}>
                <i class="fa fa-check"></i> Saya sudah bayar
              </button>
              <a
                class="btn btn-ghost btn-block mt-2"
                href="https://wa.me/6285139495106?text={encodeURIComponent('Halo NOCIFY, saya mau tanya soal langganan Mikhmon.')}"
                target="_blank"
                rel="noopener"
              >
                <i class="fa fa-whatsapp"></i> Tanya dulu
              </a>
            </div>

            <div class="box info mt-3">
              <i class="fa fa-info-circle"></i>
              Pembayaran QRIS diperiksa manual, jadi tidak langsung aktif. Setelah menekan
              <b>Saya sudah bayar</b>, NOCIFY mengaktifkan paket Anda dari portal ini.
            </div>
          </div>
        </div>
      </div>
    {/if}
  </div>

  <div class="center muted tiny mt-3">
    QRIS merchant milik NOCIFY &middot; halaman ini hanya ada di portal.nocify.id
  </div>
</div>

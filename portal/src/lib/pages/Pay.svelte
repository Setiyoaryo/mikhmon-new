<script>
  import QrisCard from '../QrisCard.svelte'
  import { path, match } from '../router.js'
  import { payInfo, payClaim } from '../api.js'
  import { fmtRupiah, fmtDate, prettyId, stateLabel, pesanGagal } from '../format.js'

  /* Dipakai kalau tautannya tidak dikenali: nomor WA NOCIFY. */
  const WA_FALLBACK = '6285139495106'

  let params = $derived(match('/pay/:token', $path))
  let token = $derived(params ? params.token : '')

  let loading = $state(true)
  let notFound = $state(false)
  let loadError = $state('')
  let data = $state(null)

  let picked = $state('')
  let claim = $state(null)
  let submitting = $state(false)
  let submitError = $state('')

  let plans = $derived(data && data.plans ? data.plans : [])
  let chosen = $derived(plans.find((p) => p.code === picked) ?? plans[0] ?? null)
  let amount = $derived(chosen ? chosen.price : 0)

  let sub = $derived(data ? data.subscription : null)
  let state = $derived(sub ? sub.state : 'active')
  let days = $derived(sub ? sub.days_left : 0)

  /* Isi bilah progres dari started_at sampai expires_at. */
  let progress = $derived.by(() => {
    if (!sub) return 0
    if (sub.state === 'expired') return 100
    const total = new Date(sub.expires_at).getTime() - new Date(sub.started_at).getTime()
    const used = Date.now() - new Date(sub.started_at).getTime()
    if (!(total > 0)) return 2
    return Math.max(2, Math.min(100, Math.round((used / total) * 100)))
  })

  let waText = $derived(
    encodeURIComponent(
      `Halo NOCIFY, saya sudah bayar perpanjangan Mikhmon.\n\n` +
        `Instalasi : ${data ? data.customer.institution : ''} (${prettyId(data ? data.instance.id : '')})\n` +
        `Paket     : ${claim ? claim.plan_label : chosen ? chosen.label : '-'}\n` +
        `Nominal   : ${fmtRupiah(claim ? claim.amount : amount)}\n\n` +
        `Mohon dibantu aktivasi. Terima kasih.`
    )
  )

  $effect(() => {
    const t = token
    load(t)
  })

  async function load(t) {
    loading = true
    notFound = false
    loadError = ''
    submitError = ''
    data = null
    claim = null
    picked = ''

    if (!t) {
      notFound = true
      loading = false
      return
    }

    try {
      const d = await payInfo(t)
      data = d
      picked = d.plans && d.plans.length ? d.plans[0].code : ''
      claim = d.pending_claim || null
    } catch (e) {
      if (e.status === 404) notFound = true
      else loadError = pesanGagal(e, 'Gagal memuat data langganan.')
    } finally {
      loading = false
    }
  }

  async function submitClaim() {
    if (!chosen || submitting) return
    submitting = true
    submitError = ''
    try {
      claim = await payClaim(token, chosen.code)
    } catch (e) {
      if (e.status === 409 && e.claim) claim = e.claim
      else if (e.status === 404) {
        data = null
        notFound = true
      } else submitError = pesanGagal(e, 'Klaim gagal dikirim. Coba lagi.')
    } finally {
      submitting = false
    }
  }
</script>

<div class="topbar">
  <div class="brand">
    <img src="./logo-nocify-on-dark.svg" alt="NOCIFY" class="logo" />
    <span class="sub">Billing Mikhmon</span>
  </div>
  <div class="grow"></div>
  <span class="tiny muted mono nowrap">portal.nocify.id</span>
</div>

<div class="wrap narrow">
  {#if loading}
    <!-- --------------------------------------------------------- memuat --- -->
    <div class="card">
      <div class="card-head">
        <i class="fa fa-wifi"></i>
        <h2>Langganan Mikhmon</h2>
      </div>
      <div class="card-body">
        <div class="muted small">Memuat…</div>
      </div>
    </div>
  {:else if notFound}
    <!-- ----------------------------------------------------- tak dikenal --- -->
    <div class="card">
      <div class="card-head">
        <i class="fa fa-link"></i>
        <h2>Langganan Mikhmon</h2>
      </div>
      <div class="card-body">
        <div class="box solid-danger">
          <i class="fa fa-exclamation-triangle"></i>
          Tautan pembayaran ini tidak dikenali. Mungkin salah ketik, atau tautannya sudah
          tidak berlaku lagi.
        </div>

        <div class="row wrap mt-3">
          <a
            class="btn btn-success"
            href="https://wa.me/{WA_FALLBACK}?text={encodeURIComponent('Halo NOCIFY, tautan pembayaran saya tidak dikenali. Mohon dibantu.')}"
            target="_blank"
            rel="noopener"
          >
            <i class="fa fa-whatsapp"></i> Hubungi NOCIFY
          </a>
        </div>
      </div>
    </div>
  {:else if loadError}
    <!-- -------------------------------------------------------- gagal ----- -->
    <div class="card">
      <div class="card-head">
        <i class="fa fa-wifi"></i>
        <h2>Langganan Mikhmon</h2>
      </div>
      <div class="card-body">
        <div class="box danger">
          <i class="fa fa-exclamation-triangle"></i>
          {loadError}
        </div>
        <button class="btn btn-ghost mt-3" onclick={() => load(token)}>
          <i class="fa fa-rotate-right"></i> Coba lagi
        </button>
      </div>
    </div>
  {:else if data}
    <!-- ------------------------------------------------------- status ----- -->
    <div class="card">
      <div class="card-head">
        <i class="fa fa-wifi"></i>
        <h2>Langganan Mikhmon</h2>
        <div class="grow"></div>
        <span class="badge {state}">
          {#if state === 'active'}<i class="fa fa-check"></i>{/if}
          {#if state === 'warning' || state === 'grace'}<i class="fa fa-clock-o"></i>{/if}
          {#if state === 'expired'}<i class="fa fa-lock"></i>{/if}
          {stateLabel[state] ?? state}
        </span>
      </div>
      <div class="card-body">
        <div class="row between wrap">
          <div>
            <h1>{data.customer.institution}</h1>
            <div class="muted small">
              Router <span class="mono">{data.instance.router_name}</span> &middot; {data.instance.version}
            </div>
          </div>
          <div class="right">
            <div class="tiny muted">ID INSTALASI</div>
            <div class="mono" style="font-size:15px">{prettyId(data.instance.id)}</div>
          </div>
        </div>

        <div class="grid cols-4 mt-3">
          <div>
            <div class="tiny muted">PAKET</div>
            <div style="font-weight:600">{sub.plan_label}</div>
          </div>
          <div>
            <div class="tiny muted">MULAI</div>
            <div style="font-weight:600">{fmtDate(sub.started_at)}</div>
          </div>
          <div>
            <div class="tiny muted">BERLAKU SAMPAI</div>
            <div style="font-weight:600">{fmtDate(sub.expires_at)}</div>
          </div>
          <div>
            <div class="tiny muted">SISA WAKTU</div>
            <div style="font-weight:600" class:cl-danger={state === 'expired'}>
              {#if days >= 0}{days} hari lagi{:else}Lewat {Math.abs(days)} hari{/if}
            </div>
          </div>
        </div>

        <div class="mt-3">
          <div class="progress {state === 'expired' ? 'expired' : state === 'warning' || state === 'grace' ? 'warning' : ''}">
            <span style="width:{progress}%"></span>
          </div>
        </div>

        {#if state === 'expired'}
          <div class="box solid-danger mt-2">
            <i class="fa fa-lock"></i>
            Panel Mikhmon Anda <b>dikunci</b> sejak {fmtDate(sub.expires_at)}. Panel akan terbuka
            sendiri begitu pembayaran dikonfirmasi — tidak perlu instal ulang apa pun.
          </div>
        {:else if state === 'warning' || state === 'grace'}
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

      {#if claim}
        <!-- --------------------------------------------- menunggu verifikasi -->
        <div class="card-body">
          <div class="box solid-success">
            <i class="fa fa-check-circle"></i>
            <b>Terima kasih. Klaim pembayaran Anda sudah masuk.</b>
          </div>

          <div class="grid cols-2 mt-3">
            <div>
              <div class="tiny muted">NOMOR REFERENSI</div>
              <div class="mono" style="font-size:17px;font-weight:700">{claim.ref}</div>
              <div class="tiny muted mt-2">
                Sebutkan nomor ini kalau menghubungi NOCIFY, supaya lebih cepat dicari.
              </div>
            </div>
            <div>
              <div class="tiny muted">YANG DIPESAN</div>
              <div style="font-weight:600">{claim.plan_label} &middot; {fmtRupiah(claim.amount)}</div>
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
            <a class="btn btn-success" href="https://wa.me/{data.wa_number}?text={waText}" target="_blank" rel="noopener">
              <i class="fa fa-whatsapp"></i> Kirim bukti via WhatsApp
            </a>
            <button class="btn btn-ghost" onclick={() => (claim = null)}>
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
              <button class="plan" class:selected={chosen ? chosen.code === p.code : false} onclick={() => (picked = p.code)}>
                <i class="fa fa-check-circle tick"></i>
                <div class="t">{p.label}</div>
                <div class="p">{fmtRupiah(p.price)}</div>
                <div class="d">{p.note ? p.note : `Rp ${Math.round(p.price / (p.months || 1)).toLocaleString('id-ID')}/bln`}</div>
              </button>
            {/each}
          </div>

          <div class="muted small mb-2 mt-3"><b>2.</b> Bayar lewat QRIS</div>
          <div class="grid cols-2">
            <div>
              <QrisCard
                amount={amount}
                merchant={data.qris.merchant}
                nmid={data.qris.nmid}
                imageUrl={data.qris.image_url}
              />
            </div>

            <div>
              <div class="amount">
                <div class="tiny muted">NOMINAL YANG HARUS DIBAYAR</div>
                <div class="big">{fmtRupiah(amount)}</div>
                <div class="tiny muted mt-1">
                  Masukkan nominal <b>persis</b> sebesar itu di aplikasi GoPay / bank Anda.
                </div>
              </div>

              <div class="mt-3">
                <button class="btn btn-primary btn-block" onclick={submitClaim} disabled={submitting || !chosen}>
                  {#if submitting}
                    <i class="fa fa-spinner"></i> Mengirim…
                  {:else}
                    <i class="fa fa-check"></i> Saya sudah bayar
                  {/if}
                </button>
                <a
                  class="btn btn-ghost btn-block mt-2"
                  href="https://wa.me/{data.wa_number}?text={encodeURIComponent('Halo NOCIFY, saya mau tanya soal langganan Mikhmon.')}"
                  target="_blank"
                  rel="noopener"
                >
                  <i class="fa fa-whatsapp"></i> Tanya dulu
                </a>
              </div>

              {#if submitError}
                <div class="box danger mt-3">
                  <i class="fa fa-exclamation-triangle"></i>
                  {submitError}
                </div>
              {/if}

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
  {/if}
</div>

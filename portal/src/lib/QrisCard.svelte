<script>
  /*
   * Kartu QRIS contoh.
   *
   * Di portal sungguhan bagian ini menampilkan gambar QRIS merchant yang
   * diunggah admin. Untuk mockup, polanya digambar di sini supaya tidak perlu
   * file gambar dan tetap terlihat seperti QRIS asli.
   */
  let {
    amount = 0,
    merchant = 'NOCIFY, SOFTWARE',
    nmid = 'ID1026599320839',
    imageUrl = null
  } = $props()

  let hasImage = $derived(typeof imageUrl === 'string' && imageUrl.length > 0)

  const N = 25

  function isFinder(x, y) {
    return (x < 7 && y < 7) || (x >= N - 7 && y < 7) || (x < 7 && y >= N - 7)
  }

  const cells = (() => {
    let s = 987654321
    const rand = () => ((s = (s * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff)
    const out = []
    for (let y = 0; y < N; y++) {
      for (let x = 0; x < N; x++) {
        if (isFinder(x, y)) continue
        out.push({ x, y, on: rand() > 0.52 })
      }
    }
    return out
  })()

  const finders = [
    { x: 0, y: 0 },
    { x: N - 7, y: 0 },
    { x: 0, y: N - 7 }
  ]
</script>

<div class="qris">
  <div class="band">
    <i class="fa fa-credit-card-alt"></i>
    <span class="gopay">gopay <span style="font-weight:400">merchant</span></span>
  </div>

  <div class="body">
    <div class="head">
      <div class="qrislabel">QRIS</div>
      <div class="tiny" style="color:#41525f;text-align:right;line-height:1.3">
        QR Code Standar<br />Pembayaran Nasional
      </div>
    </div>

    <div class="merchant">{merchant}</div>
    <div class="nmid">NMID: {nmid}</div>

    <div class="code">
      {#if hasImage}
        <img
          src={imageUrl}
          alt="Kode QRIS merchant"
          style="width:215px;max-width:100%;height:auto;display:block"
        />
      {:else}
        <svg viewBox="0 0 {N * 10} {N * 10}" width="215" height="215" role="img" aria-label="Contoh kode QRIS">
          <rect width={N * 10} height={N * 10} fill="#fff" />
          {#each cells as c (c.x + '-' + c.y)}
            {#if c.on}
              <rect x={c.x * 10} y={c.y * 10} width="10" height="10" fill="#111" />
            {/if}
          {/each}
          {#each finders as f (f.x + ':' + f.y)}
            <rect x={(f.x + 0.5) * 10} y={(f.y + 0.5) * 10} width="60" height="60" fill="none" stroke="#111" stroke-width="10" />
            <rect x={(f.x + 2) * 10} y={(f.y + 2) * 10} width="30" height="30" fill="#111" />
          {/each}
        </svg>
      {/if}
    </div>

    {#if amount > 0}
      <div style="text-align:center;font-weight:700;font-size:15px;margin-top:2px">
        Rp {amount.toLocaleString('id-ID')}
      </div>
    {/if}

    <div class="foot">Terima pembayaran QRIS dari mana saja</div>
  </div>
</div>

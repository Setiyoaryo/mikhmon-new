<script>
  import { path, go } from './lib/router.js'
  import Pay from './lib/pages/Pay.svelte'
  import Admin from './lib/pages/Admin.svelte'

  /*
   * Mockup: dua halaman dengan pemilih di bawah supaya bisa dibandingkan.
   * Saat portal sungguhan dibangun, bar ini hilang dan rutenya:
   *   #/pay/<token>   halaman pelanggan, dari tautan yang dikirim NOCIFY
   *   #/admin         halaman admin (di balik login)
   */
  let variant = $state('warning')

  let page = $derived($path.indexOf('/admin') === 0 ? 'admin' : 'pay')

  if (typeof window !== 'undefined' && window.location.hash === '') {
    window.location.hash = '/pay/NOC-8F3A-2C71'
  }
</script>

{#if page === 'admin'}
  <Admin />
{:else}
  {#key variant}
    <Pay {variant} />
  {/key}
{/if}

<div class="mockbar">
  <span class="tag">Mockup</span>
  <button class:on={page === 'pay'} onclick={() => go('/pay/NOC-8F3A-2C71')}>
    Halaman pelanggan
  </button>
  <button class:on={page === 'admin'} onclick={() => go('/admin')}>Admin</button>

  {#if page === 'pay'}
    <span class="tag" style="border-left:1px solid var(--line);margin-left:4px;padding-left:12px">
      Status
    </span>
    <button class:on={variant === 'active'} onclick={() => (variant = 'active')}>Aktif</button>
    <button class:on={variant === 'warning'} onclick={() => (variant = 'warning')}>Mau habis</button>
    <button class:on={variant === 'expired'} onclick={() => (variant = 'expired')}>Habis</button>
  {/if}
</div>

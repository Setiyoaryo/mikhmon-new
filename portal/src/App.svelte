<script>
  import { path, go } from './lib/router.js'
  import Pay from './lib/pages/Pay.svelte'
  import Admin from './lib/pages/Admin.svelte'

  /*
   * Rute hash:
   *   #/pay/<token>   halaman pelanggan, dari tautan yang dikirim NOCIFY
   *   #/admin         halaman admin (login sendiri)
   *
   * Akar situs diarahkan ke halaman admin. Sebelumnya alamat
   * https://control.nocify.id tanpa rute apa pun jatuh ke halaman pembayaran
   * dengan token kosong, jadi yang membukanya melihat pesan "tautan tidak
   * dikenali" dan mengira portalnya rusak.
   */
  const dev = import.meta.env.DEV

  let page = $derived($path.indexOf('/pay/') === 0 ? 'pay' : 'admin')

  if (typeof window !== 'undefined' && (window.location.hash === '' || window.location.hash === '#')) {
    window.location.hash = dev ? '/pay/NOC-8F3A-2C71' : '/admin'
  }
</script>

{#if page === 'admin'}
  <Admin />
{:else}
  <Pay />
{/if}

{#if dev}
  <div class="mockbar">
    <span class="tag">Mockup</span>
    <button class:on={page === 'pay'} onclick={() => go('/pay/NOC-8F3A-2C71')}>
      Halaman pelanggan
    </button>
    <button class:on={page === 'admin'} onclick={() => go('/admin')}>Admin</button>
  </div>
{/if}

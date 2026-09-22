<script>
  import { adminLogin } from '../api.js'
  import { pesanGagal } from '../format.js'

  let { onlogin = null } = $props()

  let password = $state('')
  let busy = $state(false)
  let error = $state('')

  async function submit(e) {
    if (e) e.preventDefault()
    if (busy) return
    busy = true
    error = ''
    try {
      await adminLogin(password)
      password = ''
      if (onlogin) onlogin()
    } catch (err) {
      error =
        err && err.status === 401
          ? 'Password salah. Coba lagi.'
          : pesanGagal(err, 'Tidak bisa masuk. Coba lagi.')
    } finally {
      busy = false
    }
  }
</script>

<div class="wrap narrow">
  <div class="card" style="max-width:360px;margin:0 auto">
    <div class="card-head">
      <i class="fa fa-lock"></i>
      <h3>Masuk admin</h3>
    </div>
    <div class="card-body">
      {#if error}
        <div class="box danger mb-2">
          <i class="fa fa-exclamation-triangle"></i>
          {error}
        </div>
      {/if}

      <form onsubmit={submit}>
        <div class="tiny muted mb-2">PASSWORD ADMIN</div>
        <input
          class="input"
          type="password"
          bind:value={password}
          placeholder="Password"
          autocomplete="current-password"
        />
        <button class="btn btn-primary btn-block mt-2" type="submit" disabled={busy || password === ''}>
          {#if busy}
            <i class="fa fa-spinner"></i> Memeriksa…
          {:else}
            <i class="fa fa-sign-in"></i> Masuk
          {/if}
        </button>
      </form>

      <div class="center muted tiny mt-3">
        Halaman ini hanya untuk pengelola NOCIFY.
      </div>
    </div>
  </div>
</div>

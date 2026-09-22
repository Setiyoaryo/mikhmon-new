import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Satu binary Go akan meng-embed folder dist/ ini, jadi asetnya memakai path
// relatif: hasil build tetap jalan walau disajikan dari sub-path mana pun.
export default defineConfig({
  plugins: [svelte()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsInlineLimit: 0
  }
})

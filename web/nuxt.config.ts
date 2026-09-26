export default defineNuxtConfig({
  compatibilityDate: '2026-09-26',
  modules: ['@nuxt/ui', '@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  devtools: { enabled: false },
  ui: { fonts: false },
  icon: { provider: 'server', fallbackToApi: false },
  runtimeConfig: { backendUrl: 'http://127.0.0.1:8081' },
  app: { head: { title: 'ByteMonitor', htmlAttrs: { lang: 'en' } } },
})

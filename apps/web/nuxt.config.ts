export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@vueuse/nuxt'],
  css: ['~/assets/css/main.css'],
  devServer: {
    port: 4300
  },
  app: {
    baseURL: process.env.NUXT_APP_BASE_URL || '/flow/',
    head: {
      title: 'Ralleh Flow',
      htmlAttrs: { class: 'dark' },
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'theme-color', content: '#09090b' }
      ]
    }
  },
  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '/api/flow/v1',
      appName: process.env.NUXT_PUBLIC_APP_NAME || 'Ralleh Flow'
    }
  },
  postcss: {
    plugins: {
      '@tailwindcss/postcss': {}
    }
  },
  typescript: {
    strict: true
  }
})

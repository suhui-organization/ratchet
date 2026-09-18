import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2026-09-18',
  css: ['~/assets/css/main.css'],
  devtools: { enabled: false },
  // Tailwind v4 走 Vite 插件，不需要 postcss 配置
  vite: { plugins: [tailwindcss()] },
  app: {
    head: {
      title: 'Ratchet',
      meta: [
        { name: 'description', content: '从 agent 的真实行为编译最小权限策略，并给出收货方能自己验证的证据' },
      ],
    },
  },
})

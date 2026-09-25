import tailwindcss from '@tailwindcss/vite'
import { PRERENDER_ROUTES } from './routes'

export default defineNuxtConfig({
  compatibilityDate: '2026-09-18',
  css: ['~/assets/css/main.css'],
  devtools: { enabled: false },
  // Tailwind v4 走 Vite 插件，不需要 postcss 配置
  vite: { plugins: [tailwindcss()] },
  // 站点是静态导出的：没有 Node 运行时，构建期没生成的页面线上就是 404。
  // routes 显式列全（见 routes.ts，/thanks 是那个必须手写的例子）；
  // failOnError 让"某一页没渲染出来"直接打断构建，而不是安静地少一个文件。
  nitro: {
    prerender: {
      crawlLinks: true,
      routes: PRERENDER_ROUTES,
      failOnError: true,
    },
  },
  app: {
    head: {
      title: 'Ratchet',
      meta: [
        { name: 'description', content: '从 agent 的真实行为编译最小权限策略，并给出收货方能自己验证的证据' },
      ],
    },
  },
})

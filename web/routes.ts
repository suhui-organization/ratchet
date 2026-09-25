/**
 * 站点真实存在的路由清单。
 *
 * 为什么需要一份手写清单，而不是让构建去爬页面上的链接：
 *
 * 站点是**纯静态**的——`nuxt generate` 生成 HTML，运行时只有 nginx，没有 Node
 * 进程，请求打不到文件就是 404，不会临时渲染。所以每一条真实路由都必须在构建期
 * 生成出来，而"爬链接"这件事只能发现**页面上有入口**的路由。
 *
 * `/thanks` 恰好没有入口：它是支付成功后的跳转目标，站内没有任何地方链到它。
 * 靠爬链接时它不会被生成，生产环境上点完付款就落在一个 404 上（真实踩过）。
 *
 * 这份清单与 `app/pages/` 目录、与 `app/content/legal.ts` 的文档集合是否一致，
 * 由 `tests/routes.test.ts` 盯着：新增页面或新增一份法律文档却忘了登记，测试会
 * 点名失败，而不是等上线后由客户点出一个 404。
 *
 * 每条路由都要有中英两份（`/guard` 与 `/zh/guard`）——语言是路径前缀，不是
 * 客户端状态，两种语言必须各自是一个能被分享、能被搜索引擎收录的独立地址。
 */

/** 站点的两种语言前缀。空串是英文（默认语言，无前缀）。 */
const LOCALE_PREFIXES = ['', '/zh'] as const

/** 与语言无关的页面路径。每一项都会被展开成中英两条路由。 */
const PAGE_PATHS = [
  '/',
  '/guard',
  '/pricing',
  '/verify',
  // 支付成功页：没有站内入口，只能在这里登记，否则线上 404
  '/thanks',
  // 法律文档：slug 必须与 app/content/legal.ts 里每份文档的 slug 一致
  '/legal/terms',
  '/legal/privacy',
  '/legal/refund',
] as const

export const PRERENDER_ROUTES: string[] = LOCALE_PREFIXES.flatMap((prefix) =>
  PAGE_PATHS.map((path) => (path === '/' ? prefix || '/' : `${prefix}${path}`)),
)

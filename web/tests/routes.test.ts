import { readdirSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { LEGAL } from '../app/content/legal'
import { PRERENDER_ROUTES } from '../routes'

/**
 * 预渲染路由清单必须和"站点上真实有哪些页面"完全对上。
 *
 * 为什么这条测试值得存在：站点是纯静态导出的，构建期没生成出来的路由，线上
 * 就是一个 404，没有任何运行时会兜底。而路由发现的默认机制是爬页面上的链接——
 * 一个没有入口的页面（`/thanks`）会被静默漏掉，直到有人在生产环境点完付款。
 * 这个失败模式不报错、不告警，只在最贵的那一刻出现，所以用测试把它钉死。
 */

const PAGES_DIR = resolve(__dirname, '../app/pages')

/** 递归收集 app/pages 下所有 .vue 文件，返回相对路径，如 `[[lang]]/guard.vue`。 */
function pageFiles(dir = PAGES_DIR): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) return pageFiles(full)
    return entry.name.endsWith('.vue') ? [relative(PAGES_DIR, full)] : []
  })
}

const LEGAL_SLUGS = LEGAL.en.map((doc) => doc.slug).sort()

/**
 * 把页面文件路径翻成它实际服务的路由。
 *
 * 只认两种写法：`[[lang]]`（可选语言前缀）和 `[doc]`（法律文档 slug）。
 * 出现别的动态段就直接抛错——那说明路由的构成方式变了，这份推导已经不准，
 * 与其猜一个可能错的答案，不如让测试停下来要求人来更新它。
 */
function routesFor(file: string): string[] {
  const segments = file.replace(/\.vue$/, '').split('/')
  let prefixes = ['']

  for (const segment of segments) {
    if (segment === '[[lang]]') {
      // 英文不带前缀，中文加 /zh：两种语言各自是一个独立地址
      prefixes = [...prefixes, ...prefixes.map((p) => `${p}/zh`)]
      continue
    }
    if (segment === 'index') continue
    if (segment === '[doc]') {
      prefixes = prefixes.flatMap((p) => LEGAL_SLUGS.map((slug) => `${p}/${slug}`))
      continue
    }
    if (segment.startsWith('[')) {
      throw new Error(`routes.test.ts 不认识动态段 ${segment}（来自 ${file}）：请更新这份推导，别让它猜。`)
    }
    prefixes = prefixes.map((p) => `${p}/${segment}`)
  }

  // 去掉重复的 '' 和多余的尾部斜杠
  return [...new Set(prefixes.map((p) => p.replace(/\/+$/, '') || '/'))]
}

const expected = pageFiles().flatMap(routesFor).sort()

describe('静态站点路由', () => {
  it('从 app/pages 推导出的路由是有内容的（防止上面推导坏掉后变成空转）', () => {
    expect(expected.length).toBeGreaterThanOrEqual(10)
    expect(expected).toContain('/')
    expect(expected).toContain('/zh/guard')
    expect(expected).toContain('/legal/refund')
  })

  it('每一页都被登记进预渲染清单（漏登记 = 线上 404）', () => {
    const missing = expected.filter((route) => !PRERENDER_ROUTES.includes(route))
    expect(missing).toEqual([])
  })

  it('清单里没有已经不存在的页面', () => {
    const stale = PRERENDER_ROUTES.filter((route) => !expected.includes(route))
    expect(stale).toEqual([])
  })

  it('没有重复登记', () => {
    expect(PRERENDER_ROUTES.length).toBe(new Set(PRERENDER_ROUTES).size)
  })

  it('中英两棵语言树必须服务同一批路径', () => {
    const zh = PRERENDER_ROUTES.filter((r) => r === '/zh' || r.startsWith('/zh/')).map((r) =>
      r === '/zh' ? '/' : r.slice(3),
    )
    const en = PRERENDER_ROUTES.filter((r) => r !== '/zh' && !r.startsWith('/zh/'))
    expect(zh.sort()).toEqual(en.sort())
  })

  it('中文法律文档与英文是同一批 slug（多一份或漏一份都会有一个语言 404）', () => {
    expect(LEGAL.zh.map((doc) => doc.slug).sort()).toEqual(LEGAL_SLUGS)
  })
})

#!/usr/bin/env node
/**
 * 对**构建出来的静态产物**做一次真实浏览器验收：每条路由 × 390px / 1440px，
 * 量横向溢出、越界元素、小于 24px 的点击目标、文字对比度、`<html lang>`，
 * 以及**客户端到底有没有启动**。
 *
 * 用法（先构建，再起一个服务器把产物发出来，然后跑它）：
 *   cd web && npm run generate
 *   docker run --rm -d --name ratchet-check -p 18080:80 \
 *     -v "$PWD/../deploy/docker/default.conf:/etc/nginx/conf.d/default.conf:ro" \
 *     -v "/tmp/security-headers.inc:/etc/nginx/conf.d/security-headers.inc:ro" \
 *     -v "$PWD/.output/public:/usr/share/nginx/html:ro" nginx:1.27-alpine
 *   node scripts/check-pages.mjs http://127.0.0.1:18080
 *
 * 路由列表直接从产物目录里扫（扫到哪些 .html 就量哪些），不再手写一份：
 * 手写的那份会漂移，而这份**量的一定是准备发出去的东西**。
 * "有哪些路由本该存在"由 tests/routes.test.ts 守着。
 *
 * 为什么要量"水合"：这个站点最阴的故障不是白屏，而是 HTML 一切正常、文字都在，
 * 但客户端从没启动——收银台按钮点了没反应、/verify 的哈希算不出来。
 * 截图看不见，肉眼也看不见。实测踩过（CSP 拦掉了 Nuxt 的内联脚本）。
 *
 * 需要本机有 `google-chrome`；纯静态分析替代不了它。
 */
import { spawn } from 'node:child_process'
import { mkdtempSync, readdirSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const [base, distArg] = process.argv.slice(2)
if (!base) {
  console.error('用法: node scripts/check-pages.mjs <baseUrl> [dist目录]')
  process.exit(2)
}

const DIST = resolve(distArg ?? new URL('../.output/public', import.meta.url).pathname)

/** 扫产物目录得到路由：`index.html` → `/`，`a/b/index.html` → `/a/b`。 */
function routesFromDir(dir, prefix = '') {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) return routesFromDir(full, `${prefix}/${name}`)
    if (!name.endsWith('.html')) return []
    const route = name === 'index.html' ? prefix || '/' : `${prefix}/${name.slice(0, -5)}`
    // 200.html / 404.html 是 nitro 的 SPA 兜底壳，站点不对外提供（nginx 里挡掉了）
    return /^\/(200|404)$/.test(route) ? [] : [route]
  })
}

const routes = routesFromDir(DIST).sort()
if (routes.length === 0) {
  console.error(`错误：${DIST} 里没扫到页面——是不是还没跑 npm run generate？`)
  process.exit(1)
}

const profile = mkdtempSync(join(tmpdir(), 'ratchet-check-'))
const chrome = spawn(
  'google-chrome',
  [
    '--headless=new',
    '--no-sandbox',
    '--disable-gpu',
    '--disable-dev-shm-usage',
    '--remote-debugging-port=0',
    `--user-data-dir=${profile}`,
    'about:blank',
  ],
  { stdio: ['ignore', 'pipe', 'pipe'] },
)

const wsUrl = await new Promise((resolve, reject) => {
  let buf = ''
  const t = setTimeout(() => reject(new Error('chrome 没在 25s 内报出 DevTools 地址')), 25000)
  chrome.stderr.on('data', (d) => {
    buf += d.toString()
    const m = buf.match(/ws:\/\/[^\s]+/)
    if (m) {
      clearTimeout(t)
      resolve(m[0])
    }
  })
})

const ws = new WebSocket(wsUrl)
await new Promise((resolve, reject) => {
  ws.onopen = resolve
  ws.onerror = (e) => reject(new Error(`ws 连不上: ${e.message ?? e}`))
})

let nextId = 1
const pending = new Map()
const listeners = new Set()
ws.onmessage = (ev) => {
  const msg = JSON.parse(ev.data)
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id)
    pending.delete(msg.id)
    if (msg.error) reject(new Error(msg.error.message))
    else resolve(msg.result)
  } else if (msg.method) {
    for (const fn of listeners) fn(msg)
  }
}

function send(method, params = {}, sessionId) {
  const id = nextId++
  ws.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }))
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }))
}

const { targetId } = await send('Target.createTarget', { url: 'about:blank' })
const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
await send('Page.enable', {}, sessionId)
await send('Runtime.enable', {}, sessionId)
await send('Log.enable', {}, sessionId)

// CSP 违规、被拦下的脚本、未捕获异常都从这里冒出来，必须算作失败。
// 文档自身的 404 状态（量 /nope 时）和浏览器自动请求的 favicon 不算。
let noise = []
let currentUrl = ''
listeners.add((msg) => {
  if (msg.method === 'Log.entryAdded') {
    const e = msg.params.entry
    const url = e.url ?? ''
    if (url === currentUrl || url.endsWith('/favicon.ico')) return
    if (e.level === 'error' || e.source === 'security') noise.push(`[${e.source}] ${e.text.slice(0, 140)}`)
  }
  if (msg.method === 'Runtime.exceptionThrown') {
    noise.push(`[exception] ${msg.params.exceptionDetails.text}`)
  }
})

/** 在页面里跑的度量：返回这一页在当前视口下的所有问题。 */
const MEASURE = `(() => {
  const vw = window.innerWidth
  const de = document.documentElement
  const out = {
    title: document.title,
    lang: de.getAttribute('lang'),
    hydrated: !!document.querySelector('#__nuxt')?.__vue_app__,
    overflowX: de.scrollWidth - vw,
    overflowers: [],
    smallTargets: [],
    contrast: [],
  }
  const visible = (el) => {
    const cs = getComputedStyle(el)
    return cs.display !== 'none' && cs.visibility !== 'hidden' && cs.opacity !== '0'
  }
  const label = (el) => {
    const cls = (el.getAttribute('class') || '').split(/\\s+/).slice(0, 3).join('.')
    return el.tagName.toLowerCase() + (cls ? '.' + cls : '')
  }

  for (const el of document.querySelectorAll('body *')) {
    if (!visible(el)) continue
    const r = el.getBoundingClientRect()
    if (r.width === 0 || r.height === 0) continue
    if (r.right > vw + 1 || r.left < -1) {
      out.overflowers.push({ el: label(el), left: Math.round(r.left), right: Math.round(r.right) })
    }
  }

  // WCAG 2.5.8：可点目标至少 24×24
  for (const el of document.querySelectorAll('a[href], button, [role="button"], input, select, textarea, summary')) {
    if (!visible(el)) continue
    if (getComputedStyle(el).pointerEvents === 'none') continue
    const r = el.getBoundingClientRect()
    if (r.width === 0 && r.height === 0) continue
    if (r.width < 24 || r.height < 24) {
      out.smallTargets.push({ el: label(el), w: +r.width.toFixed(1), h: +r.height.toFixed(1) })
    }
  }

  // WCAG AA 对比度：<script> 的原始字节决定了哈希，这里只关心文字。
  const parse = (c) => {
    const m = c.match(/rgba?\\(([^)]+)\\)/)
    if (!m) return null
    const p = m[1].split(/[,\\s\\/]+/).filter(Boolean).map(Number)
    return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 }
  }
  const over = (fg, bg) => ({
    r: fg.r * fg.a + bg.r * (1 - fg.a),
    g: fg.g * fg.a + bg.g * (1 - fg.a),
    b: fg.b * fg.a + bg.b * (1 - fg.a),
    a: 1,
  })
  const lum = (c) => {
    const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4) }
    return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b)
  }
  const ratio = (a, b) => {
    const [x, y] = [lum(a), lum(b)].sort((m, n) => n - m)
    return (x + 0.05) / (y + 0.05)
  }
  // 文字背后的颜色要把祖先的背景一层层叠上来（含半透明），不能只看直接父元素
  const effectiveBg = (el) => {
    let bg = { r: 255, g: 255, b: 255, a: 1 }
    const chain = []
    for (let n = el; n; n = n.parentElement) chain.push(n)
    for (const n of chain.reverse()) {
      const c = parse(getComputedStyle(n).backgroundColor)
      if (c && c.a > 0) bg = over(c, bg)
    }
    return bg
  }

  for (const el of document.querySelectorAll('body *')) {
    if (!visible(el)) continue
    if (![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue
    const cs = getComputedStyle(el)
    const fg = parse(cs.color)
    if (!fg || fg.a === 0) continue
    const bg = effectiveBg(el)
    const text = fg.a < 1 ? over(fg, bg) : fg
    const size = parseFloat(cs.fontSize)
    const weight = Number(cs.fontWeight) || 400
    const need = size >= 24 || (size >= 18.66 && weight >= 700) ? 3 : 4.5
    const r = ratio(text, bg)
    if (r < need - 0.01) {
      out.contrast.push({ el: label(el), ratio: +r.toFixed(2), need, sample: el.textContent.trim().slice(0, 28) })
    }
  }
  return out
})()`

const SIZES = [
  { w: 390, h: 844, mobile: true },
  { w: 1440, h: 900, mobile: false },
]

let failures = 0
const rows = []

for (const size of SIZES) {
  await send('Emulation.setDeviceMetricsOverride',
    { width: size.w, height: size.h, deviceScaleFactor: 1, mobile: size.mobile }, sessionId)
  for (const route of routes) {
    noise = []
    currentUrl = base + route
    const loaded = new Promise((resolve) => {
      const fn = (msg) => {
        if (msg.method === 'Page.loadEventFired') {
          listeners.delete(fn)
          resolve()
        }
      }
      listeners.add(fn)
    })
    await send('Page.navigate', { url: currentUrl }, sessionId)
    await loaded
    // 等水合：对比度、lang、可点区域都是客户端跑完之后才定下来的
    await new Promise((r) => setTimeout(r, 700))
    const { result } = await send('Runtime.evaluate',
      { expression: MEASURE, returnByValue: true }, sessionId)
    const m = result.value
    const bad = m.overflowX > 0 || m.overflowers.length || m.smallTargets.length ||
      m.contrast.length || !m.lang || !m.hydrated || noise.length
    if (bad) failures++
    rows.push({ size: size.w, route, log: [...noise], ...m })
  }
}

for (const r of rows) {
  const flag = r.overflowX > 0 || r.overflowers.length || r.smallTargets.length ||
    r.contrast.length || !r.lang || !r.hydrated || r.log.length
  console.log(
    `${flag ? 'FAIL' : 'ok  '} ${String(r.size).padStart(4)} ${r.route.padEnd(22)} ` +
      `lang=${r.lang} 水合=${r.hydrated ? 'yes' : 'NO'} overflowX=${r.overflowX} ` +
      `越界=${r.overflowers.length} 小目标=${r.smallTargets.length} ` +
      `对比度失败=${r.contrast.length} 控制台=${r.log.length}`,
  )
  for (const o of r.overflowers.slice(0, 4)) console.log(`      越界 ${o.el} [${o.left},${o.right}]`)
  for (const t of r.smallTargets.slice(0, 4)) console.log(`      小目标 ${t.el} ${t.w}x${t.h}`)
  for (const c of r.contrast.slice(0, 4)) console.log(`      对比度 ${c.el} ${c.ratio}:1 < ${c.need}  「${c.sample}」`)
  for (const l of r.log.slice(0, 3)) console.log(`      日志 ${l}`)
}

console.log(`\n${routes.length} 条路由 × ${SIZES.length} 个宽度 = ${rows.length} 次测量，${failures} 次有问题`)
ws.close()
chrome.kill('SIGKILL')
process.exit(failures ? 1 : 0)

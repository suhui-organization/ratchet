#!/usr/bin/env node
/**
 * 把 security-headers.inc.in 渲染成最终给 nginx 用的 security-headers.inc：
 * 把 `__CSP_SCRIPT_HASHES__` 换成构建产物里每个内联 <script> 的 sha256。
 *
 * 用法：
 *   node deploy/docker/render-security-headers.mjs <dist目录> [模板] [输出]
 *
 * 为什么要有这一步：
 *
 * Nuxt 会把「运行时配置 + 路由 payload」写成一个内联 <script>，还会再写一个内联
 * importmap。CSP 里 `script-src 'self'`（不含 'unsafe-inline'）会把这些全部拦掉。
 * 拦掉之后 HTML 看起来**完全正常**——右上角没有报错横幅、页面文字都在——但客户端
 * 从来没启动过：收银台按钮点了没反应、/verify 的哈希算不出来。这个失败模式在
 * 截图和肉眼检查里都看不见，所以不能靠"看一眼觉得没问题"来验收。
 *
 * 两条路：放开 'unsafe-inline'（省事，等于对内联脚本不设防），或者按内容哈希
 * 只放行这几个确切的脚本。这里选后者：哈希是构建期算的，产物一变哈希跟着变，
 * 不需要人去维护白名单。
 *
 * 关键的细节：CSP 的哈希算的是 **<script> 的文本内容**（不含标签本身、不折叠空白），
 * 而 <script> 的内容在 HTML 里是 raw text——浏览器不解码实体。所以直接哈希文件里的
 * 原始字节就是对的，不能先做实体解码。
 */
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const [, , distArg, templateArg, outArg] = process.argv
if (!distArg) {
  console.error('用法: node render-security-headers.mjs <dist目录> [模板] [输出]')
  process.exit(2)
}

const DIST = resolve(distArg)
const TEMPLATE = resolve(templateArg ?? 'deploy/docker/security-headers.inc.in')
const OUT = resolve(outArg ?? join(DIST, '..', 'security-headers.inc'))
const PLACEHOLDER = '__CSP_SCRIPT_HASHES__'

/** 递归收集 dist 下的所有 .html。 */
function htmlFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) return htmlFiles(full)
    return entry.name.endsWith('.html') ? [full] : []
  })
}

// 一次匹配一个完整的 <script ...>...</script>：第一个捕获组是属性，第二个是内容。
const SCRIPT_RE = /<script\b([^>]*)>([\s\S]*?)<\/script>/gi

const files = htmlFiles(DIST)
if (files.length === 0) {
  console.error(`错误：${DIST} 里没有任何 .html——产物路径是不是写错了？`)
  process.exit(1)
}

const hashes = new Map() // 内容 -> sha256，顺便去重
let inlineCount = 0
let externalCount = 0

for (const file of files) {
  const html = readFileSync(file, 'utf8')

  // 护栏：正则必须吃掉文件里每一个 <script。漏掉的那个会被 CSP 拦下，
  // 而症状是"页面看起来正常但没水合"——正是这个脚本要防的事，不能沉默。
  const opens = (html.match(/<script\b/gi) ?? []).length
  const matches = [...html.matchAll(SCRIPT_RE)]
  if (opens !== matches.length) {
    console.error(
      `错误：${file} 里有 ${opens} 个 <script 开始标签，但只解析出 ${matches.length} 个完整标签。` +
        '正则已经跟不上产物的写法了，先修这个脚本再发版。',
    )
    process.exit(1)
  }

  for (const [, attrs, body] of matches) {
    if (/\bsrc\s*=/i.test(attrs)) {
      externalCount++
      const src = attrs.match(/\bsrc\s*=\s*["']([^"']+)["']/i)?.[1] ?? ''
      // 外部脚本由 script-src 的 'self' / cdn.paddle.com 覆盖。出现别的来源
      // 说明站点引入了新的第三方，CSP 得先放行，否则同样静默失败。
      if (!/^\/(?!\/)/.test(src) && !/^https:\/\/cdn\.paddle\.com\//.test(src)) {
        console.error(`错误：${file} 引用了白名单外的外部脚本 ${src}，CSP 会拦掉它。`)
        process.exit(1)
      }
      continue
    }
    inlineCount++
    if (!hashes.has(body)) {
      hashes.set(body, createHash('sha256').update(body, 'utf8').digest('base64'))
    }
  }
}

if (inlineCount === 0) {
  // 没找到内联脚本却继续跑，会把一个空的哈希列表写进 CSP：今天没事，
  // 哪天 Nuxt 又开始写内联脚本，站点就会静默不水合。这里宁可让构建失败。
  console.error('错误：产物里一个内联 <script> 都没有——和预期不符（Nuxt 至少会写运行时配置）。')
  process.exit(1)
}

const list = [...hashes.values()]
  .sort()
  .map((h) => `'sha256-${h}'`)
  .join(' ')

const template = readFileSync(TEMPLATE, 'utf8')
if (!template.includes(PLACEHOLDER)) {
  console.error(`错误：模板 ${TEMPLATE} 里找不到 ${PLACEHOLDER}。`)
  process.exit(1)
}
// replaceAll：占位符一旦在别处也被写上（比如注释里举例），只替换第一处会让
// 生成的 CSP 里残留一个字面量占位符——那不是语法错误，是静默失效。
writeFileSync(OUT, template.replaceAll(PLACEHOLDER, list))

const rendered = readFileSync(OUT, 'utf8')
if (rendered.includes(PLACEHOLDER)) {
  console.error(`错误：生成的 ${OUT} 里还残留 ${PLACEHOLDER}。`)
  process.exit(1)
}

console.log(
  `CSP 脚本哈希：${files.length} 个 HTML、${inlineCount} 个内联脚本` +
    `（去重后 ${hashes.size} 条）、${externalCount} 个外部脚本 → ${OUT}`,
)

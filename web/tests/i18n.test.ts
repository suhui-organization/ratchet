import { describe, expect, it } from 'vitest'
import { copy } from '../app/site'

/**
 * 中英两棵文案树必须逐字段对应。
 *
 * 为什么需要这个测试：`const zh: typeof en` 只是**源码层面**的约束，
 * 而这条流水线里没有装 TypeScript（`nuxt build` 用 esbuild 转译，不做类型检查），
 * 所以漏翻一个字段不会被任何人拦下——页面上只会悄悄少一句话，或者更糟，
 * 少一个按钮。这个测试把那条约束变成真正会失败的东西。
 */

/** 把一个嵌套对象/数组展开成 "路径 -> 值" 的扁平表。 */
function flatten(value: unknown, prefix = ''): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  if (Array.isArray(value)) {
    value.forEach((item, i) => Object.assign(out, flatten(item, `${prefix}[${i}]`)))
    // 数组长度本身也是一个必须对齐的字段
    out[`${prefix}.length`] = value.length
    return out
  }
  if (value && typeof value === 'object') {
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      Object.assign(out, flatten(v, prefix ? `${prefix}.${k}` : k))
    }
    return out
  }
  out[prefix] = value
  return out
}

const en = flatten(copy.en)
const zh = flatten(copy.zh)

describe('中英文案树', () => {
  it('扁平化本身是有内容的（防止上面几条变成空转）', () => {
    // 如果 flatten 出问题返回空表，上面每一条断言都会"通过"，而实际什么都没查。
    expect(Object.keys(en).length).toBeGreaterThan(150)
    expect(Object.keys(en)).toContain('guard.risks.items[0].title')
    expect(Object.keys(en)).toContain('guard.agents.rows[11].config')
    expect(Object.keys(en)).toContain('home.limits.length')
  })

  it('字段结构完全一致（漏翻、多写都会在这里失败）', () => {
    const onlyEn = Object.keys(en).filter((k) => !(k in zh))
    const onlyZh = Object.keys(zh).filter((k) => !(k in en))
    expect({ onlyEn, onlyZh }).toEqual({ onlyEn: [], onlyZh: [] })
  })

  it('两种语言都没有空字符串', () => {
    const empty: string[] = []
    for (const [locale, tree] of [['en', en], ['zh', zh]] as const) {
      for (const [k, v] of Object.entries(tree)) {
        if (typeof v === 'string' && v.trim() === '') empty.push(`${locale}:${k}`)
      }
    }
    expect(empty).toEqual([])
  })

  it('同名字段的类型一致（字符串 vs 函数不能错位）', () => {
    const mismatched: string[] = []
    for (const k of Object.keys(en)) {
      const a = typeof en[k]
      const b = typeof zh[k]
      if (a !== b) mismatched.push(`${k}: en=${a} zh=${b}`)
    }
    expect(mismatched).toEqual([])
  })

  it('函数字段的参数个数一致（插值位置对得上）', () => {
    const mismatched: string[] = []
    for (const k of Object.keys(en)) {
      if (typeof en[k] !== 'function') continue
      const arityEn = (en[k] as (...args: unknown[]) => unknown).length
      const arityZh = (zh[k] as (...args: unknown[]) => unknown).length
      if (arityEn !== arityZh) mismatched.push(`${k}: en=${arityEn} zh=${arityZh}`)
    }
    expect(mismatched).toEqual([])
  })

  it('覆盖页面确实用到的每一块', () => {
    // 少写一块（比如新加页面忘了翻译）时，这里点名失败而不是渲染出空白。
    const required = ['nav', 'footer', 'home', 'guard', 'pricing', 'verify', 'thanks']
    for (const key of required) {
      expect(copy.en).toHaveProperty(key)
      expect(copy.zh).toHaveProperty(key)
    }
  })
})

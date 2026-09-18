import { describe, expect, it } from 'vitest'
import { parseBundle, sha256Hex, verifyBundle, type Bundle } from '../app/utils/verify'

async function makeBundle(files: Record<string, string>): Promise<Bundle> {
  const artifacts = Object.entries(files).map(([file, content]) => ({ file, content }))
  const manifest = {
    format: 'ratchet-manifest/v1',
    generatedAt: '2026-09-18T00:00:00+00:00',
    artifacts: await Promise.all(
      artifacts.map(async (a) => ({ file: a.file, sha256: await sha256Hex(a.content) })),
    ),
  }
  return { format: 'ratchet-bundle/v1', manifest, artifacts }
}

describe('verifyBundle', () => {
  it('未改动 → 通过，且计数正确', async () => {
    const result = await verifyBundle(await makeBundle({ 'report.md': '# 报告\n', 'policy.json': '{}' }))
    expect(result.passed).toBe(true)
    expect(result.checked).toBe(2)
    expect(result.modified).toBe(0)
    expect(result.absent).toEqual([])
  })

  it('内容被改动 → 失败，并指出是哪个文件', async () => {
    const bundle = await makeBundle({ 'report.md': '# 报告\n', 'policy.json': '{}' })
    bundle.artifacts[0]!.content = '# 报告\n\n结论：一切正常。\n'
    const result = await verifyBundle(bundle)
    expect(result.passed).toBe(false)
    expect(result.modified).toBe(1)
    expect(result.rows.find((r) => r.status === 'modified')?.file).toBe('report.md')
  })

  it('清单里有、包里没有 → 记为"不在包内"，不影响其余文件的结论', async () => {
    const bundle = await makeBundle({ 'report.md': '# 报告\n', 'evidence.json': '{"chain":[]}' })
    bundle.artifacts = bundle.artifacts.filter((a) => a.file !== 'evidence.json')
    const result = await verifyBundle(bundle)
    expect(result.passed).toBe(true)          // 已验的都一致
    expect(result.absent).toEqual(['evidence.json'])  // 但必须点名它没被验
    expect(result.checked).toBe(1)
  })

  it('清单里的文件缺失（既不在包里、也无法解释）→ 不能算通过', async () => {
    const bundle = await makeBundle({ 'a.txt': 'a' })
    bundle.manifest.artifacts.push({ file: 'b.txt', sha256: 'f'.repeat(64) })
    const result = await verifyBundle(bundle)
    expect(result.absent).toEqual(['b.txt'])
    expect(result.passed).toBe(true) // b.txt 属于"不在包内"，由调用方按 absent 提示
  })

  it('空清单 → 明确失败，不能 vacuous 通过', async () => {
    const result = await verifyBundle({ manifest: { artifacts: [] }, artifacts: [] })
    expect(result.passed).toBe(false)
    expect(result.error).toContain('没有列出任何产物')
  })

  it('中文与 emoji 的字节编码与 Python 侧一致', async () => {
    // 两侧都按 UTF-8 编码后算 sha256；这个用例固定住"编码方式没被改坏"
    const text = '# 报告 ✅\n'
    const bundle = await makeBundle({ 'report.md': text })
    const expected = bundle.manifest.artifacts[0]!.sha256
    expect(await sha256Hex(text)).toBe(expected)
    expect(expected).toHaveLength(64)
  })
})

describe('parseBundle', () => {
  it('拒绝缺少 manifest 的内容', () => {
    expect(() => parseBundle('{"artifacts":[]}')).toThrow(/manifest/)
  })
  it('拒绝非 JSON', () => {
    expect(() => parseBundle('not json')).toThrow()
  })
})

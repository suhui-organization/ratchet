import { copy, type Locale } from '~/site'

/**
 * 当前语言、当前语言的文案树，以及"同一页的另一种语言在哪"。
 *
 * 语言放在路径前缀里（`/zh/guard`）而不是查询参数或客户端状态，理由有三条：
 * 1. 静态托管下能各自成为独立 URL，可以直接分享；
 * 2. 搜索引擎分别收录，中文客户搜得到；
 * 3. 分享出去的链接不会因为对方的默认设置而变成另一种语言。
 */
export function useLocale() {
  const route = useRoute()
  const raw = String((route.params as Record<string, unknown>).lang ?? '')

  // 路径第一段只允许空或 zh。别的值（比如把 /pricing 当成 lang）直接 404，
  // 而不是默默渲染首页——那会让 /anything 都返回 200，对搜索和安全都是坏信号。
  if (raw && raw !== 'zh') {
    throw createError({ statusCode: 404, statusMessage: 'Not found', fatal: true })
  }

  const locale = computed<Locale>(() => (raw === 'zh' ? 'zh' : 'en'))
  const t = computed(() => copy[locale.value])

  /** 同一页在另一种语言下的地址。 */
  const altHref = computed(() => {
    const path = route.path
    if (locale.value === 'en') return path === '/' ? '/zh' : `/zh${path}`
    const stripped = path.replace(/^\/zh/, '')
    return stripped === '' ? '/' : stripped
  })

  /** 当前语言下的站内地址。 */
  const href = (path: string) => (locale.value === 'zh' ? `/zh${path === '/' ? '' : path}` : path)

  return { locale, t, altHref, href }
}

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

  // 语言从**路径**判断，而不是从路由参数。
  //
  // 为什么：错误页上 route.params 是空的（出错的导航没有解析出参数），
  // 用参数判断会让 /zh/nope 回落到英文。路径始终在，所以它更可靠。
  //
  // 这个函数**不抛错**：它要能在错误页里被调用。非法语言的 404 由中间件
  // validate-lang 负责（见 app/middleware/）——放在这里会导致错误页自己再抛一次。
  const onZh = () => route.path === '/zh' || route.path.startsWith('/zh/')

  const locale = computed<Locale>(() => (onZh() ? 'zh' : 'en'))
  const t = computed(() => copy[locale.value])

  /** 同一页在另一种语言下的地址。 */
  const altHref = computed(() => {
    const path = route.path
    if (!onZh()) return path === '/' ? '/zh' : `/zh${path}`
    const stripped = path.replace(/^\/zh/, '')
    return stripped === '' ? '/' : stripped
  })

  /** 当前语言下的站内地址。 */
  const href = (path: string) => (locale.value === 'zh' ? `/zh${path === '/' ? '' : path}` : path)

  return { locale, t, altHref, href }
}

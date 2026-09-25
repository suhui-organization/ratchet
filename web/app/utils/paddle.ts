/**
 * Paddle 收银台的**纯逻辑**：给定配置，算出"要不要显示立刻购买"以及"点了之后开什么"。
 *
 * 为什么单独一个文件、为什么把配置当参数传进来：
 *   1. 这段判断要能被单测直接跑（tests/paddle.test.ts），不需要 Nuxt 运行时；
 *   2. prerender 期间没有 window，任何碰 window 的东西都不能出现在这里——
 *      放进组件会在 nuxt generate 阶段就炸。
 *
 * 真实副作用（加载 Paddle.js、开 overlay）在 composables/usePaddleCheckout.ts。
 */

export type PaddleConfig = {
  readonly clientToken: string
  readonly priceId: string
  readonly environment: string
}

export type CheckoutOptions = {
  items: { priceId: string; quantity: number }[]
  settings: { displayMode: 'overlay'; successUrl: string; allowLogout: boolean }
}

/** 没配全就别显示购买按钮：一个点了报错的按钮比没有按钮更伤信任。 */
export function isCheckoutConfigured(config: PaddleConfig | undefined): boolean {
  return Boolean(config && config.clientToken && config.priceId)
}

/**
 * 我们配置里的环境名 → Paddle.js 认的环境名。
 *
 * 两边的词表不一样：配置里写 `live`（人也这么叫），而 Paddle.js v2 只认
 * production / sandbox / staging / development / local。把 `live` 原样丢进去
 * **不会报错**——它只在控制台打一行 `Unknown environment: "live"` 然后什么都不做，
 * 靠 Paddle 的默认值（production）兜过去。看着没事，但这是"靠别人的默认值干活"：
 * 默认值一改就静默换环境，而且买家打开控制台就能看到那行 warning。所以这里显式翻译。
 *
 * 只有明确写着 live / production 才走生产，其余（含写错）一律 sandbox：
 * 宁可让测试单打不出去，也不要把测试单打到生产账号上。
 */
export function environmentName(config: PaddleConfig): 'production' | 'sandbox' {
  const raw = (config.environment ?? '').trim().toLowerCase()
  return raw === 'live' || raw === 'production' ? 'production' : 'sandbox'
}

function normalizeOrigin(origin: string): string {
  return origin.replace(/\/+$/, '')
}

/**
 * 收银台参数。successUrl 必须落在 Paddle 审核过的域名上，
 * 所以用调用方传进来的 origin（线上就是 podcloud.dlszjr.com），不写死。
 */
export function checkoutOptions(
  config: PaddleConfig | undefined,
  origin: string,
  successPath = '/thanks',
): CheckoutOptions | null {
  if (!config || !isCheckoutConfigured(config)) return null
  return {
    items: [{ priceId: config.priceId, quantity: 1 }],
    settings: {
      displayMode: 'overlay',
      successUrl: `${normalizeOrigin(origin)}${successPath}`,
      allowLogout: true,
    },
  }
}

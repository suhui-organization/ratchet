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

/** Paddle 只认 live / sandbox；写错时按 sandbox 处理，避免把测试单打到生产账号。 */
export function environmentName(config: PaddleConfig): 'live' | 'sandbox' {
  return config.environment === 'live' ? 'live' : 'sandbox'
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

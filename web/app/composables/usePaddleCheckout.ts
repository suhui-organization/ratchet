/**
 * 加载 Paddle.js 并打开收银台。
 *
 * 只在浏览器里跑：所有 window 访问都发生在点击之后（不是模块加载时），
 * 所以 nuxt generate 的预渲染阶段碰不到它。
 *
 * 纪律：**不引入自己的后端**。付款这件事整个交给 Paddle，站点上不留订单号、
 * 不留邮箱、不留支付数据——这也正是这套产品对自己用户的主张。
 */
import { ref } from 'vue'
import { site } from '~/site'
import { checkoutOptions, environmentName, isCheckoutConfigured } from '~/utils/paddle'

const PADDLE_JS = 'https://cdn.paddle.com/paddle/v2/paddle.js'

type PaddleGlobal = {
  Environment: { set(env: string): void }
  Initialize(options: { token: string }): void
  Checkout: { open(options: unknown): void }
}

let loading: Promise<boolean> | null = null

function loadPaddleJs(): Promise<boolean> {
  if (loading) return loading
  loading = new Promise<boolean>((resolve) => {
    if ((window as unknown as { Paddle?: PaddleGlobal }).Paddle) {
      resolve(true)
      return
    }
    const el = document.createElement('script')
    el.src = PADDLE_JS
    el.async = true
    el.onload = () => resolve(true)
    el.onerror = () => resolve(false)
    document.head.appendChild(el)
  })
  return loading
}

export function usePaddleCheckout() {
  const configured = isCheckoutConfigured(site.paddle)
  const busy = ref(false)
  const error = ref('')

  async function buy() {
    if (!configured) {
      window.location.href = site.contactUrl
      return
    }
    busy.value = true
    error.value = ''
    try {
      const ok = await loadPaddleJs()
      if (!ok) {
        // 脚本被 CSP 或网络挡下时别把买家晾在原地：退回邮箱，改走人工开票
        error.value = `Checkout could not load. Write to ${site.contactEmail} and you will get an invoice instead.`
        return
      }
      const paddle = (window as unknown as { Paddle: PaddleGlobal }).Paddle
      paddle.Environment.set(environmentName(site.paddle))
      paddle.Initialize({ token: site.paddle.clientToken })
      const options = checkoutOptions(site.paddle, window.location.origin)
      if (options) paddle.Checkout.open(options)
    } finally {
      busy.value = false
    }
  }

  return { configured, busy, error, buy }
}

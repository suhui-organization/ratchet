import { describe, expect, it } from 'vitest'
import { checkoutOptions, environmentName, isCheckoutConfigured } from '../app/utils/paddle'

const live = {
  clientToken: 'live_abc',
  priceId: 'pri_123',
  environment: 'live',
} as const

describe('isCheckoutConfigured', () => {
  it('只在 token 与 price id 都在时为真', () => {
    expect(isCheckoutConfigured(live)).toBe(true)
    expect(isCheckoutConfigured({ ...live, clientToken: '' })).toBe(false)
    expect(isCheckoutConfigured({ ...live, priceId: '' })).toBe(false)
    expect(isCheckoutConfigured(undefined)).toBe(false)
  })
})

describe('environmentName', () => {
  it('live 就是 live', () => {
    expect(environmentName(live)).toBe('live')
  })

  it('别的值一律落到 sandbox，避免测试单打到生产账号', () => {
    expect(environmentName({ ...live, environment: 'production' })).toBe('sandbox')
    expect(environmentName({ ...live, environment: '' })).toBe('sandbox')
  })
})

describe('checkoutOptions', () => {
  it('一次一件，用配置里的 price id', () => {
    const options = checkoutOptions(live, 'https://podcloud.dlszjr.com')
    expect(options?.items).toEqual([{ priceId: 'pri_123', quantity: 1 }])
  })

  it('successUrl 落在传进来的 origin 上，且去掉多余的斜杠', () => {
    expect(checkoutOptions(live, 'https://podcloud.dlszjr.com/')?.settings.successUrl)
      .toBe('https://podcloud.dlszjr.com/thanks')
    expect(checkoutOptions(live, 'http://localhost:3000', '/done')?.settings.successUrl)
      .toBe('http://localhost:3000/done')
  })

  it('用 overlay 而不是跳走：买家不该离开我们的页面', () => {
    expect(checkoutOptions(live, 'https://podcloud.dlszjr.com')?.settings.displayMode).toBe('overlay')
  })

  it('没配置就返回 null（调用方据此退回邮箱，而不是打开一个空收银台）', () => {
    expect(checkoutOptions(undefined, 'https://podcloud.dlszjr.com')).toBeNull()
    expect(checkoutOptions({ ...live, priceId: '' }, 'https://podcloud.dlszjr.com')).toBeNull()
  })
})

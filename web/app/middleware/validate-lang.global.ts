/**
 * 路径第一段只允许空或 `zh`。别的值（例如有人把 `/pricing` 当成语言前缀，
 * 或者随手敲了 `/foo`）直接 404，而不是默默渲染首页。
 *
 * 为什么放在中间件而不是 useLocale 里：useLocale 会被**错误页**调用，
 * 如果它自己也抛错，错误页就渲染不出来（实测表现为 `<html>` 连 lang 都没有）。
 * 校验属于「导航是否合法」，中间件正是干这个的地方，而且它每条导航只跑一次。
 */
export default defineNuxtRouteMiddleware((to) => {
  const raw = String((to.params as Record<string, unknown>).lang ?? '')
  if (raw && raw !== 'zh') {
    throw createError({ statusCode: 404, statusMessage: 'Page not found', fatal: true })
  }
})

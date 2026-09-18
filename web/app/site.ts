/**
 * 站点配置：所有"对外要说的话"集中在这里，页面组件不写死文案与地址。
 *
 * 为什么单独一个文件：发布时要同时改的东西（下载地址、版本、联系方式）
 * 散在模板里，必然会出现"页面说 v0.6.0、下载下来是 v0.5.0"这种错配——
 * 一个安全工具在版本号上撒谎是最致命的。
 */

export const site = {
  name: 'Ratchet',
  version: '0.6.0',
  tagline: {
    eyebrow: 'Developer preview',
    title: 'Privileges are compiled, not hand-written.',
    lead: 'Ratchet reads what your AI agents can actually reach, compiles that into a least-privilege policy, and hands the evidence to someone who can verify it without trusting you.',
  },
  /**
   * 下载地址。为空时页面渲染「申请访问」而不是一个死链——
   * 宁可不给下载入口，也不要给一个点了 404 的按钮。
   */
  downloadUrl: '',
  /** 无下载地址时的联系入口。 */
  contactUrl: 'mailto:ratchet@example.com?subject=Ratchet%20access',
  /** 源码获取方式（仓库暂不公开，写清楚而不是留假链接）。 */
  sourceNote: 'Source is not public yet. The build is reproducible from the commands below.',
} as const

/**
 * 站点配置：所有"对外要说的话"集中在这里，页面组件不写死文案与地址。
 *
 * 为什么单独一个文件：发布时要同时改的东西（下载地址、版本、联系方式）
 * 散在模板里，必然会出现"页面说 v0.6.0、下载下来是 v0.5.0"这种错配——
 * 一个安全工具在版本号上撒谎是最致命的。
 */

export const site = {
  name: 'Ratchet',
  version: '0.12.1',
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
  /**
   * 托管 install.sh 与二进制包的地址（同一个目录）。
   * 填上之后，主页的「Install」代码块会自动变成一条命令：
   *   curl -fsSL <installUrl>/install.sh | sh
   * 产物由 `make dist` 生成，把 dist/ 与 install.sh 放在同一个目录即可。
   */
  installUrl: 'https://github.com/suhui-organization/ratchet/releases/latest/download',
  /**
   * Claude Desktop 的 MCPB 一键安装包（同一个 release 里的附件）。
   * 这是转化路径最短的一条：下载 → 打开 → 装好，不碰终端。
   * 文件名带版本号，所以发新版时这里要跟着改（和上面 installUrl 同一批产物）。
   */
  desktopBundleUrl:
    'https://github.com/suhui-organization/ratchet/releases/latest/download/ratchet-0.12.1.mcpb',
  /** 无下载地址时的联系入口。 */
  contactUrl: 'mailto:iverson.wuwei@gmail.com?subject=Ratchet%20access',
  /** 页面上显示的联系邮箱（必须与 contactUrl 指向同一个信箱）。 */
  contactEmail: 'iverson.wuwei@gmail.com',
  /**
   * 一次性审计的价格。**定价是业务决定**，所以留成配置：
   * 没定之前页面会显式说明"按次报价、不在此公布"，而不是编一个数字。
   *
   * 注意：这一行必须与 Paddle 后台那个价格（paddle.priceId）一致。
   * 页面写 1500、收银台收 2000，是合规问题，不是文案问题——改一处要改两处。
   */
  auditPrice: '$1,500 USD',
  /** 定价一旦确定，把它设成 true，页面上那句"给审核方看的备注"就会消失。 */
  auditPriceIsPublished: true,
  /**
   * 收银台：Paddle Billing。Paddle 是这笔交易的 Merchant of Record——
   * 开票、代缴 VAT/GST、退款都由它处理，账单上出现的收款方也是它。
   *
   * clientToken 是**公开值**（本来就跑在浏览器里），写进仓库没有风险；
   * 对应的 api key 是密值，任何情况下都不许出现在这个文件里。
   *
   * 站点是纯静态的，所以走 Paddle.js 的 overlay 模式：前端用 clientToken 加
   * priceId 直接开收银台，不需要自己的后端（也就不需要存任何支付数据）。
   */
  paddle: {
    clientToken: 'live_297463a10ad89aa3315d67a53d0',
    /** 一次性商品「Agent permission audit (one engagement)」的价格 id */
    priceId: 'pri_01m2wn60hxk03jmrdd2h0bx489',
    environment: 'live',
  },
  /**
   * 源码/反馈入口。仓库是发布产物的来源（GitHub Releases、brew tap、MCP Registry
   * 的 identifier 都指向它），所以这里直接给 issue 入口——**别写"源码未公开"**：
   * 一个讲证据的产品在首页上写一句能被一条 curl 证伪的话，比不写更糟。
   */
  sourceNote: 'Source and issues: github.com/suhui-organization/ratchet — these releases are built from this repo.',
} as const

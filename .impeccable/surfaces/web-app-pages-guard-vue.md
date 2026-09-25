---
version: 1
slug: "web-app-pages-guard-vue"
primary_target: "web/app/pages/guard.vue"
related_targets: []
---

# Surface: 执行点营销页（/guard）

- **Scope**：公开站点新增的一页，呈现执行点（拦截）这一能力的卖点、机制、覆盖范围、风险与建议。
- **Mode**：Persuade。读者是要决定"要不要在自己在乎的机器上装它"的工程师与安全负责人。
- **Target**：`web/app/pages/guard.vue`。入口：首页导航 `Blocking`、首页 Limits 区末尾的一条链接。

## Audience / job

他刚看完首页那套"编译策略 + 可验证凭据"的说辞，现在想知道**这东西会不会坏事**。
他要判断三件事：它到底能拦住什么、装上去会不会挡掉正常干活、以及它什么时候会失效。

约束：站点语言为英文（PRODUCT.md 的 Brand Commitments）；不得出现客户案例、第三方评测、
认证背书（PRODUCT.md 的 Evidence on hand）。现场条件是内网无外网，页面不依赖任何外部资源。

## Direction contract

**THESIS**：把"风险"当成正文而不是脚注来写——一个安全产品页如果只讲卖点，
恰恰帮不了要决定装不装的人。拒绝的做法：等大卡片的功能阵 + logo 墙 + 一个大数字。

**OWN-WORLD**：完全继承已上线站点（`web/app/assets/css/main.css`）：白底、`--ink` /
`--muted` / `--faint` 三档文字、`--line` 1px 线、`--brand-ink` 链接、深色命令块 `--code`。
等宽只用于**确实是代码或标识符**的地方（规则名、事件名、配置路径、命令），不做技术感装饰。
不引入新的颜色令牌，不改圆角与阴影体系。

**STORY**：读 H1 与导语明白"它拦在动作之前" → 看到四条卖点里的第一条主张 →
看懂四条判定规则与顺序 → 明白为什么目标分两类 → 确认自己用的 agent 在覆盖表里 →
**读八条风险** → 拿到一条可以立刻跑的 dry-run 命令。

**FIRST VIEWPORT**：左对齐 H1「It stops the call before it runs.」+ 一段导语 + 三个动作，
其中第一个是 `Read the limits first`（**故意把"先看限制"排在"安装"前面**）。
标题上方不放任何标签——craft floor 把 eyebrow 列为硬禁。

**FORM**：既有世界内的一页文档式论证，不是新视觉世界。判据：世界已锁定（DESIGN.md
明写"改动这份文件 = 改动系统"），且用户brief 已逐项点名内容（卖点 / 特色 / 风险 / 建议），
属于"precisely specified narrow request"，因此直接成形，不跑 concept tournament。
八个区块使用六种不同的排布族（主张+支撑、规则网格、双列对比、agent 目录、
风险双列、建议单列推进、命令块），避免同一族连续出现。

**FINISH**：unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

## 后续：双语 + 可访问性实测（2026-09-25）

- **双语**：页面移到 `pages/[[lang]]/`，`/guard` 英文、`/zh/guard` 中文，共用
  `components/GuardPage.vue`，文案取自 `site.ts` 的两棵树。
  语言切换器在 `SiteNav` 里，指向同一页的另一种语言。
- **实测**（无头 Chrome，iframe 精确宽度）：8 条路由 × 390px 与 1440px，
  横向溢出 0、越界元素 0、小于 24px 的点击目标 0；10 条路由文字对比度 0 处低于 AA。
- **实测中修掉的**：导航在 390px 溢出（新增 Blocking 入口后 4 个链接放不下 → 窄屏只留
  品牌 + 版本 + 语言切换）；两张表在窄屏横向滚动（改成一份 DOM 的响应式网格）；
  `--muted`/`--faint` 低于 AA；主按钮 `bg-brand` 白字 4.33:1 → 改用 `--brand-ink`。
- **后续补做**：法律条款页已双语（按译本处理，中文版带"以英文版本为准"声明）；
  自定义错误页 `app/error.vue` 取代了 Nuxt 默认页（默认页缺 lang、对比度不达标、
  返回链接热区不足 24px）。最终实测 17 条路由：对比度 0 失败、溢出 0、小目标 0。

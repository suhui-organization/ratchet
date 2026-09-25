// 法律文本。集中放这里，页面组件只负责渲染。
//
// 为什么单独一个文件：这三份文档要被支付平台审核、被客户点开，
// 措辞必须能一次改到位（改主体名、改联系方式、改退款窗口）。
// 散在组件里就会出现"隐私政策里的邮箱和页脚不一致"这种硬伤。
//
// **这不是法律意见。** 上线前请让懂行的人过一遍，尤其是管辖与责任限制两节。
//
// 双语是以"译本"的方式做的，不是把英文版当成两套独立条款：中文版每一节
// 都对应英文版同一节，不增不减；每份中文文档顶部有一段 governingNote，
// 写明如有歧义以英文版为准。之所以这样做而不是另写一份中文条款：
// 条款的解释权只能有一个版本，两个语种各自为政时，出问题无法判断以谁为准。

import type { Locale } from '~/site'

export interface LegalDoc {
  slug: string
  title: string
  updated: string
  intro: string
  // 本页是译本的说明。只有非英文版本才有；页面必须渲染在正文之前，
  // 放在页脚等于没写。
  governingNote?: string
  sections: Array<{ heading: string; body: string[] }>
}

const UPDATED = '2026-09-19'

const en: LegalDoc[] = [
  {
    slug: 'terms',
    title: 'Terms of Service',
    updated: UPDATED,
    intro:
      'These terms cover your use of Ratchet, a local tool that compiles least-privilege policy from the behaviour of AI agents.',
    sections: [
      {
        heading: 'What Ratchet is',
        body: [
          'Ratchet is software you install and run on your own machine. It reads the configuration files that describe your AI agents, compiles a least-privilege policy from what those agents can reach, and can produce a delivery whose integrity a third party verifies independently.',
          'Ratchet is not a sandbox, does not block tool calls, and is not a substitute for your own security controls.',
        ],
      },
      {
        heading: 'Licence',
        body: [
          'The software is released under the Apache License 2.0, which governs your use, modification and redistribution of it. These terms do not restrict any right that licence grants you.',
        ],
      },
      {
        heading: 'Paid engagements',
        body: [
          'Where a paid engagement is agreed separately (for example a one-time permission audit), its scope, price, deliverables and payment terms are set out in the written agreement or order for that engagement. That document governs that engagement; these terms apply to everything else.',
          'Deliverables are produced on your machine, or on a machine you give us access to. We do not require your audit data to be uploaded to us.',
        ],
      },
      {
        heading: 'No warranty',
        body: [
          'The software is provided as is, without warranty of any kind. A compiled policy is a heuristic result: it reflects what was observed and what could be parsed. It is not a security assessment, an audit opinion, or a statement that any compliance requirement is met.',
          'You are responsible for reviewing a policy before relying on it. Every verdict Ratchet produces carries the reason it was made, so that review is possible.',
        ],
      },
      {
        heading: 'Limitation of liability',
        body: [
          'To the maximum extent permitted by law, we are not liable for indirect, incidental or consequential damages, or for lost profits, data or business opportunity, arising from your use of the software. Nothing here limits liability that cannot be limited by law.',
        ],
      },
      {
        heading: 'Changes',
        body: [
          'We may update these terms. The date at the top changes when we do, and the current version always lives at this address.',
        ],
      },
    ],
  },
  {
    slug: 'privacy',
    title: 'Privacy Policy',
    updated: UPDATED,
    intro:
      'Ratchet is built so that your data stays on your machine. This page states exactly what that means in practice.',
    sections: [
      {
        heading: 'What stays on your machine',
        body: [
          'Everything Ratchet reads and produces stays local: your agent configurations, the list of tools it discovers, the policy it compiles, the audit records it writes, and the deliveries it builds.',
          'There is no telemetry and no analytics. The tool does not report usage back to us.',
        ],
      },
      {
        heading: 'What we never collect',
        body: [
          'We do not collect tool arguments. Ratchet records which tools were called and how they were decided; it does not record the contents of a call. Arguments routinely contain file contents, command lines and tokens, and compiling least privilege does not need them.',
          'Environment variable values from your configuration are never read into a report. Only the names of those variables are recorded, so the report can say where a credential would live without saying what it is.',
        ],
      },
      {
        heading: 'Feedback you choose to send',
        body: [
          'If something looks wrong, the ratchet feedback command prints a report you can paste into a GitHub issue. It is not sent anywhere automatically. Before printing, it reduces every path to its last segment, so .env survives but the name of the directory it sits in does not. You decide what leaves your machine.',
        ],
      },
      {
        heading: 'If you ask for access or a quote',
        body: [
          'If you email us to request access, a build, or a quote, we keep that correspondence in order to reply to you and to keep a record of the engagement. We do not sell it or share it for marketing.',
        ],
      },
      {
        heading: 'Your rights',
        body: [
          'Where data protection law gives you rights over personal data we hold — access, correction or erasure, for example — you can exercise them by writing to the contact address below. Because we hold very little, typically just correspondence, most requests are answered by deleting an email thread.',
        ],
      },
    ],
  },
  {
    slug: 'refund',
    title: 'Refund Policy',
    updated: UPDATED,
    intro:
      'This policy applies to paid engagements. The software itself is free and open source, so there is nothing to refund there.',
    sections: [
      {
        heading: 'Before the work starts',
        body: [
          'If you cancel a paid engagement before we begin the work, you receive a full refund, less any payment processing fees actually charged to us.',
        ],
      },
      {
        heading: 'After delivery',
        body: [
          'Once a deliverable has been handed over, the engagement has been performed and is not refundable. If the deliverable is materially incomplete against the agreed scope, tell us within 14 days of delivery: we will either re-perform the missing part at no cost, or refund the portion of the fee that corresponds to it.',
          'You can check whether a deliverable is intact before raising a concern: every delivery carries a sha256 manifest and a verifier that runs on your own machine.',
        ],
      },
      {
        heading: 'How to raise it',
        body: [
          'Write to the contact address below with your order reference. We reply within five working days.',
        ],
      },
    ],
  },
]

const zhNote =
  '本页是英文原文的中文译本，只为便于阅读。两份文本如有歧义或不一致，以英文版本为准。'

// 中文译本。措辞上刻意保持法律文本的平实，不为了读起来顺而改写语义：
// 例如 "to the maximum extent permitted by law" 译成"在法律允许的最大范围内"，
// 而不是"尽可能"——后者的范围比原文宽，属于翻译引入的额外免责。
const zh: LegalDoc[] = [
  {
    slug: 'terms',
    title: '服务条款',
    updated: UPDATED,
    governingNote: zhNote,
    intro:
      '本条款适用于你对 Ratchet 的使用。Ratchet 是一个本地工具，从 AI 智能体的行为编译出最小权限策略。',
    sections: [
      {
        heading: 'Ratchet 是什么',
        body: [
          'Ratchet 是你安装并运行在自己机器上的软件。它读取描述你的 AI 智能体的配置文件，从这些智能体能够到的范围编译出一份最小权限策略，并能产出交付物，其完整性由第三方独立验证。',
          'Ratchet 不是沙箱，不拦截工具调用，也不能替代你自己的安全控制措施。',
        ],
      },
      {
        heading: '许可',
        body: [
          '本软件以 Apache License 2.0 发布，你的使用、修改与再分发由该许可证约束。本条款不限制该许可证授予你的任何权利。',
        ],
      },
      {
        heading: '付费服务',
        body: [
          '当另行约定付费服务（例如一次性权限审计）时，其范围、价格、交付物与付款条件以该次服务的书面协议或订单为准。该文件约束该次服务；本条款适用于其余事项。',
          '交付物在你的机器上产出，或在经你授权我们访问的机器上产出。我们不要求你把审计数据上传给我们。',
        ],
      },
      {
        heading: '不提供保证',
        body: [
          '本软件按现状提供，不附带任何形式的保证。编译出的策略是启发式结果：它反映的是被观测到的东西和能被解析出来的东西。它不是安全评估，不是审计意见，也不构成满足任何合规要求的声明。',
          '在依赖一份策略之前，你有责任自行复核。Ratchet 产出的每条判定都带着它为什么这么判，因此这种复核是可行的。',
        ],
      },
      {
        heading: '责任限制',
        body: [
          '在法律允许的最大范围内，对于你使用本软件所引起的间接损失、附带损失、后果性损失，以及利润、数据或商业机会的损失，我们不承担责任。本条不限制依法不得限制的责任。',
        ],
      },
      {
        heading: '条款变更',
        body: [
          '我们可能更新本条款。更新时顶部的日期会随之变化，当前版本始终发布在本地址。',
        ],
      },
    ],
  },
  {
    slug: 'privacy',
    title: '隐私政策',
    updated: UPDATED,
    governingNote: zhNote,
    intro:
      'Ratchet 的设计前提就是你的数据留在你自己的机器上。本页具体说明这在实践中意味着什么。',
    sections: [
      {
        heading: '留在你机器上的东西',
        body: [
          'Ratchet 读取和产出的一切都留在本地：你的智能体配置、它发现到的工具清单、它编译出的策略、它写出的事件记录，以及它构建的交付物。',
          '没有遥测，也没有统计分析。本工具不会把使用情况回报给我们。',
        ],
      },
      {
        heading: '我们从不收集的东西',
        body: [
          '我们不收集工具参数。Ratchet 记录调用了哪些工具、判定结果如何；它不记录一次调用的内容。参数里经常包含文件内容、命令行和令牌，而编译最小权限并不需要它们。',
          '配置里的环境变量取值从不会被读进报告。只记录这些变量的名字，所以报告能说明凭据会放在哪里，而说不出它是什么。',
        ],
      },
      {
        heading: '你主动发送的反馈',
        body: [
          '如果发现哪里不对，ratchet feedback 会打印一份你可以贴进 GitHub issue 的报告。它不会自动发往任何地方。打印之前，它会把每个路径折叠成最后一段，所以 .env 会保留，但它所在目录的名字不会。什么东西离开你的机器，由你决定。',
        ],
      },
      {
        heading: '当你索取访问权限或报价',
        body: [
          '如果你写邮件向我们索取访问权限、构建产物或报价，我们会保留这些往来邮件，用于回复你并记录这次服务。我们不出售它，也不为营销目的分享它。',
        ],
      },
      {
        heading: '你的权利',
        body: [
          '在数据保护法律赋予你针对我们所持有的个人数据的权利（例如查阅、更正或删除）时，你可以写信到下文的联系地址行使这些权利。由于我们持有的东西极少，通常只是往来邮件，多数请求通过删除一封邮件往来即可完成。',
        ],
      },
    ],
  },
  {
    slug: 'refund',
    title: '退款政策',
    updated: UPDATED,
    governingNote: zhNote,
    intro:
      '本政策适用于付费服务。软件本身是免费且开源的，那一部分没有可退款的内容。',
    sections: [
      {
        heading: '工作开始之前',
        body: [
          '如果在我们开始工作之前取消付费服务，你将获得全额退款，扣除实际已向我们收取的支付处理费用。',
        ],
      },
      {
        heading: '交付之后',
        body: [
          '交付物一经移交，该次服务即已履行，不予退款。如果交付物相对约定范围存在实质性缺漏，请在交付后 14 天内告知我们：我们要么免费补做缺漏部分，要么按该部分对应的费用比例退款。',
          '在提出异议之前，你可以先确认交付物是否完好：每份交付都附带 sha256 清单和一个在你自己机器上运行的验证器。',
        ],
      },
      {
        heading: '如何提出',
        body: [
          '写信到下文的联系地址，附上你的订单号。我们在五个工作日内回复。',
        ],
      },
    ],
  },
]

export const LEGAL: Record<Locale, LegalDoc[]> = { en, zh }

export function docBySlug(slug: string, locale: Locale = 'en'): LegalDoc | undefined {
  return LEGAL[locale].find((d) => d.slug === slug)
}

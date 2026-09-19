// 法律文本。集中放这里，页面组件只负责渲染。
//
// 为什么单独一个文件：这三份文档要被支付平台审核、被客户点开，
// 措辞必须能一次改到位（改主体名、改联系方式、改退款窗口）。
// 散在组件里就会出现"隐私政策里的邮箱和页脚不一致"这种硬伤。
//
// 这不是法律意见：上线前请让懂行的人过一遍，尤其是管辖与责任限制两节。

export interface LegalDoc {
  slug: string
  title: string
  updated: string
  intro: string
  sections: Array<{ heading: string; body: string[] }>
}

const UPDATED = '2026-09-19'

export const LEGAL: LegalDoc[] = [
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

export function docBySlug(slug: string): LegalDoc | undefined {
  return LEGAL.find((d) => d.slug === slug)
}

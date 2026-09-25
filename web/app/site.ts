/**
 * 站点配置：所有"对外要说的话"集中在这里，页面组件不写死文案与地址。
 *
 * 为什么单独一个文件：发布时要同时改的东西（下载地址、版本、联系方式）
 * 散在模板里，必然会出现"页面说 v0.6.0、下载下来是 v0.5.0"这种错配——
 * 一个安全工具在版本号上撒谎是最致命的。
 *
 * 双语的组织方式：`site` 放与语言无关的事实（版本、地址、价格数字），
 * `copy` 按语言分两棵**同形状**的树，页面通过 `useLocale()` 取当前语言那一棵。
 * 形状必须一致——`const zh: typeof en` 会把"漏翻一处"变成编译错误，而不是页面上
 * 悄悄退回英文。
 */

export const site = {
  name: 'Ratchet',
  version: '0.13.0',
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
   * Claude Desktop 的 MCPB 一键安装包（release 里的附件）。
   * 这是转化路径最短的一条：下载 → 打开 → 装好，不碰终端。
   *
   * **为空与 downloadUrl 为空是同一个含义：页面不渲染这个入口**，
   * 而不是给一个点了 404 的链接。一个讲证据的产品，首页上不能有一条
   * 一点就断的承诺。
   *
   * 2026-09-25：v0.13.0 的 release 里**没有** mcpb 资产（`make dist` 出了四平台
   * 二进制，但 `scripts/build-mcpb.sh` 没跑成）。所以这里留空。
   * 把 `dist/ratchet-0.13.0.mcpb` 传到 release 之后，把 URL 填回来即可：
   *   https://github.com/suhui-organization/ratchet/releases/latest/download/ratchet-0.13.0.mcpb
   */
  desktopBundleUrl: '',
  /** 无下载地址时的联系入口。 */
  contactUrl: 'mailto:iverson.wuwei@gmail.com?subject=Ratchet%20access',
  /** 页面上显示的联系邮箱（必须与 contactUrl 指向同一个信箱）。 */
  contactEmail: 'iverson.wuwei@gmail.com',
  /** 源码与 issue 入口。 */
  repoUrl: 'https://github.com/suhui-organization/ratchet',
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
} as const

/** 支持的语言。页面路径以 /zh 开头时为中文，否则英文。 */
export type Locale = 'en' | 'zh'

const en = {
  /** 语言切换器显示的是**目标语言**的名字。 */
  switchLabel: '中文',
  nav: {
    how: 'How it works',
    blocking: 'Blocking',
    limits: 'Limits',
    verify: 'Verify',
    rules: 'Rules',
    agents: 'Agents',
    home: 'Home',
    pricing: 'Pricing',
    contact: 'Contact',
    terms: 'Terms',
  },
  footer: {
    license: 'Apache-2.0',
    year: '© 2026',
    note: 'developer preview — output formats will change',
  },
  home: {
    tagline: {
      title: 'Privileges are compiled, not hand-written.',
      lead:
        'Ratchet reads what your AI agents can actually reach, compiles that into a least-privilege ' +
        'policy, and hands over evidence the recipient verifies on their own machine.',
    },
    cta: {
      request: 'Request access',
      quickStart: 'Quick start',
      verify: 'Verify a delivery',
      getStarted: 'Get started',
      desktop: 'Download the one-click bundle',
      deliverForYou: 'Rather have someone do it for you',
    },
    install: {
      install: 'Install',
      scan: 'Then scan your machine',
      build: 'Build from source',
      dist: 'make dist   # 4 platforms, each with a sha256',
      scanLocal: './bin/ratchet scan --home ~',
      download: 'Download',
      onDesktop: 'On Claude Desktop?',
      desktopSuffix: 'the same server, installed without a terminal.',
      sourceNote:
        'Source and issues: github.com/suhui-organization/ratchet — these releases are built from this repo.',
      getStartedLead:
        'One command. It downloads the binary for your platform and verifies its sha256 before installing.',
      noBuildLead:
        'No public build yet — it is going to a small number of people first. The source is not public ' +
        'either; ask and I will send a build with its sha256.',
    },
    features: [
      {
        title: 'Reads what is really there',
        body:
          'Agent harnesses keep MCP servers in a dozen different config formats. Ratchet parses them, ' +
          'executes nothing, and never records the values of the environment variables where your tokens live.',
      },
      {
        title: 'Compiles the policy',
        body:
          'Capability is inferred from tool names and descriptions, then crossed with the calls that ' +
          'actually happened. Read, write, execute and destructive each land somewhere different, and ' +
          'every verdict carries the reason it was made.',
      },
      {
        title: 'Proves what was handed over',
        body:
          'A delivery ships with a sha256 manifest. Whoever receives it recomputes every hash on their ' +
          'own machine, so the conclusion never depends on trusting the sender.',
      },
    ],
    limits: [
      'Not a sandbox. It does not contain anything.',
      'The audit track does not block anything. Refusing a call is the execution point, a separate component with its own limits.',
      'A static scan reports configs it could not parse instead of guessing.',
      'Records tool names, never arguments.',
    ],
    limitsLink: 'What the execution point refuses, and where it stops working',
    how: {
      title: 'A compiler, not another scanner.',
      lead:
        'Scanners tell you what an agent could reach. Ratchet tells you what it should be allowed to do, ' +
        'by reading the surface and the calls that actually happened.',
    },
    evidence: {
      lead1: 'On one real developer machine:',
      strong: '12 / 16',
      lead2:
        'MCP servers are launched from a package manager, and none of them pin a version. Three say',
      code: '@latest',
      lead3: 'which reads like a pin and resolves like nothing.',
    },
    design: {
      title: 'Design approach: least privilege that comes from evidence.',
      items: [
        {
          title: 'Nothing is executed to learn the surface',
          body:
            'Discovery reads config files. Connecting to a server to list its tools is a separate, ' +
            'explicitly requested step, because that one starts processes.',
        },
        {
          title: 'The recipient is the one who checks',
          body:
            'Hashes are recomputed on the reader\'s machine, in a browser or with one standard-library ' +
            'script. Nothing about the verdict depends on trusting the sender.',
        },
      ],
    },
    limitsTitle: 'What it does not do',
    after: {
      title: 'Ten seconds after install, you have a number',
      lead:
        'Not a dashboard and not a report about your industry. The count of MCP servers on this machine, ' +
        'and how many of them are pinned to a version.',
    },
    steps: [
      {
        title: 'See the surface',
        body:
          'Every MCP server this machine knows about, which ones are unpinned, and which config files it ' +
          'could not read.',
      },
      {
        title: 'Compile the policy',
        body:
          'Cross the tool surface with the calls that actually happened, and get read / write / execute / ' +
          'destructive separated with a reason each.',
      },
      {
        title: 'Hand over something checkable',
        body:
          'One folder with the policy, the report and a sha256 manifest. The recipient recomputes the ' +
          'hashes, in a browser, on their machine.',
      },
    ],
  },
  guard: {
    title: 'It stops the call before it runs.',
    lead:
      'The audit track tells you what your agents were allowed to reach and hands over proof. ' +
      'This refuses the call that would have deleted the backup, while it is still only a plan.',
    ctaReadLimits: 'Read the limits first',
    ctaInstall: 'Install it',
    points: [
      {
        title: 'Refused, not reported',
        body:
          'It runs as a PreToolUse hook, before the tool executes. The agent never gets the result. ' +
          'A report tells you what happened; this decides whether it happens.',
      },
      {
        title: 'Anchored on the target, not the tool',
        body:
          'A tool you allow can still be stopped from touching a backup. Delete capability hides behind ' +
          'delete_file, rm, drop_table, or a plain shell string. What gets deleted does not become ' +
          'recoverable because the tool was renamed.',
      },
      {
        title: 'Every refusal is checkable',
        body:
          'Each decision records the rule that fired, a hash of the offending value, and the fingerprint ' +
          'of the policy it was judged against. Refusals enter the same hash-chained event stream as ' +
          'everything else, so "it blocked a deletion on this date" is verifiable, not a claim.',
      },
      {
        title: 'It can only tighten',
        body:
          'The guard never emits allow, never returns a looser verdict than the policy, and never blocks ' +
          'via an exit code. Silence means no opinion, and the agent\'s own permission flow still runs.',
      },
    ],
    rulesTitle: 'Four rules, in this order',
    rulesLead:
      'The first two do not consult the tool\'s own verdict. A tool the policy allows is still stopped ' +
      'from writing to a path it must never touch. Every refusal names the rule that fired, so a disputed ' +
      'block can be argued with.',
    rulesHead: { n: '#', name: 'Rule', when: 'Fires when', result: 'Result' },
    rules: [
      {
        n: '1',
        name: 'guard.sensitive-target',
        when: 'An argument names a credential: .env, a private key, cloud credentials, kubeconfig.',
        then: 'Refuse',
        why: 'Reading a key and deleting a key cost the same, and a read leaves no trace.',
      },
      {
        n: '2',
        name: 'guard.protected-target',
        when: 'An argument names something irreplaceable, and this call writes or deletes.',
        then: 'Refuse',
        why: 'Backups, dumps, data directories, production paths, .git. There is no second copy.',
      },
      {
        n: '3',
        name: 'policy.*',
        when: 'The compiled policy has an opinion about this tool.',
        then: 'deny, ask, or allow',
        why: 'The same three states the audit track produced, with the reason it was made.',
      },
      {
        n: '4',
        name: 'guard.destructive',
        when: 'The call is destructive and its target is not on the safe list.',
        then: 'At least ask',
        why: 'A destructive verb in an argument counts, even when the tool name is neutral.',
      },
    ],
    targets: {
      title: 'Two classes, because they should be refused at different moments',
      body:
        'Merging them has a concrete cost. A schema.sql you read every day sits in the same list as a ' +
        'backup volume you must never touch. Refuse reads too, and the agent cannot open a migration ' +
        'file. The customer turns the hook off that day, and protection drops to zero.',
      classes: [
        {
          name: 'Sensitive',
          contents: '.env, private keys, cloud credentials, kubeconfig',
          when: 'Refused on any operation, reads included',
        },
        {
          name: 'Protected',
          contents: 'Backups, dumps, *.sql, data directories, production paths, .git',
          when: 'Refused only when the call writes or deletes',
        },
      ],
    },
    agents: {
      title: 'Twelve agents, and the shapes they demand',
      body:
        'Each row below was verified against that vendor\'s own documentation. A hook that installs but ' +
        'never fires is worse than one that is honestly absent, so anything without a blocking hook is ' +
        'listed as unsupported rather than approximated.',
      rows: [
        { agent: 'Claude Code', event: 'PreToolUse', config: '~/.claude/settings.json' },
        { agent: 'Codex CLI', event: 'PreToolUse', config: '~/.codex/hooks.json' },
        { agent: 'CodeBuddy', event: 'PreToolUse', config: '~/.codebuddy/settings.json' },
        { agent: 'Qwen Code', event: 'PreToolUse', config: '~/.qwen/settings.json' },
        { agent: 'Qoder', event: 'PreToolUse', config: '~/.qoder/settings.json' },
        { agent: 'Gemini CLI', event: 'BeforeTool', config: '~/.gemini/settings.json' },
        { agent: 'Antigravity CLI', event: 'PreToolUse', config: '~/.gemini/antigravity-cli/hooks.json' },
        { agent: 'Cursor', event: 'preToolUse', config: '~/.cursor/hooks.json' },
        { agent: 'Crush', event: 'PreToolUse', config: '~/.config/crush/crush.json' },
        { agent: 'Factory Droid', event: 'PreToolUse', config: '~/.factory/hooks.json' },
        { agent: 'Cline', event: 'PreToolUse', config: '.clinerules/hooks/PreToolUse (executable)' },
        { agent: 'OpenCode', event: 'tool.execute.before', config: '.opencode/plugin/ratchet-guard.js (plugin)' },
      ],
      note:
        'Five config shapes and seven output protocols sit behind that table. Sending Claude\'s shape to ' +
        'Antigravity produces no error at all; it just does not block. Not covered: Kimi CLI (no hook ' +
        'system yet), Aider (git hooks only), Kilo Code (prompt-level rules). Those can be observed ' +
        'but not stopped.',
    },
    risks: {
      title: 'Where this stops working',
      lead:
        'Read this before installing it on a machine you care about. Each item below is a real boundary ' +
        'of the current implementation, not a hedge.',
      items: [
        {
          title: 'Only calls that go through the agent',
          body:
            'A person typing rm, a cron job, a build script, or a different agent is outside the hook. ' +
            'The guard sees the tool calls its agent routes to it, and nothing else.',
        },
        {
          title: 'Argument analysis is heuristic',
          body:
            'Encoding a delete in base64, or writing a script that deletes and then invoking it, evades ' +
            'the scan for destructive verbs. The target rules catch the direct forms. A crafted bypass ' +
            'is a different problem class.',
        },
        {
          title: 'It is not a sandbox',
          body:
            'It decides about calls it can see. It does not contain processes, restrict the filesystem, ' +
            'or limit blast radius once something runs.',
        },
        {
          title: 'Some agents cannot ask',
          body:
            'Codex, Gemini CLI, Antigravity, Cursor, Crush, Cline and OpenCode have no reliable ' +
            '"a human should look at this" state. On those, a request for confirmation falls back to ' +
            'the agent\'s own permission flow, which may auto-approve. Refusals still hold.',
        },
        {
          title: 'A payload mismatch fails open, loudly',
          body:
            'If an agent upgrade changes its payload shape, the guard allows the call and writes a ' +
            'warning to stderr. Blocking on unrecognized input would freeze the agent on unrelated ' +
            'work. The cost is that a misconfigured hook protects nothing, and you find out from the ' +
            'warning rather than from an incident.',
        },
        {
          title: 'Hooks can be disabled',
          body:
            'They are configuration: a user can uninstall them, turn them off, or (on Codex and ' +
            'CodeBuddy) decline to trust them. An unmanaged hook is a guardrail, not an enforcement ' +
            'boundary.',
        },
        {
          title: 'Two integrations are less proven',
          body:
            'Cursor\'s hooks have community reports of not firing, and its preToolUse does not enforce ' +
            '"ask". The OpenCode integration is a plugin we wrote: correct against the documented API, ' +
            'not yet field-tested. Cline stays inert until Enable Hooks is checked in its settings.',
        },
        {
          title: 'Defaults are deliberately noisy',
          body:
            'The safe-target list ships empty, so clearing node_modules raises a confirmation instead ' +
            'of passing silently. That is a deliberate default, not a bug, and turning it on is a ' +
            'decision the operator should make after seeing what gets asked.',
        },
      ],
    },
    advice: {
      title: 'How to install it without breaking anyone\'s day',
      items: [
        {
          when: 'Before you install',
          body:
            'Dry-run with a real payload. Feed a captured PreToolUse event to ratchet guard check and ' +
            'read the verdict. The only question that matters at this stage is whether it blocks a ' +
            'normal working day.',
        },
        {
          when: 'First week',
          body:
            'One agent at a time, and read guard.jsonl. Every line names the rule that fired, so a ' +
            'noisy rule is visible immediately instead of costing someone an afternoon.',
        },
        {
          when: 'Then',
          body:
            'Turn on the safe-target list once you have seen what gets asked. That is the only switch ' +
            'in this system that lowers protection, and it should be an operator decision.',
        },
        {
          when: 'Unattended machines',
          body:
            'Use --fail-closed on CI and servers, where nobody is watching and a locked-out agent is ' +
            'cheaper than a silent one. Keep the default on laptops, where the trade is reversed.',
        },
        {
          when: 'Every upgrade',
          body:
            'Re-run the dry run after updating an agent. Payload shapes move between versions, and the ' +
            'guard will warn rather than block when one changes.',
        },
        {
          when: 'Always',
          body:
            'Keep the policy in version control. The policy fingerprint in each record points back to ' +
            'an exact file, which is how you answer "what were the rules at the time" months later.',
        },
      ],
    },
    installTitle: 'Try it against one call before you install it',
    installLead:
      'Feed it a real payload and read the verdict. If it blocks something you need, you have found ' +
      'that out without changing anyone\'s machine.',
    install: {
      dry: 'Dry run first',
      then: 'Then install',
      listPrefix: 'See which agents are supported:',
      verify: 'Verify a delivery',
      ask: 'Ask a question',
    },
    commands: {
      check: 'ratchet guard check --policy policy.json < payload.json',
      install: 'ratchet guard install --harness codebuddy',
      write: 'ratchet guard install --harness codebuddy --write',
      all: 'ratchet guard install --list',
    },
  },
  pricing: {
    title: 'Pricing',
    lead:
      'The tool is free and open source. The paid work is a one-time audit: someone runs it against ' +
      'your machines, reviews what it found with you, and hands you a deliverable your own client or ' +
      'auditor can verify without trusting us.',
    free: {
      title: 'Self-serve',
      price: 'Free',
      body:
        'Install it, scan your machines, compile a policy, and produce deliveries yourself. Everything ' +
        'in the core is on your machine; nothing is held back behind a licence.',
      bullets: [
        'One-command install, or Homebrew',
        'Local scan, policy compiler, audit chain',
        'Verifiable deliveries a client can check',
      ],
      cta: 'Get it on GitHub',
    },
    audit: {
      title: 'Permission audit',
      per: 'one engagement · one flat fee',
      body:
        'For people who have to answer "what can your agent actually touch, and can you prove it". We ' +
        'agree the scope at the kickoff call; if your setup is far outside what the flat fee covers, ' +
        'you hear that before anything is charged.',
      bullets: [
        'We run it with you and review what it found',
        'A least-privilege policy with the reason behind each verdict',
        'A delivery your client verifies on their own machine',
        'Kickoff within 1 business day, delivery within 5',
      ],
    },
    buy: 'Buy the audit',
    opening: 'Opening checkout…',
    requestQuote: 'Request a quote',
    paddleNote:
      'Paid through Paddle, the seller of record. Invoices, VAT/GST and refunds are handled by them; ' +
      'we never see your card details.',
    reviewNoteTitle: 'Note for review:',
    reviewNoteBody1: 'the audit price band is set per engagement and is not published here yet. Write to',
    reviewNoteBody2: 'and you will get a written quote before anything is charged.',
  },
  verify: {
    title: 'Verify a delivery',
    lead1: 'Drop the ',
    lead2:
      ' you received into this page. The hashes are recomputed in your browser and compared against the ' +
      'manifest, so',
    leadStrong: 'the conclusion comes from your machine, and you never have to trust the sender',
    leadDot: '.',
    dropTitle: 'Drop share.json here, or click to pick a file',
    dropNote: 'The file is read in this page only. It is never uploaded anywhere.',
    status: { ok: 'matches', modified: 'changed', missing: 'missing', not_in_bundle: 'not in bundle' },
    passed: 'All hashes match',
    failed: 'Mismatch found',
    checked: 'Checked',
    modifiedLabel: 'Changed',
    missingLabel: 'Missing',
    readFile: 'Read',
    okNote: (n: number) => `${n} artifacts match the manifest exactly. This delivery has not been altered since it was produced.`,
    badNote: (modified: number, missing: number, checked: number) =>
      `${modified} changed, ${missing} missing (of ${checked} checked).`,
    absentNote: (n: number) => `${n} more are not in the bundle, so they cannot be verified here:`,
    table: { file: 'File', verdict: 'Verdict', expected: 'Expected sha256', actual: 'Actual sha256' },
    dash: '—',
  },
  thanks: {
    title: 'Payment received',
    lead:
      'Paddle has emailed you the receipt and the invoice. Paddle is the seller on record for this ' +
      'transaction, so the charge on your statement comes from them, not from us.',
    next: 'What happens next',
    step1When: '1. Within one business day',
    step1Body: (email: string) =>
      `you get an email from ${email} proposing two or three slots for a 30-minute kickoff.`,
    clientTitle: 'How your client checks it',
    clientBody:
      'They open the verify page and drop the bundle in. Every hash is recomputed in their own browser; ' +
      'nothing in the verdict depends on trusting us.',
    clientCta: 'Open the verify page',
    refundNote1: 'Refunds are handled by Paddle, see the',
    refundNoteLink: 'refund policy',
    refundNote2: 'If something went wrong with the payment, write to',
    refundNote3: 'with your Paddle order id.',
  },
}

/**
 * 中文那棵树用 `typeof en` 约束：漏翻一个字段、多写一个字段、或者函数签名对不上，
 * 都是编译错误。这比"上线后发现某一块悄悄退回英文"便宜得多。
 */
const zh: typeof en = {
  switchLabel: 'English',
  nav: {
    how: '怎么用',
    blocking: '拦截',
    limits: '能力边界',
    verify: '验证',
    rules: '判定规则',
    agents: '支持的 agent',
    home: '首页',
    pricing: '定价',
    contact: '联系',
    terms: '条款',
  },
  footer: {
    license: 'Apache-2.0',
    year: '© 2026',
    note: '开发者预览版，输出格式还会变',
  },
  home: {
    tagline: {
      title: '权限是编译出来的，不是手写的。',
      lead:
        'Ratchet 读的是你的 AI agent 实际能够到什么，把它编译成一份最小权限策略，' +
        '并交出收货方在自己机器上就能验的证据。',
    },
    cta: {
      request: '申请访问',
      quickStart: '快速开始',
      verify: '验证一份交付物',
      getStarted: '开始用',
      desktop: '下载一键安装包',
      deliverForYou: '想让人代做',
    },
    install: {
      install: '安装',
      scan: '然后扫这台机器',
      build: '从源码构建',
      dist: 'make dist   # 四个平台，各自带 sha256',
      scanLocal: './bin/ratchet scan --home ~',
      download: '下载',
      onDesktop: '用 Claude Desktop？',
      desktopSuffix: '同一个 server，不碰终端就能装好。',
      sourceNote: '源码与 issue：github.com/suhui-organization/ratchet，这些发布产物就是从该仓库构建的。',
      getStartedLead: '一条命令。它会下载你所在平台的二进制，并在安装前校验 sha256。',
      noBuildLead: '还没有公开构建，先给少数人用。源码也尚未公开；来问我要一份带 sha256 的构建。',
    },
    features: [
      {
        title: '读的是真实存在的东西',
        body:
          '各家 agent 把 MCP server 存在十几种不同格式的配置里。Ratchet 把它们解析出来，' +
          '不执行任何东西，也从不记录那些存放令牌的环境变量的值。',
      },
      {
        title: '编译出策略',
        body:
          '能力从工具名与描述推断，再和真实发生过的调用交叉。读取、写入、执行、破坏性各归各档，' +
          '而且每条判定都带着它为什么这么判。',
      },
      {
        title: '证明交出去的是什么',
        body:
          '一份交付物附带 sha256 清单。收到的人在**自己机器上**重算每一个哈希，' +
          '所以结论从不依赖于"相信发送方"。',
      },
    ],
    limits: [
      '不是沙箱，它不隔离任何东西。',
      '审计那条线不拦任何调用。真正拒绝一次调用的是执行点，那是另一个组件，有它自己的边界。',
      '静态扫描遇到读不懂的配置会如实报告，不猜。',
      '只记录工具名，从不记录参数。',
    ],
    limitsLink: '执行点拒绝什么，以及它在哪儿会失效',
    how: {
      title: '是编译器，不是又一个扫描器。',
      lead: '扫描器告诉你 agent 能够到什么。Ratchet 告诉它应该被允许做什么，依据是能力面与真实发生过的调用。',
    },
    evidence: {
      lead1: '在一台真实的开发者机器上：',
      strong: '12 / 16',
      lead2: '个 MCP server 由包管理器拉起，而它们**一个都没锁版本**。其中三个写的是',
      code: '@latest',
      lead3: '——看起来像锁了版本，实际解析出来什么都不是。',
    },
    design: {
      title: '设计主张：最小权限来自证据。',
      items: [
        {
          title: '为了摸清能力面，不执行任何东西',
          body: '发现过程只读配置文件。连上 server 去列它的工具是另一步，而且必须显式要求——因为那一步会启动进程。',
        },
        {
          title: '检查的人是收货方自己',
          body:
            '哈希在读的人自己机器上重算，用浏览器，或者一个只用标准库的脚本。' +
            '结论的任何一部分都不依赖于相信发送方。',
        },
      ],
    },
    limitsTitle: '它不做什么',
    after: {
      title: '装完十秒，你会得到一个数字',
      lead: '不是仪表盘，也不是关于你所在行业的报告。是这台机器上 MCP server 的数量，以及其中有几个锁了版本。',
    },
    steps: [
      {
        title: '看见能力面',
        body: '这台机器知道的所有 MCP server、其中哪些没锁版本、以及哪些配置文件它读不懂。',
      },
      {
        title: '编译策略',
        body: '把能力面和真实发生过的调用交叉，得到分开的读取 / 写入 / 执行 / 破坏性，每条带依据。',
      },
      {
        title: '交出一份能被检查的东西',
        body: '一个目录，装策略、报告与 sha256 清单。收货方在自己机器上、或在浏览器里重算哈希。',
      },
    ],
  },
  guard: {
    title: '它在动作发生之前就把它停下。',
    lead:
      '审计那条线告诉你 agent 被允许到什么、并交出自证。这一条是把那次要删掉备份的调用，' +
      '挡在它还只是一个计划的时候。',
    ctaReadLimits: '先看能力边界',
    ctaInstall: '装上它',
    points: [
      {
        title: '是拒绝，不是事后报告',
        body:
          '它作为 PreToolUse 钩子运行，在工具执行之前。agent 拿不到执行结果。' +
          '报告告诉你发生过什么；这个决定它会不会发生。',
      },
      {
        title: '锚点在目标上，不在工具名上',
        body:
          '一个你放行的工具，照样不能去碰备份。删除能力藏在 delete_file 后面，也藏在 rm、' +
          'drop_table，或者一整句 shell 字符串里。被删掉的东西不会因为工具改了名就变得能恢复。',
      },
      {
        title: '每一次拒绝都可核验',
        body:
          '每条判定都记下命中的规则、那个值的哈希、以及按哪一版策略判的。' +
          '被拒的调用进入同一条哈希链事件流，于是"它在某天挡下了一次删除"是可验证的，不是一句声明。',
      },
      {
        title: '它只会更严，不会更松',
        body:
          '执行点从不输出 allow，从不给出比策略更松的判定，也从不靠退出码阻断。' +
          '沉默意味着没有意见，agent 自己的权限流程照常走。',
      },
    ],
    rulesTitle: '四条规则，按这个顺序',
    rulesLead:
      '前两条不看工具本身的三态。一个策略放行的工具，照样会被拦住、不许写它永远不该碰的路径。' +
      '每次拒绝都点名是哪条规则拦下的，所以有争议的拦截可以被反驳。',
    rulesHead: { n: '#', name: '规则', when: '何时触发', result: '结果' },
    rules: [
      {
        n: '1',
        name: 'guard.sensitive-target',
        when: '参数里出现了凭据：.env、私钥、云凭证、kubeconfig。',
        then: '拒绝',
        why: '读走一把密钥和删掉一把密钥代价一样，而且读走不留痕迹。',
      },
      {
        n: '2',
        name: 'guard.protected-target',
        when: '参数里出现了不可恢复的东西，且这次调用会写或删。',
        then: '拒绝',
        why: '备份、转储、数据目录、生产路径、.git。它们没有第二份。',
      },
      {
        n: '3',
        name: 'policy.*',
        when: '编译出的策略对这个工具有意见。',
        then: 'deny、ask 或 allow',
        why: '与审计那条线产出的同一套三态，带着它为什么这么判。',
      },
      {
        n: '4',
        name: 'guard.destructive',
        when: '这次调用是破坏性的，且目标不在安全清单里。',
        then: '至少要求人工确认',
        why: '破坏性动词出现在参数里就算数，哪怕工具名完全中性。',
      },
    ],
    targets: {
      title: '分成两类，因为它们该在不同的时机被拒',
      body:
        '合成一张表有很实际的代价。你每天都要读的 schema.sql，会和绝不能碰的备份卷坐在同一个清单里。' +
        '连读也拒，agent 连迁移脚本都打不开。客户当天就把钩子关掉，保护强度归零。',
      classes: [
        { name: '凭据类', contents: '.env、私钥、云凭证、kubeconfig', when: '任何操作都拒，包括读' },
        {
          name: '不可恢复类',
          contents: '备份、转储、*.sql、数据目录、生产路径、.git',
          when: '只在调用会写或删时拒',
        },
      ],
    },
    agents: {
      title: '十二家 agent，以及它们各自的写法',
      body:
        '下表每一行都对着该厂商自己的文档核实过。一个装了却从不触发的钩子，比一句诚实的不支持更糟，' +
        '所以没有可阻断钩子的一律列为不支持，不做近似实现。',
      rows: [
        { agent: 'Claude Code', event: 'PreToolUse', config: '~/.claude/settings.json' },
        { agent: 'Codex CLI', event: 'PreToolUse', config: '~/.codex/hooks.json' },
        { agent: 'CodeBuddy（腾讯云）', event: 'PreToolUse', config: '~/.codebuddy/settings.json' },
        { agent: 'Qwen Code（阿里）', event: 'PreToolUse', config: '~/.qwen/settings.json' },
        { agent: 'Qoder / 通义灵码（阿里）', event: 'PreToolUse', config: '~/.qoder/settings.json' },
        { agent: 'Gemini CLI（Google）', event: 'BeforeTool', config: '~/.gemini/settings.json' },
        {
          agent: 'Antigravity CLI（Google）',
          event: 'PreToolUse',
          config: '~/.gemini/antigravity-cli/hooks.json',
        },
        { agent: 'Cursor', event: 'preToolUse', config: '~/.cursor/hooks.json' },
        { agent: 'Crush（Charm）', event: 'PreToolUse', config: '~/.config/crush/crush.json' },
        { agent: 'Factory Droid', event: 'PreToolUse', config: '~/.factory/hooks.json' },
        { agent: 'Cline', event: 'PreToolUse', config: '.clinerules/hooks/PreToolUse（可执行文件）' },
        {
          agent: 'OpenCode',
          event: 'tool.execute.before',
          config: '.opencode/plugin/ratchet-guard.js（插件）',
        },
      ],
      note:
        '那张表背后是五种配置形状、七套输出协议。把 Claude 的形状发给 Antigravity，一点错都不会报，' +
        '它只是不拦。不在覆盖范围内：Kimi CLI（还没有钩子系统）、Aider（只有 git 钩子）、' +
        'Kilo Code（提示词级规则）。这三家能观测，拦不住。',
    },
    risks: {
      title: '它在哪儿会失效',
      lead: '在把它装到一台你在乎的机器上之前，先读这一段。下面每一条都是当前实现的真实边界，不是免责套话。',
      items: [
        {
          title: '只覆盖走 agent 的调用',
          body:
            '人自己敲的 rm、cron 任务、构建脚本、另一个 agent，都在钩子之外。' +
            '执行点只能看见它的 agent 路由给它的那些工具调用。',
        },
        {
          title: '参数分析是启发式的',
          body:
            '把删除动作 base64 包一层，或者写一个删除脚本再去调用它，都能绕开"在参数里找破坏性动词"' +
            '这一层。目标规则拦的是直白形态；精心构造的规避是另一类问题。',
        },
        {
          title: '它不是沙箱',
          body: '它只对它看得见的调用做判定。它不隔离进程、不限制文件系统、也不控制东西跑起来之后的爆炸半径。',
        },
        {
          title: '有七家 agent 无法"要人看一眼"',
          body:
            'Codex、Gemini CLI、Antigravity、Cursor、Crush、Cline、OpenCode 没有可靠的' +
            '"这件事该让人看看"这一档。在这些上面，需要人工确认的调用会退回它们自己的权限流程，' +
            '而那个流程可能自动放行。拒绝仍然有效。',
        },
        {
          title: '载荷对不上时它放行，但会大声说',
          body:
            '如果 agent 升级改了载荷形状，执行点会放行这次调用，并往 stderr 写一条告警。' +
            '认不出输入就阻断，会把 agent 卡死在无关的事情上。代价是：配错的钩子什么都没保护，' +
            '而你是从告警里发现这一点，不是从一次事故里。',
        },
        {
          title: '钩子可以被关掉',
          body:
            '它们就是配置：用户可以卸载、关闭，或者在 Codex 与 CodeBuddy 上拒绝信任。' +
            '一个未受管制的钩子是护栏，不是强制边界。',
        },
        {
          title: '两家的证据较弱',
          body:
            'Cursor 的钩子在社区里有不触发的报告，而且它的 preToolUse 不强制 ask。' +
            'OpenCode 那条是我们自己写的插件：对着文档 API 是对的，但还没在真实环境跑过。' +
            'Cline 在设置里勾上 Enable Hooks 之前不会生效。',
        },
        {
          title: '默认值是故意吵的',
          body:
            '安全目标清单默认是空的，所以清 node_modules 会弹一次确认，而不是静默放行。' +
            '这是刻意选的默认值，不是缺陷；把它打开应该是在看过到底会问些什么之后由运维决定。',
        },
      ],
    },
    advice: {
      title: '怎么装它而不毁掉谁的一天',
      items: [
        {
          when: '装之前',
          body:
            '拿真实载荷先干跑。把一条抓到的 PreToolUse 事件喂给 ratchet guard check，读判定结果。' +
            '这个阶段唯一要紧的问题是：它会不会挡掉正常干活。',
        },
        {
          when: '第一周',
          body:
            '一次只上一家 agent，并且读 guard.jsonl。每一行都点名是哪条规则触发的，' +
            '所以哪条规则太吵会立刻看见，而不是让某个人损失一个下午。',
        },
        {
          when: '之后',
          body:
            '看过到底会问些什么之后，再打开安全目标清单。那是这套东西里唯一会降低保护强度的开关，该由运维决定。',
        },
        {
          when: '无人值守的机器',
          body:
            '在 CI 和服务器上加 --fail-closed：那里没人在看屏幕，agent 被锁住比它静默地跑要便宜。' +
            '笔记本上保持默认，那里的取舍是反的。',
        },
        {
          when: '每次升级',
          body: '更新 agent 之后重跑一遍干跑。载荷形状会随版本变，而执行点在形状变化时会告警、不会拦。',
        },
        {
          when: '长期',
          body:
            '把策略纳入版本控制。每条记录里的策略指纹都指回一个确切的文件，' +
            '这就是几个月后你回答当时规则是什么的方式。',
        },
      ],
    },
    installTitle: '先拿一次真实调用试它，再决定装不装',
    installLead: '喂一份真实载荷，读判定结果。如果它挡掉了你需要的东西，你是在没动任何人机器的情况下发现的。',
    install: {
      dry: '先干跑',
      then: '再安装',
      listPrefix: '看支持哪些 agent：',
      verify: '验证一份交付物',
      ask: '问个问题',
    },
    commands: {
      check: 'ratchet guard check --policy policy.json < payload.json',
      install: 'ratchet guard install --harness codebuddy',
      write: 'ratchet guard install --harness codebuddy --write',
      all: 'ratchet guard install --list',
    },
  },
  pricing: {
    title: '定价',
    lead:
      '工具本身免费、开源。收费的是一次性审计：有人拿它跑你的机器、和你一起看它发现了什么，' +
      '并交出一份你自己的客户或审计员不用信任我们就能验的交付物。',
    free: {
      title: '自助',
      price: '免费',
      body: '自己装、自己扫、自己编译策略、自己产出交付物。核心能力全在你机器上，没有任何东西被许可证挡住。',
      bullets: ['一条命令安装，或用 Homebrew', '本机扫描、策略编译器、审计链', '客户可以自己验的交付物'],
      cta: '去 GitHub 拿',
    },
    audit: {
      title: '权限审计',
      per: '一次服务 · 一口价',
      body:
        '给那些必须回答"你的 agent 到底能碰什么，你能证明吗"的人。范围在启动会上谈定；' +
        '如果你的环境远超一口价覆盖的范围，会在收费之前告诉你。',
      bullets: [
        '我们和你一起跑，并一起看它发现了什么',
        '一份最小权限策略，每条判定都带依据',
        '一份你的客户在自己机器上验证的交付物',
        '1 个工作日内启动，5 个工作日内交付',
      ],
    },
    buy: '购买这次审计',
    opening: '正在打开收银台…',
    requestQuote: '询价',
    paddleNote:
      '通过 Paddle 收款，它是这笔交易的卖方。发票、增值税与退款都由它处理；我们看不到你的卡号。',
    reviewNoteTitle: '给审核方的说明：',
    reviewNoteBody1: '审计价格按次确定，暂不在此公布。写信到',
    reviewNoteBody2: '，你会先拿到一份书面报价，然后才会产生任何费用。',
  },
  verify: {
    title: '独立验证',
    lead1: '把收到的 ',
    lead2: ' 拖进来。哈希在你的浏览器里重算并与清单比对，所以',
    leadStrong: '结论由你的设备得出，不需要信任出具方',
    leadDot: '。',
    dropTitle: '拖入 share.json，或点这里选择文件',
    dropNote: '文件只在本页读取，不会上传到任何服务器',
    status: { ok: '一致', modified: '被改动', missing: '缺失', not_in_bundle: '不在包内' },
    passed: '全部哈希一致',
    failed: '发现不一致',
    checked: '已检查',
    modifiedLabel: '被改动',
    missingLabel: '缺失',
    readFile: '已读取',
    okNote: (n: number) => `已校验 ${n} 份产物，与清单完全一致——这份交付物自生成以来未被改动。`,
    badNote: (modified: number, missing: number, checked: number) =>
      `${modified} 份被改动、${missing} 份缺失（共校验 ${checked} 份）。`,
    absentNote: (n: number) => `另有 ${n} 份不在包内，因此无法在这里验证：`,
    table: { file: '文件', verdict: '结论', expected: '期望 sha256', actual: '实际 sha256' },
    dash: '—',
  },
  thanks: {
    title: '已收到付款',
    lead:
      'Paddle 已经把收据和发票发到你的邮箱。这笔交易的卖方是 Paddle，' +
      '所以账单上出现的是它们的名字，不是我们的。',
    next: '接下来会发生什么',
    step1When: '1. 一个工作日内',
    step1Body: (email: string) => `你会收到来自 ${email} 的邮件，里面给出两三个 30 分钟启动会的时间选项。`,
    clientTitle: '你的客户怎么验证它',
    clientBody:
      '他打开验证页，把交付包拖进去。每一个哈希都在他自己的浏览器里重算；结论里没有任何一部分依赖于相信我们。',
    clientCta: '打开验证页',
    refundNote1: '退款由 Paddle 处理，见',
    refundNoteLink: '退款政策',
    refundNote2: '如果付款过程出了问题，写信到',
    refundNote3: '，附上你的 Paddle 订单号。',
  },
}

export const copy: Record<Locale, typeof en> = { en, zh }

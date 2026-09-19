# Content pack: "How to answer 'what can your agent touch?' in five minutes"

> 面向 LinkedIn / 乙方与顾问圈（中文与英文各一版）。
> 纪律：这是一份**做法**，不是战果。不虚构客户、不写"某客户如何如何"。
> canonical 指向 `https://podcloud.dlszjr.com`。

---

## LinkedIn（英文）

Every AI project eventually hits the same question, and it never comes from engineering:

*"What can your agent actually touch?"*

Usually asked by a client, an auditor, or whoever signs the risk acceptance. And the honest answer is often "let me get back to you", because the answer lives in a dozen config files and a tool list nobody wrote down.

Here's the five-minute version, and it's the same five minutes whether you're answering for yourself or for a client.

**1. Enumerate, don't describe.** Read the harness configs, list the servers, count the ones that are unpinned. That number is the first thing worth saying out loud — it's concrete, it's yours, and it's checkable.

**2. Ask the servers what they expose.** Connecting and listing tools is a separate, explicit step, because it starts processes. Do it deliberately, and write down what answered as well as what didn't. "Four servers didn't answer" is part of the answer.

**3. Compile a policy, with reasons.** Three states, not two: allow, approve (a human decides at call time), deny. Every verdict needs the reason attached, or nobody can correct it — including you.

**4. Hand over something they can verify without trusting you.** A folder with the policy, the report, and a sha256 manifest. They recompute the hashes in their own browser. That last step is what turns "trust me" into "check me".

The part people skip is 4, because it feels like extra work. It isn't: it's the only part that survives the question being asked a second time, by someone who doesn't know you.

What this doesn't do: it isn't a sandbox, it doesn't block anything at runtime, and a name/description heuristic doesn't know what a tool actually does. It gives you a defensible position, not a guarantee.

I write down the whole procedure, including the artifacts, here: https://podcloud.dlszjr.com

---

## LinkedIn（中文）

做 AI 落地到最后总会撞上同一个问题，而且提问题的人通常不是工程师：

*"你这个 agent 到底能碰什么？"*

问的人一般是甲方、审计、或者签字接受风险的那个人。诚实的回答往往是"我回去看看"——因为答案散在十几个配置文件里，还有一份没人写下来过的工具清单。

五分钟版是这样的，给自己答和替客户答，走的都是这五步。

**一、先把清单数出来，别先解释。** 读 harness 的配置，列出 server，数出其中没锁版本的个数。这个数字是第一个值得说出口的东西：具体、属于你、而且能被别人复核。

**二、问 server 自己能做什么。** 连上去列工具是单独的一步，因为它会真的启动进程，所以要显式做。记下答话的，也记下没答话的——"有四个 server 没答话"本身就是答案的一部分。

**三、编译成策略，而且每条都带依据。** 三态而不是两态：允许、待批准（调用时由人决定）、拒绝。每条判定都必须附上理由，否则没人能纠正它，包括你自己。

**四、交出去一份不需要信任你的东西。** 一个目录：策略、报告、sha256 清单。对方在自己浏览器里重算一遍哈希。这一步才是把"信我"变成"验我"的地方。

大多数人跳过第四步，因为它看起来是额外工作量。恰恰相反：只有它能让同一个问题在第二次、由不认识你的人问起时还站得住。

它做不到什么也说清楚：它不是沙箱、运行时拦不住任何调用、靠名称与描述的启发式判断也不知道工具真正干了什么。它给的是一条能自证的立场，不是一句保证。

完整流程与产物我写在 https://podcloud.dlszjr.com

---

## dev.to / 博客（长文骨架）

标题：**Answering "what can your agent touch?" without hand-waving**

1. 场景：这句话什么时候被问出来，为什么工程侧的回答会失效（§ 上文的开头两段）
2. 为什么"列清单"这件事本身有价值：未锁版本的 server 数（引用 02 篇的真实数字）
3. 枚举工具的那一步（引用 03 篇的 164 个工具与 15 条 deny）
4. 三态策略与"每条判定带依据"——顺带给出两个误判例子，说明为什么要留依据
5. 交付物与验证：sha256 清单 + 浏览器里重算（链接指向 `/verify`）
6. 边界：不是沙箱、不拦调用、启发式会错
7. 结尾不要 call to action，只留一句"这五步任何人都能自己走一遍"

## 发布纪律（贴之前再读一遍）

1. 全篇**不出现客户**。没有真实客户就不写客户；写了就得能拿出证据。
2. 结尾不放"立即购买"。放流程与站点链接就够，转化发生在对方照着做完之后。
3. 中文版不要直译英文版：中文圈会把"审计"读成"合规认证"，而这里明确不是认证。
4. 有人问价：直接给定价页，不要在现场谈折扣。

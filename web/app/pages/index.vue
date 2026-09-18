<template>
  <main class="mx-auto max-w-3xl px-6 py-16">
    <h1 class="text-3xl font-semibold tracking-tight">Ratchet</h1>
    <p class="mt-3 text-lg text-slate-600">
      从 agent 的真实行为编译最小权限策略，并给出一份<strong class="font-semibold text-slate-800">收货方自己能验证</strong>的证据。
    </p>

    <section class="mt-10 rounded-lg border border-slate-200 bg-white p-6">
      <h2 class="text-sm font-medium uppercase tracking-wide text-slate-500">三步</h2>
      <ol class="mt-4 space-y-4 text-sm">
        <li>
          <p class="font-medium">1. 编译策略</p>
          <pre class="mt-1 overflow-x-auto rounded bg-slate-900 p-3 text-slate-100"><code>ratchet policy draft --from inventory.json --out policy.json</code></pre>
          <p class="mt-1 text-slate-500">读、写、执行、破坏性各自落到 allow / approve / deny，每条判定都带依据。</p>
        </li>
        <li>
          <p class="font-medium">2. 出交付物</p>
          <pre class="mt-1 overflow-x-auto rounded bg-slate-900 p-3 text-slate-100"><code>ratchet-report build --dir ./delivery
ratchet-report bundle --dir ./delivery --out share.json</code></pre>
          <p class="mt-1 text-slate-500">报告 + 策略 + 逐文件 sha256 清单，打成<strong>一个 JSON</strong>。</p>
        </li>
        <li>
          <p class="font-medium">3. 收货方验证</p>
          <p class="mt-1 text-slate-500">
            把 <code class="rounded bg-slate-100 px-1">share.json</code> 拖进
            <NuxtLink to="/verify" class="font-medium text-slate-900 underline">验证页</NuxtLink>，
            哈希在<strong>他的浏览器里</strong>重算。
          </p>
        </li>
      </ol>
    </section>

    <section class="mt-8 rounded-lg border border-slate-200 bg-white p-6">
      <h2 class="text-sm font-medium uppercase tracking-wide text-slate-500">它不做什么</h2>
      <ul class="mt-3 space-y-1 text-sm text-slate-600">
        <li>不是沙箱、不做容器隔离；</li>
        <li>不是 prompt 内容过滤器；</li>
        <li>不要求你把日志传到任何地方——验证不需要服务端。</li>
      </ul>
    </section>
  </main>
</template>

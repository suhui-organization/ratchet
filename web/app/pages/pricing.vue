<script setup lang="ts">
import { site } from '~/site'

// 收银台：Paddle overlay。配置缺失时按钮自动退回"写邮件询价"，不会出现点了没反应的按钮。
const { configured, busy, error, buy } = usePaddleCheckout()
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-5xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink to="/" class="font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
        <div class="ml-auto flex gap-5 text-muted">
          <NuxtLink to="/" class="hover:text-brand">Home</NuxtLink>
          <a :href="site.contactUrl" class="hover:text-brand">Contact</a>
        </div>
      </nav>
    </header>

    <main class="mx-auto max-w-5xl px-6 py-16">
      <h1 class="text-[2.2rem] font-semibold tracking-[-0.03em] text-ink">Pricing</h1>
      <p class="mt-4 max-w-[60ch] text-[15px] leading-relaxed text-muted">
        The tool is free and open source. The paid work is a one-time audit: someone runs it
        against your machines, reviews what it found with you, and hands you a deliverable your
        own client or auditor can verify without trusting us.
      </p>

      <div class="mt-10 grid gap-6 md:grid-cols-2">
        <section class="rounded-xl border border-line p-6">
          <h2 class="text-[17px] font-semibold text-ink">Self-serve</h2>
          <p class="mt-3 text-[2rem] font-semibold tracking-tight text-ink">Free</p>
          <p class="mt-3 text-[14px] leading-relaxed text-muted">
            Install it, scan your machines, compile a policy, and produce deliveries yourself.
            Everything in the core is on your machine; nothing is held back behind a licence.
          </p>
          <ul class="mt-5 space-y-2 text-[14px] text-muted">
            <li>· One-command install, or Homebrew</li>
            <li>· Local scan, policy compiler, audit chain</li>
            <li>· Verifiable deliveries a client can check</li>
          </ul>
          <a href="https://github.com/suhui-organization/ratchet"
             class="mt-6 inline-block rounded-lg border border-line px-4 py-2 text-[14px] text-ink hover:border-brand">
            Get it on GitHub
          </a>
        </section>

        <section class="rounded-xl border border-brand/40 bg-surface-2 p-6">
          <h2 class="text-[17px] font-semibold text-ink">Permission audit</h2>
          <p class="mt-3 text-[2rem] font-semibold tracking-tight text-ink">{{ site.auditPrice }}</p>
          <p class="mt-1 text-[13px] text-faint">one engagement · one flat fee</p>
          <p class="mt-3 text-[14px] leading-relaxed text-muted">
            For people who have to answer "what can your agent actually touch, and can you prove
            it". We agree the scope at the kickoff call; if your setup is far outside what the flat
            fee covers, you hear that before anything is charged.
          </p>
          <ul class="mt-5 space-y-2 text-[14px] text-muted">
            <li>· We run it with you and review what it found</li>
            <li>· A least-privilege policy with the reason behind each verdict</li>
            <li>· A delivery your client verifies on their own machine</li>
            <li>· Kickoff within 1 business day, delivery within 5</li>
          </ul>
          <button v-if="configured" type="button" :disabled="busy"
                  class="mt-6 inline-block rounded-lg bg-brand px-4 py-2 text-[14px] font-medium text-white hover:bg-brand-ink disabled:opacity-60"
                  @click="buy">
            {{ busy ? 'Opening checkout…' : 'Buy the audit' }}
          </button>
          <a v-else :href="site.contactUrl"
             class="mt-6 inline-block rounded-lg bg-brand px-4 py-2 text-[14px] font-medium text-white hover:bg-brand-ink">
            Request a quote
          </a>
          <p v-if="error" class="mt-3 text-[13px] leading-relaxed text-muted">{{ error }}</p>
          <p class="mt-3 text-[12px] leading-relaxed text-faint">
            Paid through Paddle — the seller of record. Invoices, VAT/GST and refunds are handled
            by them; we never see your card details.
          </p>
        </section>
      </div>

      <p v-if="!site.auditPriceIsPublished" class="mt-8 rounded-lg border border-line bg-surface-2 px-5 py-4 text-[13.5px] text-muted">
        <strong class="font-medium text-ink">Note for review:</strong> the audit price band is set
        per engagement and is not published here yet. Write to
        <a :href="site.contactUrl" class="text-brand hover:underline">{{ site.contactEmail }}</a>
        and you will get a written quote before anything is charged.
      </p>

      <footer class="mt-14 flex flex-wrap gap-x-6 gap-y-2 border-t border-line pt-6 text-[13px] text-faint">
        <NuxtLink to="/legal/terms" class="hover:text-brand">Terms of Service</NuxtLink>
        <NuxtLink to="/legal/privacy" class="hover:text-brand">Privacy Policy</NuxtLink>
        <NuxtLink to="/legal/refund" class="hover:text-brand">Refund Policy</NuxtLink>
        <a :href="site.contactUrl" class="hover:text-brand">Contact</a>
      </footer>
    </main>
  </div>
</template>

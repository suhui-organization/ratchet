<script setup lang="ts">
import { site } from '~/site'
</script>

<template>
  <div class="min-h-screen bg-surface">
    <header class="border-b border-line">
      <nav class="mx-auto flex max-w-3xl items-center gap-6 px-6 py-4 text-sm">
        <NuxtLink to="/" class="font-semibold tracking-tight text-ink">{{ site.name }}</NuxtLink>
      </nav>
    </header>

    <main class="mx-auto max-w-3xl px-6 py-20">
      <h1 class="text-[2rem] font-semibold tracking-[-0.03em] text-ink">Payment received</h1>
      <p class="mt-4 max-w-[62ch] text-[15px] leading-relaxed text-muted">
        Paddle has emailed you the receipt and the invoice — Paddle is the seller on record for
        this transaction, so the charge on your statement comes from them, not from us.
      </p>

      <h2 class="mt-12 text-[17px] font-semibold text-ink">What happens next</h2>
      <ol class="mt-5 space-y-4 text-[14.5px] leading-relaxed text-muted">
        <li>
          <span class="font-medium text-ink">1. Within one business day</span> — you get an email
          from {{ site.contactEmail }} proposing two or three slots for a 30-minute kickoff.
        </li>
        <li>
          <span class="font-medium text-ink">2. Kickoff</span> — we agree on which machines and
          which agents are in range, and what the audit has to answer. Nothing is charged beyond
          the flat fee.
        </li>
        <li>
          <span class="font-medium text-ink">3. Delivery</span> — a directory containing the scan
          result, the compiled least-privilege policy with the reason behind each verdict, and a
          sha256 manifest.
        </li>
      </ol>

      <h2 class="mt-12 text-[17px] font-semibold text-ink">How your client checks it</h2>
      <p class="mt-4 max-w-[62ch] text-[15px] leading-relaxed text-muted">
        They open the verify page and drop the bundle in. Every hash is recomputed in their own
        browser; nothing in the verdict depends on trusting us.
      </p>
      <NuxtLink to="/verify"
                class="mt-5 inline-block rounded-lg bg-brand px-5 py-2.5 text-[14px] font-medium text-white hover:bg-brand-ink">
        Open the verify page
      </NuxtLink>

      <p class="mt-12 border-t border-line pt-6 text-[13px] leading-relaxed text-faint">
        Refunds are handled by Paddle, see the
        <NuxtLink to="/legal/refund" class="text-brand hover:underline">refund policy</NuxtLink>.
        If something went wrong with the payment, write to
        <a :href="site.contactUrl" class="text-brand hover:underline">{{ site.contactEmail }}</a>
        with your Paddle order id.
      </p>
    </main>
  </div>
</template>

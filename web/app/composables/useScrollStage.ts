import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

/**
 * 滚动驱动的画面序列。
 *
 * 做法（与 paddle.com 同类）：外层给足高度（`height: N × 100vh`），内层 `sticky`
 * 钉在视口里不动，滚动只用来产生一个 0→1 的进度值；进度再映射到每一层的
 * 透明度/位移/缩放。屏幕上看起来是"内容在原地换"，实际动的是滚动。
 *
 * 三个刻意的选择：
 *  1. **没有动画库**。一个进度值 + transform 就够，引 GSAP 只会让首屏多 30KB。
 *  2. **尊重 prefers-reduced-motion**：关闭时退化成三张静态堆叠图，
 *     信息一条不少，只是不动。
 *  3. **单一定时源**（rAF 节流）。滚动里做多份计算是卡顿的常见来源。
 */
export function useScrollStage(stage: Ref<HTMLElement | null>, layers: number) {
  const progress = ref(0)          // 0→1，整段滚动的归一化进度
  const active = ref(0)            // 当前层索引
  let frame = 0
  let reduced = false

  function measure() {
    const el = stage.value
    if (!el) return
    const rect = el.getBoundingClientRect()
    const total = el.offsetHeight - window.innerHeight
    if (total <= 0) {
      progress.value = 0
      return
    }
    const scrolled = -rect.top
    const p = Math.min(1, Math.max(0, scrolled / total))
    progress.value = p
    // 层索引：把 0→1 均分成 layers 段，最后一段收口在 layers-1
    active.value = Math.min(layers - 1, Math.floor(p * layers))
  }

  function onScroll() {
    if (frame) return
    frame = requestAnimationFrame(() => {
      frame = 0
      measure()
    })
  }

  /** 第 i 层的局部进度：0 = 刚开始进入，1 = 完全离开 */
  function layerProgress(i: number): number {
    const span = 1 / layers
    const start = i * span
    return Math.min(1, Math.max(0, (progress.value - start) / span))
  }

  onMounted(() => {
    reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (reduced) {
      progress.value = 0
      return
    }
    measure()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll, { passive: true })
  })

  onBeforeUnmount(() => {
    window.removeEventListener('scroll', onScroll)
    window.removeEventListener('resize', onScroll)
    if (frame) cancelAnimationFrame(frame)
  })

  return { progress, active, layerProgress, isReduced: () => reduced }
}

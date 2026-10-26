<script setup lang="ts">
withDefaults(defineProps<{ folded?: boolean; state?: 'idle' | 'thinking' | 'replying' | 'error'; happy?: boolean }>(), { state: 'idle' })
</script>

<template>
  <svg xmlns="http://www.w3.org/2000/svg" class="cat"
    :class="[folded ? 'cat-folded' : 'cat-seated', 'is-' + state, { 'is-happy': happy }]"
    :viewBox="folded ? '0 0 26 56' : '0 0 64 64'"
    fill="var(--cat-main, #F3B777)" stroke="var(--cat-ink, #765447)"
    :stroke-width="folded ? 1.5 : 2" stroke-linecap="round" stroke-linejoin="round"
    aria-hidden="true" focusable="false">
    <g v-if="folded">
  <path class="body" d="M23 18C11 16 6 26 7 37C7 47 14 51 24 48V19Z"/>
  <g class="tail">
  <path d="M17 44C5 49 2 42 4 35" fill="none" stroke="var(--cat-ink, #765447)" stroke-width="5"/>
  <path d="M17 44C5 49 2 42 4 35" fill="none" stroke-width="2.5" stroke="var(--cat-main, #F3B777)"/>
  </g>
  <path class="body" d="M24 9C18 5 9 8 8 17C7 25 15 29 24 25Z"/>
  <path class="ear" d="M9 13L8 5Q8 3 10 5L16 9Z"/>
  <path class="ear" d="M19 8L23 3Q24 2 24 5V12Z"/>
  <path d="M15 10L16 13M20 10L20 13" stroke="var(--cat-stripe, #CF8B50)"/>
  <path class="eye" d="M11 18Q13 20 15 18" fill="none"/>
  <path d="M20 20L21 21L22 20" fill="none"/>
  <path d="M19 27L24 28" stroke="var(--cat-accent, #3182BD)" stroke-width="2.5"/>
  <path class="paw" d="M19 29V34Q19 37 24 36" fill="var(--cat-belly, #FFF0DA)"/>
</g>
    <g v-else>
  <g class="tail">
    <path d="M43 54C59 58 61 44 55 41" fill="none" stroke-width="7"/>
    <path d="M43 54C59 58 61 44 55 41" fill="none" stroke="var(--cat-main, #F3B777)" stroke-width="3"/>
  </g>
  <path class="body" d="M22 33C17 39 15 50 18 57Q32 62 46 57C49 50 45 38 41 33Z"/>
  <path d="M25 44C21 51 24 57 32 57S43 51 39 44C36 39 28 39 25 44" fill="var(--cat-belly, #FFF0DA)" stroke="none"/>
  <g class="paw paw-wave">
    <path d="M43 44C48 43 51 37 50 32C49 28 44 29 44 33L40 39Z"/>
    <path d="M46 33L46 35" fill="none"/>
  </g>
  <path class="body" d="M15 22C15 12 49 12 49 22V26C49 36 41 40 32 40S15 36 15 26Z"/>
  <path class="ear" d="M16 21L14 8Q14 5 17 7L26 15Z"/>
  <path class="ear" d="M38 15L47 7Q50 5 50 8L48 21Z"/>
  <path d="M18 13L19 17M46 13L45 17" stroke="var(--cat-belly, #FFF0DA)"/>
  <path d="M28 17L29 21M36 17L35 21" stroke="var(--cat-stripe, #CF8B50)"/>
  <path class="eye" d="M22 27Q24 24 26 27M38 27Q40 24 42 27" fill="none"/>
  <path d="M30 30L32 32L34 30Z" fill="var(--cat-ink, #765447)" stroke="none"/>
  <path d="M28 33Q30 36 32 33Q34 36 36 33" fill="none"/>
  <path d="M23 39Q32 43 41 39" stroke="var(--cat-accent, #3182BD)" stroke-width="3" fill="none"/>
  <path class="paw" d="M22 51V57Q25 60 29 57V52M35 52V57Q39 60 42 57V51"/>
</g>
    <g v-if="happy && !folded" class="love" fill="#DF7C83" stroke="none">
      <path d="M53 14C44 9 49 3 53 7C57 3 62 9 53 14Z"/>
    </g>
  </svg>
</template>

<style scoped>
.cat{--cat-main:#F3B777;--cat-ink:#765447;--cat-belly:#FFF0DA;--cat-stripe:#CF8B50;--cat-accent:#3182BD;overflow:visible}
.cat-seated .tail{transform-origin:43px 54px;animation:cat-tail 3s ease-in-out infinite}
.cat-folded .tail{transform-origin:17px 44px;animation:cat-tail 4s ease-in-out infinite}
.cat-seated .paw-wave{transform-origin:42px 42px;animation:cat-wave 4s ease-in-out infinite}
.cat .eye{transform-box:fill-box;transform-origin:center;animation:cat-blink 5.5s ease-in-out infinite}
@keyframes cat-tail{0%,100%{transform:rotate(-3deg)}50%{transform:rotate(5deg)}}
@keyframes cat-wave{0%,65%,100%{transform:rotate(0)}72%,86%{transform:rotate(12deg)}79%,93%{transform:rotate(-6deg)}}
@keyframes cat-blink{0%,43%,49%,100%{transform:scaleY(1)}46%{transform:scaleY(.12)}}
@media(prefers-reduced-motion:reduce){.cat .tail,.cat .paw,.cat .eye{animation:none}}

.cat{display:block;width:100%;height:100%}
.cat-seated.is-thinking .ear{transform-box:fill-box;transform-origin:center bottom;animation:cat-listen 1.4s ease-in-out infinite}
.cat-seated.is-thinking .paw-wave{animation:none}
.cat.is-replying .tail{animation-duration:1.2s}
.cat.is-error{--cat-accent:#C67953}
.cat.is-happy .paw-wave{animation:cat-wave .8s ease-in-out infinite}
.love{animation:cat-love 1.6s ease-out both;pointer-events:none}
@keyframes cat-listen{50%{transform:rotate(8deg)}}
@keyframes cat-love{from{opacity:0;transform:translateY(5px)}25%,70%{opacity:1}to{opacity:0;transform:translateY(-5px)}}
@media(prefers-reduced-motion:reduce){.cat *, .love{animation:none!important}}
</style>

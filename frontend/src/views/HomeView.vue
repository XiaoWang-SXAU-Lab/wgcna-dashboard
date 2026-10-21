<script setup lang="ts">
import { useRouter } from 'vue-router'
import { t } from '../utils/i18n'
const router = useRouter()

const stats = [
  { value: '6', labelKey: 'home.stat1' },
  { value: '5+', labelKey: 'home.stat2' },
  { value: 'AI', labelKey: 'home.stat3' },
  { value: '2', labelKey: 'home.stat4' },
]

const features = [
  { icon: 'bi-funnel', titleKey: 'home.f1title', descKey: 'home.f1desc', to: '/data-import' },
  { icon: 'bi-activity', titleKey: 'home.f2title', descKey: 'home.f2desc', to: '/sft' },
  { icon: 'bi-diagram-3', titleKey: 'home.f3title', descKey: 'home.f3desc', to: '/network' },
  { icon: 'bi-table', titleKey: 'home.f4title', descKey: 'home.f4desc', to: '/module-trait' },
  { icon: 'bi-search', titleKey: 'home.f5title', descKey: 'home.f5desc', to: '/interested-mod' },
  { icon: 'bi-stars', titleKey: 'home.f6title', descKey: 'home.f6desc', to: '/hub-gene' },
]
</script>

<template>
  <div class="home-page">
    <section class="home-hero">
      <div class="container-fluid px-4 px-xl-5">
        <div class="home-hero-grid">
          <div>
            <div class="home-kicker">{{ t('home.kicker') }}</div>
            <h1 class="home-title">{{ t('home.title') }}</h1>
            <p class="home-lead">{{ t('home.lead') }}</p>
            <div class="home-actions d-flex flex-wrap gap-2">
              <button class="btn btn-home-primary" @click="router.push('/data-import')"><i class="bi bi-play-fill me-1"></i>{{ t('home.start') }}</button>
              <button class="btn btn-home-secondary" @click="router.push('/one-step')"><i class="bi bi-lightning-charge-fill me-1"></i>{{ t('home.oneStep') }}</button>
            </div>
            <div class="home-quick-stats">
              <div v-for="s in stats" :key="s.value" class="home-stat">
                <div class="home-stat-value">{{ s.value }}</div>
                <div class="home-stat-label">{{ t(s.labelKey) }}</div>
              </div>
            </div>
          </div>
          <div class="home-hero-visual">
            <svg viewBox="0 0 480 340" xmlns="http://www.w3.org/2000/svg" class="home-network">
              <g stroke="#cfe2e2" stroke-width="1.5">
                <line x1="80" y1="70" x2="170" y2="120"/>
                <line x1="80" y1="70" x2="140" y2="220"/>
                <line x1="170" y1="120" x2="300" y2="90"/>
                <line x1="170" y1="120" x2="260" y2="200"/>
                <line x1="300" y1="90" x2="400" y2="60"/>
                <line x1="300" y1="90" x2="400" y2="150"/>
                <line x1="400" y1="150" x2="400" y2="250"/>
                <line x1="140" y1="220" x2="260" y2="200"/>
                <line x1="260" y1="200" x2="400" y2="250"/>
                <line x1="260" y1="200" x2="360" y2="290"/>
                <line x1="140" y1="220" x2="60" y2="280"/>
                <line x1="170" y1="120" x2="140" y2="220"/>
              </g>
              <g>
                <circle cx="80" cy="70" r="17" fill="#1f78b4"/>
                <circle cx="170" cy="120" r="24" fill="#33a02c"/>
                <circle cx="300" cy="90" r="15" fill="#e31a1c"/>
                <circle cx="400" cy="60" r="12" fill="#ff7f00"/>
                <circle cx="400" cy="150" r="18" fill="#6a3d9a"/>
                <circle cx="400" cy="250" r="14" fill="#b15928"/>
                <circle cx="260" cy="200" r="28" fill="#a6cee3"/>
                <circle cx="140" cy="220" r="13" fill="#fb9a99"/>
                <circle cx="60" cy="280" r="11" fill="#e31a1c"/>
                <circle cx="360" cy="290" r="11" fill="#33a02c"/>
              </g>
              <g font-size="11" fill="#6b7a82" text-anchor="middle">
                <text x="80" y="44">MEblue</text>
                <text x="170" y="96">MEgreen</text>
                <text x="300" y="64">MEred</text>
                <text x="400" y="38">MEyellow</text>
                <text x="400" y="132">MEpurple</text>
                <text x="260" y="246">MEturquoise</text>
              </g>
            </svg>
          </div>
        </div>
      </div>
    </section>

    <section class="home-features-floating">
      <div class="container-fluid px-4 px-xl-5">
        <h2 class="home-section-title">{{ t('home.sectionTitle') }}</h2>
        <div class="home-feature-grid">
          <router-link v-for="f in features" :key="f.to" :to="f.to" class="home-feature text-decoration-none">
            <div class="home-feature-icon"><i :class="['bi', f.icon]"></i></div>
            <div class="home-feature-title">{{ t(f.titleKey) }}</div>
            <p class="home-feature-text">{{ t(f.descKey) }}</p>
          </router-link>
        </div>
      </div>
    </section>

    <section class="home-meta-band">
      <div class="container-fluid px-4 px-xl-5">
        <div class="home-two-col">
          <div class="home-panel">
            <h4 class="fw-bold text-primary mb-3">{{ t('home.workflowTitle') }}</h4>
            <p class="mb-0" style="color:#5b6871;line-height:1.7">{{ t('home.workflowText') }}</p>
          </div>
          <div class="home-panel">
            <h4 class="fw-bold text-primary mb-3">{{ t('home.citeTitle') }}</h4>
            <div class="home-cite">{{ t('home.cite1') }}</div>
            <div class="home-cite">{{ t('home.cite2') }}</div>
            <div class="home-cite mb-0">{{ t('home.cite3') }}</div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.home-page { min-height: calc(100vh - 62px); background: #f8f9fa; color: #26343d; }
.home-hero { background: linear-gradient(135deg, #008080 0%, #20c997 100%); color: #fff; padding: 64px 0 96px; position: relative; overflow: hidden; }
.home-hero-grid { display: grid; grid-template-columns: minmax(0, 0.95fr) minmax(400px, 1.05fr); gap: 34px; align-items: center; }
.home-kicker { color: rgba(255,255,255,0.88); font-weight: 800; font-size: 0.92rem; margin-bottom: 0.65rem; }
.home-title { font-size: 2.85rem; line-height: 1.17; font-weight: 850; margin: 0 0 1rem; }
.home-lead { color: rgba(255,255,255,0.92); font-size: 1.08rem; line-height: 1.72; margin-bottom: 1.25rem; }
.home-actions .btn { border-radius: 7px; font-weight: 800; padding: 0.68rem 1.02rem; }
.btn-home-primary { background: #fff; color: #008080; border: 1px solid #fff; box-shadow: 0 8px 20px rgba(0,0,0,0.16); }
.btn-home-primary:hover { background: #f4fbfb; color: #006f6f; }
.btn-home-secondary { color: #fff; background: transparent; border: 1px solid rgba(255,255,255,0.55); }
.btn-home-secondary:hover { color: #fff; background: rgba(255,255,255,0.12); }
.home-hero-visual { background: #fff; border: 1px solid rgba(255,255,255,0.35); border-radius: 20px; padding: 18px; box-shadow: 0 20px 50px rgba(0,0,0,0.26); transform: rotate(-2deg); }
.home-network { width: 100%; display: block; border-radius: 14px; }
.home-quick-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.home-stat { background: rgba(255,255,255,0.1); border: 1px solid rgba(255,255,255,0.18); border-radius: 8px; padding: 12px; }
.home-stat-value { color: #fff; font-size: 1.25rem; font-weight: 850; }
.home-stat-label { color: #c7dbdc; font-size: 0.82rem; line-height: 1.35; }
.home-features-floating { margin-top: -66px; position: relative; z-index: 2; padding-bottom: 32px; }
.home-section-title { font-size: 1.35rem; font-weight: 850; color: #12383b; margin: 0 0 14px; }
.home-feature-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.home-feature { background: #fff; border: 1px solid rgba(0,0,0,0.04); border-top: 5px solid transparent; border-radius: 12px; padding: 18px; min-height: 168px; box-shadow: 0 12px 30px rgba(0,0,0,0.08); transition: transform 0.24s ease, border-top-color 0.24s ease, box-shadow 0.24s ease; color: inherit; }
.home-feature:hover { transform: translateY(-5px); border-top-color: #008080; box-shadow: 0 16px 34px rgba(0,0,0,0.11); }
.home-feature-icon { width: 38px; height: 38px; display: inline-flex; align-items: center; justify-content: center; border-radius: 8px; background: #e5f5f1; color: #008080; font-size: 1.1rem; margin-bottom: 0.9rem; }
.home-feature-title { font-weight: 850; color: #24313a; margin-bottom: 0.55rem; }
.home-feature-text { color: #5f6d76; line-height: 1.62; font-size: 0.94rem; margin: 0; }
.home-meta-band { background: #fff; border-top: 1px solid #dbe3e6; padding: 26px 0; }
.home-two-col { display: grid; grid-template-columns: minmax(0, 0.95fr) minmax(320px, 1.05fr); gap: 26px; }
.home-panel { background: #f8fbfb; border: 1px solid #dbe3e6; border-radius: 8px; padding: 18px; }
.home-cite { border-left: 4px solid #2fb28f; padding-left: 14px; color: #56636c; line-height: 1.65; margin-bottom: 12px; }
@media (max-width: 1180px) { .home-hero-grid, .home-two-col { grid-template-columns: 1fr; } .home-feature-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 720px) { .home-title { font-size: 1.82rem; } .home-hero { padding-bottom: 70px; } .home-features-floating { margin-top: -44px; } .home-feature-grid, .home-quick-stats { grid-template-columns: 1fr; } }
</style>

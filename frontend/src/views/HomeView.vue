<script setup lang="ts">
import { computed } from 'vue'
import { locale, t } from '../utils/i18n'
const en = computed(() => locale.value === 'en')
const text = (zh: string, english: string) => en.value ? english : zh
const capabilities = computed(() => [
  ['COMPUTE', text('Rust 计算引擎', 'Rust computation engine'), text('网络构建与模块识别', 'Network construction and module detection')],
  ['EXPLORE', text('交互式结果探索', 'Interactive exploration'), text('模块性状 · GS / MM · Hub 网络', 'Module–trait · GS / MM · Hub networks')],
  ['INTERPRET', text('模块功能分析', 'Module function analysis'), text('GO / KEGG 富集与基因注释', 'GO / KEGG enrichment and gene annotation')],
  ['EXPORT', text('离线分析报告', 'Offline analysis reports'), text('交互图表 · 分析参数 · 结果导出', 'Interactive charts · Parameters · Results')],
])
const steps = computed(() => [
  ['/data-import', t('nav.dataImport')], ['/sft', t('nav.sft')], ['/network', t('nav.network')],
  ['/module-trait', t('nav.moduleTrait')], ['/module-explorer', t('nav.moduleExplorer')], ['/report', t('nav.report')],
])
const clusters = [[102,113,66,'#169c9c'],[280,73,48,'#577cb8'],[283,172,38,'#b18358']] as const
const nodes = clusters.flatMap(([cx,cy,rad,color], k) => Array.from({length:25},(_,i) => {
  const a=i*2.39996, d=rad*Math.sqrt((i+.5)/25)
  return { x:cx+Math.cos(a)*d, y:cy+Math.sin(a)*d, color, group:k, r:i%6===0?5:2.7 }
}))
const edges = nodes.flatMap((n,i) => nodes.slice(i+1).filter(m => m.group===n.group && Math.hypot(n.x-m.x,n.y-m.y)<clusters[n.group]![2]*.75).map(m=>({a:n,b:m})))
const year = new Date().getFullYear()
</script>
<template>
  <div class="research-home">
    <div class="home-eyebrow">WEIGHTED GENE CO-EXPRESSION NETWORK ANALYSIS</div>
    <section class="research-hero">
      <div>
        <div class="home-tag">RUST ENGINE / INTERACTIVE ANALYSIS</div>
        <h1><img class="home-brand-icon" src="/rsWGCNA_icon.svg" alt="" />rsWGCNA<span>{{ text('基因共表达网络分析', 'Gene co-expression network analysis') }}</span></h1>
        <p class="home-intro">{{ text('从表达矩阵、模块识别到候选基因筛选的集成分析工具。', 'An integrated tool for expression matrices, module detection and candidate gene screening.') }}</p>
        <p class="home-description">{{ text('使用 Rust 计算引擎构建共表达网络，交互探索模块与性状的关联，结合 GO / KEGG 富集分析解释模块功能，并导出可离线浏览的分析报告。', 'Build co-expression networks with a Rust engine. Explore module–trait associations, interpret module functions with GO / KEGG enrichment, and export reports for offline viewing.') }}</p>
        <router-link to="/data-import" class="btn btn-primary mt-3">{{ text('开始分析', 'Start analysis') }} <i class="bi bi-arrow-right ms-3" aria-hidden="true"></i></router-link>
      </div>
      <div class="home-network-panel">
        <div class="home-figure-heading">NETWORK → MODULE → FUNCTION<small>{{ text('分析流程示意 · 非实际结果', 'Schematic illustration · Not analysis results') }}</small></div>
        <svg viewBox="0 0 400 225" role="img" :aria-label="text('共表达模块网络示意', 'Schematic co-expression module network')">
          <line v-for="(edge,i) in edges" :key="'e'+i" :x1="edge.a.x" :y1="edge.a.y" :x2="edge.b.x" :y2="edge.b.y" :stroke="edge.a.color" stroke-opacity=".25" stroke-width=".8" />
          <circle v-for="(node,i) in nodes" :key="i" :cx="node.x" :cy="node.y" :r="node.r" :fill="node.color" />
        </svg>
        <div class="home-module-legend"><span><b style="background:#169c9c"></b>turquoise</span><span><b style="background:#577cb8"></b>blue</span><span><b style="background:#b18358"></b>brown</span></div>
        <div class="home-figure-note">Expression matrix → Co-expression modules → Biological interpretation</div>
      </div>
    </section>
    <section class="home-capabilities">
      <article v-for="cap in capabilities" :key="cap[0]"><small>{{ cap[0] }}</small><h3>{{ cap[1] }}</h3><p>{{ cap[2] }}</p></article>
    </section>
    <section class="home-workflow">
      <div class="home-workflow-title"><h2>{{ text('分析流程', 'Analysis workflow') }}</h2><span>ANALYSIS WORKFLOW</span></div>
      <div class="home-step-grid"><router-link v-for="(step,i) in steps" :key="step[0]" :to="step[0]!" class="home-step"><small>0{{ i+1 }}</small><span>{{ step[1] }}</span><i class="bi bi-arrow-up-right" aria-hidden="true"></i></router-link></div>
    </section>
    <div class="home-methods">{{ text('方法参考', 'Methods') }}: Zhang &amp; Horvath (2005) · Langfelder &amp; Horvath (2008)</div>
    <footer class="home-footer"><div>© {{ year }} rsWGCNA<small>{{ text('山西农业大学生命科学学院', 'College of Life Sciences, Shanxi Agricultural University') }}</small><small>{{ text('山西农业大学后稷实验室', 'Houji Laboratory, Shanxi Agricultural University') }}</small></div><a href="https://xwang-lab-sxau.cn/" target="_blank" rel="noopener noreferrer">{{ text('个人实验室主页', 'Personal laboratory website') }} ↗<small>xwang-lab-sxau.cn</small></a></footer>
  </div>
</template>
<style scoped>
.home-brand-icon { width: 1em; height: 1em; margin-right: .22em; vertical-align: -.1em; }
.research-home{color:#223747}.home-eyebrow{font-size:11px;letter-spacing:.12em;color:#607589;padding:0 0 20px}.research-hero{display:grid;grid-template-columns:1.05fr 1fr;gap:clamp(24px,4vw,80px);align-items:center;padding:18px 0 38px}.home-tag{font-size:11px;letter-spacing:.12em;color:#067d89}h1{font-size:clamp(42px,4vw,64px);line-height:1.15;font-weight:650;letter-spacing:-.04em;margin:20px 0}h1 span{display:block;font-size:clamp(22px,2vw,30px);font-weight:500;letter-spacing:0;margin-top:14px}.home-intro{font-size:17px;margin:22px 0 12px}.home-description{color:#607589;font-size:14px;line-height:1.9;max-width:640px}.home-network-panel{background:#fff;border:1px solid #dce5ec;border-radius:8px;overflow:hidden}.home-figure-heading{padding:14px 18px;font-size:11px;letter-spacing:.08em;color:#067d89;border-bottom:1px solid #e3eaf0}.home-figure-heading small{display:block;font-size:11px;letter-spacing:0;color:#607589;margin-top:4px}.home-network-panel svg{display:block;width:100%;max-height:310px}.home-module-legend{display:flex;gap:24px;flex-wrap:wrap;padding:0 18px 16px;font-size:12px}.home-module-legend b{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:8px}.home-figure-note{padding:12px 18px;border-top:1px solid #e3eaf0;font-size:11px;color:#607589}.home-capabilities{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:24px;padding:24px 0;border-top:1px solid #dce5ec;border-bottom:1px solid #dce5ec}.home-capabilities small{font-size:11px;letter-spacing:.1em;color:#067d89}.home-capabilities h3{font-size:17px;margin:9px 0;font-weight:600}.home-capabilities p{font-size:13px;color:#607589;margin:0}.home-workflow{padding:30px 0}.home-workflow-title{display:flex;gap:18px;align-items:center;flex-wrap:wrap}.home-workflow-title h2{font-size:20px;margin:0}.home-workflow-title>span{font-size:11px;letter-spacing:.1em;color:#607589}.home-step-grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:12px;margin-top:20px}.home-step{background:#fff;border:1px solid #dce5ec;padding:16px;border-radius:6px;text-decoration:none;color:#223747;position:relative}.home-step small{display:block;color:#067d89;font-size:12px;margin-bottom:12px}.home-step span{font-size:14px}.home-step i{position:absolute;right:12px;top:14px;color:#607589}.home-step:hover{border-color:#70afb7;background:#f2f9fa}.home-methods{color:#607589;font-size:12px;padding-bottom:26px}.home-footer{display:flex;justify-content:space-between;gap:20px;flex-wrap:wrap;border-top:1px solid #dce5ec;padding:22px 0;font-size:12px}.home-footer small{display:block;color:#607589;font-size:12px;margin-top:6px}.home-footer a{color:#067d89;text-decoration:none}.home-footer a:hover{text-decoration:underline}@media(min-width:1600px){.research-hero{padding-top:30px;padding-bottom:50px}.home-network-panel svg{max-height:370px}}@media(max-width:1000px){.home-step-grid{grid-template-columns:repeat(3,minmax(0,1fr))}.home-capabilities{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:767px){.research-hero{grid-template-columns:1fr}.home-step-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.home-capabilities{gap:20px}.home-eyebrow{overflow-wrap:anywhere}}
</style>

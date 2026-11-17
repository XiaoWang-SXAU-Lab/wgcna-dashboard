<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type {
  RegutraceDomainInfo,
  RegutraceGFFSegment,
  RegutracePromoterHit,
  RegutracePromoterView,
  RegutraceTFView,
  RegutraceTranscriptModel,
} from '../api/client'
import { locale } from '../utils/i18n'

const props = defineProps<{
  view: RegutraceTFView | null
  promoter?: RegutracePromoterView | null
  tfId?: string
  selectedHit?: RegutracePromoterHit | null
}>()
const emit = defineEmits<{
  (e: 'select-hit', hit: RegutracePromoterHit): void
  (e: 'focus-motif', motif: string): void
}>()

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en

interface FeatureDetail {
  kind: string
  label: string
  start: number
  end: number
  note?: string
  domain?: RegutraceDomainInfo
}

const selectedTranscriptId = ref('')
const zoom = ref(1)
const selectedFeature = ref<FeatureDetail | null>(null)
const hover = ref<{ x: number; y: number; title: string; lines: string[] } | null>(null)
const wrap = ref<HTMLElement>()

const gene = computed(() => props.view?.gene ?? null)
const transcripts = computed<RegutraceTranscriptModel[]>(() => gene.value?.transcripts ?? [])
const transcript = computed<RegutraceTranscriptModel | null>(() => {
  const list = transcripts.value
  return list.find((tr) => tr.id === selectedTranscriptId.value) ?? list[0] ?? null
})
const proteins = computed(() => props.view?.proteins ?? [])
const protein = computed(() => {
  const tr = transcript.value
  if (!tr) return null
  return proteins.value.find((p) => p.protein_id === tr.protein_id) ??
    (proteins.value.length === 1 ? proteins.value[0] : null)
})
const domains = computed(() => protein.value?.domains ?? [])
const projected = computed(() => transcript.value?.projected_domains ?? [])
const mappings = computed(() => props.view?.motif_mappings ?? [])
const targetGene = computed(() => props.view?.target_gene?.gene ?? null)
const targetTranscript = computed<RegutraceTranscriptModel | null>(() => {
  const list = targetGene.value?.transcripts ?? []
  const wanted = props.view?.target_gene?.default_transcript
  return list.find((tr) => tr.id === wanted) ?? list[0] ?? null
})

watch(
  () => props.view,
  (view) => {
    selectedTranscriptId.value = view?.default_transcript ?? view?.gene?.transcripts?.[0]?.id ?? ''
    zoom.value = 1
    selectedFeature.value = null
    hover.value = null
  },
  { immediate: true },
)

const LEFT = 90
const RIGHT = 28
const BASE_WIDTH = 1000
const svgWidth = computed(() => Math.round(BASE_WIDTH * zoom.value))
const plotWidth = computed(() => svgWidth.value - LEFT - RIGHT)
const geneStart = computed(() => {
  const gm = gene.value
  if (!gm) return 1
  const starts = [gm.start, ...transcripts.value.map((tr) => tr.start)].filter((v) => v > 0)
  return starts.length ? Math.min(...starts) : 1
})
const geneEnd = computed(() => {
  const gm = gene.value
  if (!gm) return 2
  const ends = [gm.end, ...transcripts.value.map((tr) => tr.end)].filter((v) => v > 0)
  const e = ends.length ? Math.max(...ends) : geneStart.value + 1
  return Math.max(e, geneStart.value + 1)
})
const span = computed(() => Math.max(1, geneEnd.value - geneStart.value))
const x = (pos: number) => LEFT + ((pos - geneStart.value) / span.value) * plotWidth.value
const width = (start: number, end: number) => Math.max(2, ((end - start + 1) / span.value) * plotWidth.value)

const targetStart = computed(() => {
  const g = targetGene.value
  if (!g) return 1
  const starts = [g.start, ...(g.transcripts ?? []).map((tr) => tr.start)].filter((v) => v > 0)
  return starts.length ? Math.min(...starts) : 1
})
const targetEnd = computed(() => {
  const g = targetGene.value
  if (!g) return 2
  const ends = [g.end, ...(g.transcripts ?? []).map((tr) => tr.end)].filter((v) => v > 0)
  return Math.max(ends.length ? Math.max(...ends) : targetStart.value + 1, targetStart.value + 1)
})
const targetSpan = computed(() => Math.max(1, targetEnd.value - targetStart.value))
const targetX = (pos: number) => LEFT + ((pos - targetStart.value) / targetSpan.value) * plotWidth.value
const targetWidth = (start: number, end: number) => Math.max(2, ((end - start + 1) / targetSpan.value) * plotWidth.value)

const hasPromoter = computed(() => !!props.promoter && promoterHits.value.length > 0)
const promoterHits = computed(() => {
  const p = props.promoter
  if (!p) return []
  const tf = props.tfId
  if (!tf) return p.hits
  return p.hits.filter((hit) => hit.tfs.some((item) => item.id === tf))
})

const geneY = 70
const projectedY = 145
const proteinY = 220
const promoterY = 310
const targetTop = computed(() => {
  let top = hasPromoter.value ? 360 : 285
  for (const site of promoterSites.value) top = Math.max(top, site.y + 30)
  return top
})
const targetGeneY = computed(() => targetTop.value + 42)
const hasTarget = computed(() => !!targetGene.value && !!targetTranscript.value)
const svgHeight = computed(() => {
  if (hasTarget.value) return targetGeneY.value + 62
  let height = hasPromoter.value ? 370 : 272
  for (const site of promoterSites.value) height = Math.max(height, site.y + 24)
  return height
})

function segments(values?: RegutraceGFFSegment[]): RegutraceGFFSegment[] {
  return [...(values ?? [])].sort((a, b) => a.start - b.start || a.end - b.end)
}
function exonSegments() { return segments(transcript.value?.exons) }
function cdsSegments() { return segments(transcript.value?.cds) }
function fiveUtr() { return segments(transcript.value?.five_utr) }
function threeUtr() { return segments(transcript.value?.three_utr) }
function introns() {
  const exons = exonSegments()
  const out: { start: number; end: number }[] = []
  for (let i = 1; i < exons.length; i++) {
    if (exons[i].start > exons[i - 1].end + 1) {
      out.push({ start: exons[i - 1].end + 1, end: exons[i].start - 1 })
    }
  }
  return out
}
function targetExonSegments() { return segments(targetTranscript.value?.exons) }
function targetCDSSegments() { return segments(targetTranscript.value?.cds) }
function targetFiveUTR() { return segments(targetTranscript.value?.five_utr) }
function targetThreeUTR() { return segments(targetTranscript.value?.three_utr) }
function targetIntrons() {
  const exons = targetExonSegments()
  const out: { start: number; end: number }[] = []
  for (let i = 1; i < exons.length; i++) {
    if (exons[i].start > exons[i - 1].end + 1) {
      out.push({ start: exons[i - 1].end + 1, end: exons[i].start - 1 })
    }
  }
  return out
}
const targetRuler = computed(() => {
  const step = niceStep(targetSpan.value / 5)
  const first = Math.ceil(targetStart.value / step) * step
  const out: { x: number; label: string }[] = []
  for (let pos = first; pos <= targetEnd.value; pos += step) {
    out.push({ x: targetX(pos), label: formatCoord(pos) })
  }
  return out
})

function niceStep(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1
  const power = 10 ** Math.floor(Math.log10(value))
  const normalized = value / power
  const factor = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10
  return factor * power
}

const ruler = computed(() => {
  const step = niceStep(span.value / 6)
  const first = Math.ceil(geneStart.value / step) * step
  const out: { x: number; label: string }[] = []
  for (let pos = first; pos <= geneEnd.value; pos += step) {
    out.push({ x: x(pos), label: formatCoord(pos) })
  }
  return out
})

function formatCoord(value: number): string {
  return Math.round(value).toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
}
function formatEvalue(value?: number): string {
  if (value == null || !Number.isFinite(value)) return '—'
  if (value === 0) return '0'
  return value < 1e-3 || value >= 1000 ? value.toExponential(2) : value.toPrecision(3)
}
function proteinLength(): number {
  if (protein.value?.length) return protein.value.length
  const max = Math.max(0, ...domains.value.map((d) => d.protein_end))
  return max || Math.max(1, Math.round((transcript.value?.cds.reduce((n, s) => n + s.end - s.start + 1, 0) ?? 0) / 3))
}
function domainColor(d: RegutraceDomainInfo): string {
  const palette = ['#c2410c', '#0f766e', '#7c3aed', '#0369a1', '#b45309']
  const key = `${d.family ?? ''}${d.model_accession ?? ''}${d.model_name ?? ''}`
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) >>> 0
  return palette[h % palette.length]
}
function domainLabel(d: RegutraceDomainInfo): string {
  return d.model_name || d.model_accession || d.family || text('domain', 'domain')
}
function selectedDomain(): RegutraceDomainInfo | null {
  return selectedFeature.value?.domain ?? null
}

function setHover(event: Event, title: string, lines: string[]) {
  const rect = wrap.value?.getBoundingClientRect()
  const mouse = event as MouseEvent
  hover.value = {
    x: (mouse.clientX ?? 0) - (rect?.left ?? 0) + 12,
    y: (mouse.clientY ?? 0) - (rect?.top ?? 0) + 12,
    title,
    lines,
  }
}
function selectSegment(kind: string, seg: RegutraceGFFSegment, label: string) {
  selectedFeature.value = {
    kind,
    label,
    start: seg.start,
    end: seg.end,
    note: text('基因组坐标；坐标为 1-based closed。', 'Genomic coordinates; 1-based closed.'),
  }
}
function selectDomain(d: RegutraceDomainInfo) {
  selectedFeature.value = {
    kind: text('HMMER domain', 'HMMER domain'),
    label: domainLabel(d),
    start: d.protein_start,
    end: d.protein_end,
    note: `${d.family ?? d.subfamily ?? ''}${d.model_accession ? ` · ${d.model_accession}` : ''} · E=${formatEvalue(d.full_evalue ?? d.conditional_evalue)}`,
    domain: d,
  }
}

function domainLines(d: RegutraceDomainInfo): string[] {
  return [
    d.model_accession ? `${d.model_accession}${d.model_name ? ` · ${d.model_name}` : ''}` : (d.model_name ?? ''),
    `${text('蛋白位置', 'protein')} ${d.protein_start}-${d.protein_end} aa`,
    `${text('模型位置', 'model')} ${d.model_start ?? '—'}-${d.model_end ?? '—'}`,
    `E=${formatEvalue(d.full_evalue ?? d.conditional_evalue)} · score=${d.domain_score?.toFixed(1) ?? '—'}`,
  ].filter(Boolean)
}

function proteinTicks(): { x: number; label: string }[] {
  const length = proteinLength()
  const step = niceStep(length / 5)
  const out: { x: number; label: string }[] = []
  for (let aa = 0; aa <= length; aa += step) {
    out.push({ x: LEFT + (aa / length) * plotWidth.value, label: `${formatCoord(aa)} aa` })
  }
  return out
}
function proteinX(aa: number): number {
  return LEFT + (Math.max(0, aa) / proteinLength()) * plotWidth.value
}

function promoterX(pos: number): number {
  const length = props.promoter?.length ?? 1
  return LEFT + ((Math.max(1, pos) - 1) / length) * plotWidth.value
}
interface PromoterLayout { hit: RegutracePromoterHit; x: number; width: number; y: number }
const promoterSites = computed<PromoterLayout[]>(() => {
  const p = props.promoter
  if (!p) return []
  const plus: number[] = []
  const minus: number[] = []
  const out: PromoterLayout[] = []
  for (const hit of promoterHits.value) {
    const isPlus = hit.strand !== '-'
    const lanes = isPlus ? plus : minus
    let lane = lanes.findIndex((end) => end < hit.start)
    if (lane < 0) {
      lane = lanes.length
      lanes.push(0)
    }
    lanes[lane] = hit.end
    out.push({
      hit,
      x: promoterX(hit.start),
      width: Math.max(3, ((hit.end - hit.start + 1) / p.length) * plotWidth.value),
      y: isPlus ? promoterY - 18 - lane * 8 : promoterY + 10 + lane * 8,
    })
  }
  return out
})
function isSelected(hit: RegutracePromoterHit): boolean {
  const selected = props.selectedHit
  return !!selected && selected.motif_id === hit.motif_id && selected.start === hit.start &&
    selected.end === hit.end && selected.strand === hit.strand
}
function selectPromoterHit(hit: RegutracePromoterHit) {
  emit('select-hit', hit)
}
</script>

<template>
  <div v-if="view" class="rt-tf-evidence">
    <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
      <div>
        <h6 class="mb-1">
          {{ view.tf_name || view.tf_symbol || view.tf_id }}
          <span class="text-muted fw-normal small">{{ view.tf_id }}</span>
        </h6>
        <div class="small text-muted">
          <span v-if="view.tf_family" class="badge text-bg-light border me-1">{{ view.tf_family }}</span>
          <span v-if="view.tf_status" class="badge text-bg-info me-1">{{ view.tf_status }}</span>
          <span v-if="view.tf_source">{{ view.tf_source }}</span>
        </div>
      </div>
      <div class="ms-auto d-flex align-items-center gap-2 flex-wrap">
        <select v-if="transcripts.length > 1" v-model="selectedTranscriptId" class="form-select form-select-sm" style="max-width: 270px">
          <option v-for="tr in transcripts" :key="tr.id" :value="tr.id">
            {{ tr.id }}{{ tr.protein_id ? ` · ${tr.protein_id}` : '' }}
          </option>
        </select>
        <div class="btn-group btn-group-sm" role="group" :aria-label="text('缩放', 'Zoom')">
          <button class="btn btn-outline-secondary" :disabled="zoom <= 1" @click="zoom = Math.max(1, zoom / 1.5)">−</button>
          <button class="btn btn-outline-secondary" @click="zoom = 1">{{ Math.round(zoom * 100) }}%</button>
          <button class="btn btn-outline-secondary" :disabled="zoom >= 8" @click="zoom = Math.min(8, zoom * 1.5)">+</button>
        </div>
      </div>
    </div>

    <div v-if="!view.available" class="alert alert-light border small mb-0">
      <div class="fw-semibold mb-1">{{ text('没有可绘制的基因结构或 domain', 'No gene structure or domain evidence') }}</div>
      <div>{{ view.reason || text('这个 TF 没有 GFF 基因模型或 HMMER domain。', 'This TF has no GFF gene model or HMMER domain.') }}</div>
    </div>

    <template v-else>
      <div v-if="transcript" ref="wrap" class="rt-tf-canvas" @mouseleave="hover = null">
        <div class="rt-tf-scroll">
          <svg :width="svgWidth" :height="svgHeight" :viewBox="`0 0 ${svgWidth} ${svgHeight}`" role="img"
               :aria-label="text('TF 基因结构和 domain 证据图', 'TF gene structure and domain evidence')">
            <g>
              <line :x1="LEFT" :x2="LEFT + plotWidth" :y1="24" :y2="24" class="rt-tf-axis" />
              <g v-for="tick in ruler" :key="`r-${tick.x}`">
                <line :x1="tick.x" :x2="tick.x" :y1="20" :y2="28" class="rt-tf-tick" />
                <text :x="tick.x" :y="15" text-anchor="middle" class="rt-tf-ruler">{{ tick.label }}</text>
              </g>
            </g>

            <text x="8" :y="geneY + 5" class="rt-tf-row-label">{{ text('基因结构', 'Gene model') }}</text>
            <line :x1="x(geneStart)" :x2="x(geneEnd)" :y1="geneY" :y2="geneY" class="rt-tf-gene-line" />
            <g v-for="(intron, i) in introns()" :key="`intron-${i}`">
              <line :x1="x(intron.start)" :x2="x(intron.end)" :y1="geneY" :y2="geneY" class="rt-tf-intron" />
              <path :d="`M ${(x(intron.start) + x(intron.end)) / 2 - 4} ${geneY - 3} l 4 6 l 4 -6`" class="rt-tf-chevron" />
            </g>
            <g v-for="(exon, i) in exonSegments()" :key="`exon-${i}`">
              <rect :x="x(exon.start)" :y="geneY - 9" :width="width(exon.start, exon.end)" height="18" rx="2"
                    class="rt-tf-exon" tabindex="0"
                    @mouseenter="setHover($event, text('exon', 'exon'), [`${formatCoord(exon.start)}-${formatCoord(exon.end)}`])"
                    @focus="setHover($event, text('exon', 'exon'), [`${formatCoord(exon.start)}-${formatCoord(exon.end)}`])"
                    @click="selectSegment('exon', exon, text('exon', 'exon'))"
                    @keydown.enter="selectSegment('exon', exon, text('exon', 'exon'))" />
            </g>
            <g v-for="(seg, i) in cdsSegments()" :key="`cds-${i}`">
              <rect :x="x(seg.start)" :y="geneY - 15" :width="width(seg.start, seg.end)" height="30" rx="2"
                    class="rt-tf-cds" tabindex="0"
                    @mouseenter="setHover($event, 'CDS', [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                    @focus="setHover($event, 'CDS', [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                    @click="selectSegment('CDS', seg, 'CDS')"
                    @keydown.enter="selectSegment('CDS', seg, 'CDS')" />
            </g>
            <g v-for="(seg, i) in fiveUtr()" :key="`utr5-${i}`">
              <rect :x="x(seg.start)" :y="geneY - 6" :width="width(seg.start, seg.end)" height="12" rx="2"
                    class="rt-tf-utr" tabindex="0"
                    @mouseenter="setHover($event, `5′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                    @focus="setHover($event, `5′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                    @click="selectSegment(`5′ UTR`, seg, `5′ UTR`)"
                    @keydown.enter="selectSegment(`5′ UTR`, seg, `5′ UTR`)" />
            </g>
            <g v-for="(seg, i) in threeUtr()" :key="`utr3-${i}`">
              <rect :x="x(seg.start)" :y="geneY - 6" :width="width(seg.start, seg.end)" height="12" rx="2"
                    class="rt-tf-utr" tabindex="0"
                    @mouseenter="setHover($event, `3′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                    @focus="setHover($event, `3′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                    @click="selectSegment(`3′ UTR`, seg, `3′ UTR`)"
                    @keydown.enter="selectSegment(`3′ UTR`, seg, `3′ UTR`)" />
            </g>
            <path :d="transcript.strand === '-' ? `M ${x(geneEnd)} ${geneY} l 8 -5 v 10 z` : `M ${x(geneEnd)} ${geneY} l -8 -5 v 10 z`" class="rt-tf-arrow" />
            <text :x="x(geneEnd) + (transcript.strand === '-' ? 12 : -14)" :y="geneY + 18" text-anchor="end" class="rt-tf-strand">{{ transcript.strand }}</text>

            <text x="8" :y="projectedY + 5" class="rt-tf-row-label">{{ text('投影 domain', 'Projected domains') }}</text>
            <line :x1="LEFT" :x2="LEFT + plotWidth" :y1="projectedY" :y2="projectedY" class="rt-tf-axis-light" />
            <template v-for="(domain, di) in projected" :key="`pd-${di}`">
              <rect v-for="(seg, si) in (domain.genomic ?? [])" :key="`pd-${di}-${si}`"
                    :x="x(seg.start)" :y="projectedY - 8" :width="width(seg.start, seg.end)" height="16" rx="2"
                    :fill="domainColor(domain)" opacity="0.82" tabindex="0"
                    @mouseenter="setHover($event, domainLabel(domain), domainLines(domain))"
                    @focus="setHover($event, domainLabel(domain), domainLines(domain))"
                    @click="selectDomain(domain)"
                    @keydown.enter="selectDomain(domain)" />
            </template>
            <text v-if="!projected.length" :x="LEFT" :y="projectedY + 4" class="rt-tf-empty">{{ text('没有可投影到 CDS 的 domain', 'No domains projected onto CDS') }}</text>

            <text x="8" :y="proteinY + 5" class="rt-tf-row-label">{{ text('蛋白 domain', 'Protein domains') }}</text>
            <line :x1="LEFT" :x2="LEFT + plotWidth" :y1="proteinY" :y2="proteinY" class="rt-tf-axis-light" />
            <g v-for="tick in proteinTicks()" :key="`p-${tick.x}`">
              <line :x1="tick.x" :x2="tick.x" :y1="proteinY + 10" :y2="proteinY + 15" class="rt-tf-tick" />
              <text :x="tick.x" :y="proteinY + 28" text-anchor="middle" class="rt-tf-ruler">{{ tick.label }}</text>
            </g>
            <g v-for="(domain, i) in domains" :key="`protein-domain-${i}`">
              <rect :x="proteinX(domain.protein_start - 1)" :y="proteinY - 9"
                    :width="Math.max(3, proteinX(domain.protein_end) - proteinX(domain.protein_start - 1))" height="18" rx="3"
                    :fill="domainColor(domain)" tabindex="0"
                    @mouseenter="setHover($event, domainLabel(domain), domainLines(domain))"
                    @focus="setHover($event, domainLabel(domain), domainLines(domain))"
                    @click="selectDomain(domain)"
                    @keydown.enter="selectDomain(domain)" />
              <text v-if="proteinX(domain.protein_end) - proteinX(domain.protein_start) > 55"
                    :x="(proteinX(domain.protein_start - 1) + proteinX(domain.protein_end)) / 2" :y="proteinY + 4"
                    text-anchor="middle" class="rt-tf-domain-label">{{ domainLabel(domain) }}</text>
            </g>
            <text v-if="!domains.length" :x="LEFT" :y="proteinY + 4" class="rt-tf-empty">{{ text('没有 HMMER domain 记录', 'No HMMER domain records') }}</text>

            <template v-if="hasPromoter && promoter">
              <text x="8" :y="promoterY + 5" class="rt-tf-row-label">{{ text('靶基因 motif', 'Target motifs') }}</text>
              <line :x1="LEFT" :x2="LEFT + plotWidth" :y1="promoterY" :y2="promoterY" class="rt-tf-axis-light" />
              <text :x="LEFT" :y="promoterY + 28" class="rt-tf-ruler">1</text>
              <text :x="LEFT + plotWidth" :y="promoterY + 28" text-anchor="end" class="rt-tf-ruler">{{ formatCoord(promoter.length) }} bp</text>
              <rect v-for="(site, i) in promoterSites" :key="`prom-${i}`"
                    :x="site.x" :y="site.y" :width="site.width" height="7" rx="2"
                    :class="['rt-tf-site', site.hit.strand === '-' ? 'rt-tf-site-minus' : 'rt-tf-site-plus', { 'rt-tf-site-selected': isSelected(site.hit) }]"
                    tabindex="0"
                    @mouseenter="setHover($event, site.hit.motif_id, [`${site.hit.motif_id} ${site.hit.motif_name ?? ''}`, `${text('位置', 'position')} ${site.hit.start}-${site.hit.end} (${site.hit.strand})`, `p=${site.hit.p_value?.toExponential(2) ?? '—'}`])"
                    @focus="setHover($event, site.hit.motif_id, [`${site.hit.motif_id} ${site.hit.motif_name ?? ''}`, `${text('位置', 'position')} ${site.hit.start}-${site.hit.end} (${site.hit.strand})`])"
                    @click="selectPromoterHit(site.hit)"
                    @keydown.enter="selectPromoterHit(site.hit)" />
            </template>

            <template v-if="hasTarget && view.target_gene">
              <text x="8" :y="targetTop - 8" class="rt-tf-row-label">{{ text('靶基因结构', 'Target gene') }}</text>
              <line :x1="LEFT" :x2="LEFT + plotWidth" :y1="targetTop" :y2="targetTop" class="rt-tf-axis" />
              <g v-for="tick in targetRuler" :key="`target-r-${tick.x}`">
                <line :x1="tick.x" :x2="tick.x" :y1="targetTop - 3" :y2="targetTop + 3" class="rt-tf-tick" />
                <text :x="tick.x" :y="targetTop - 7" text-anchor="middle" class="rt-tf-ruler">{{ tick.label }}</text>
              </g>
              <line :x1="targetX(targetStart)" :x2="targetX(targetEnd)" :y1="targetGeneY" :y2="targetGeneY" class="rt-tf-gene-line" />
              <g v-for="(intron, i) in targetIntrons()" :key="`target-intron-${i}`">
                <line :x1="targetX(intron.start)" :x2="targetX(intron.end)" :y1="targetGeneY" :y2="targetGeneY" class="rt-tf-intron" />
                <path :d="`M ${(targetX(intron.start) + targetX(intron.end)) / 2 - 4} ${targetGeneY - 3} l 4 6 l 4 -6`" class="rt-tf-chevron" />
              </g>
              <g v-for="(exon, i) in targetExonSegments()" :key="`target-exon-${i}`">
                <rect :x="targetX(exon.start)" :y="targetGeneY - 9" :width="targetWidth(exon.start, exon.end)" height="18" rx="2"
                      class="rt-tf-exon" tabindex="0"
                      @mouseenter="setHover($event, text('target exon', 'target exon'), [`${formatCoord(exon.start)}-${formatCoord(exon.end)}`])"
                      @focus="setHover($event, text('target exon', 'target exon'), [`${formatCoord(exon.start)}-${formatCoord(exon.end)}`])"
                      @click="selectSegment('target exon', exon, text('target exon', 'target exon'))"
                      @keydown.enter="selectSegment('target exon', exon, text('target exon', 'target exon'))" />
              </g>
              <g v-for="(seg, i) in targetCDSSegments()" :key="`target-cds-${i}`">
                <rect :x="targetX(seg.start)" :y="targetGeneY - 15" :width="targetWidth(seg.start, seg.end)" height="30" rx="2"
                      class="rt-tf-cds" tabindex="0"
                      @mouseenter="setHover($event, 'target CDS', [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                      @focus="setHover($event, 'target CDS', [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                      @click="selectSegment('target CDS', seg, 'target CDS')"
                      @keydown.enter="selectSegment('target CDS', seg, 'target CDS')" />
              </g>
              <g v-for="(seg, i) in targetFiveUTR()" :key="`target-utr5-${i}`">
                <rect :x="targetX(seg.start)" :y="targetGeneY - 6" :width="targetWidth(seg.start, seg.end)" height="12" rx="2"
                      class="rt-tf-utr" tabindex="0"
                      @mouseenter="setHover($event, `target 5′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                      @focus="setHover($event, `target 5′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                      @click="selectSegment(`target 5′ UTR`, seg, `target 5′ UTR`)"
                      @keydown.enter="selectSegment(`target 5′ UTR`, seg, `target 5′ UTR`)" />
              </g>
              <g v-for="(seg, i) in targetThreeUTR()" :key="`target-utr3-${i}`">
                <rect :x="targetX(seg.start)" :y="targetGeneY - 6" :width="targetWidth(seg.start, seg.end)" height="12" rx="2"
                      class="rt-tf-utr" tabindex="0"
                      @mouseenter="setHover($event, `target 3′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                      @focus="setHover($event, `target 3′ UTR`, [`${formatCoord(seg.start)}-${formatCoord(seg.end)}`])"
                      @click="selectSegment(`target 3′ UTR`, seg, `target 3′ UTR`)"
                      @keydown.enter="selectSegment(`target 3′ UTR`, seg, `target 3′ UTR`)" />
              </g>
              <path :d="targetTranscript?.strand === '-' ? `M ${targetX(targetEnd)} ${targetGeneY} l 8 -5 v 10 z` : `M ${targetX(targetEnd)} ${targetGeneY} l -8 -5 v 10 z`" class="rt-tf-arrow" />
              <text :x="LEFT" :y="targetGeneY + 26" class="rt-tf-strand">
                {{ view.target_gene.target_name }} · {{ view.target_gene.target_id }} · {{ targetTranscript?.id }}
              </text>
            </template>
          </svg>
        </div>
        <div v-if="hover" class="rt-tf-tooltip" :style="{ left: hover.x + 'px', top: hover.y + 'px' }">
          <div class="fw-semibold small">{{ hover.title }}</div>
          <div v-for="(line, i) in hover.lines" :key="i" class="small text-muted">{{ line }}</div>
        </div>
      </div>

      <div class="d-flex flex-wrap align-items-center gap-2 mt-1 small text-muted">
        <span><span class="rt-tf-swatch rt-tf-swatch-exon"></span>{{ text('exon', 'exon') }}</span>
        <span><span class="rt-tf-swatch rt-tf-swatch-cds"></span>CDS</span>
        <span><span class="rt-tf-swatch rt-tf-swatch-utr"></span>UTR</span>
        <span><span class="rt-tf-swatch rt-tf-swatch-domain"></span>{{ text('domain', 'domain') }}</span>
        <span class="ms-auto">{{ transcript?.id }} · {{ transcript?.strand }} · {{ text('外显子', 'exons') }} {{ exonSegments().length }}</span>
      </div>

      <div v-if="promoter && !hasTarget" class="small text-muted mt-1">
        {{ text('这个 target 没有可用的 GFF 转录本结构；仅显示 promoter motif。', 'No GFF transcript model is available for this target; only promoter motifs are shown.') }}
      </div>

      <div v-if="transcript?.projection_note" class="small text-warning mt-1">{{ transcript?.projection_note }}</div>

      <div v-if="selectedFeature" class="rt-tf-detail mt-2">
        <span class="badge text-bg-secondary me-1">{{ selectedFeature.kind }}</span>
        <span class="fw-semibold">{{ selectedFeature.label }}</span>
        <span class="text-muted ms-2">
          {{ selectedFeature.domain ? `${selectedFeature.start}-${selectedFeature.end} aa` : `${formatCoord(selectedFeature.start)}-${formatCoord(selectedFeature.end)}` }}
        </span>
        <div v-if="selectedDomain()" class="small mt-1">
          {{ selectedDomain()?.model_accession || selectedDomain()?.model_name || '—' }}
          · E={{ formatEvalue(selectedDomain()?.full_evalue ?? selectedDomain()?.conditional_evalue) }}
          · score={{ selectedDomain()?.domain_score?.toFixed(1) ?? '—' }}
          <span v-if="selectedDomain()?.genomic?.length" class="text-muted">
            · {{ text('投影', 'projected') }} {{ selectedDomain()?.genomic?.map((s) => `${s.start}-${s.end}`).join(', ') }}
          </span>
        </div>
        <div v-if="selectedFeature.note" class="small text-muted mt-1">{{ selectedFeature.note }}</div>
      </div>

      <div v-if="mappings.length" class="mt-2">
        <div class="small text-muted mb-1">{{ text('TF → motif 映射', 'TF → motif mappings') }}</div>
        <div class="d-flex flex-wrap gap-1">
          <button v-for="m in mappings" :key="`${m.motif_id}-${m.reference_tf_id}`" class="btn btn-sm btn-outline-secondary py-0"
                  @click="emit('focus-motif', m.motif_id)">
            {{ m.motif_id }}<span v-if="m.motif_name" class="text-muted"> · {{ m.motif_name }}</span>
          </button>
        </div>
      </div>
      <div v-else class="small text-muted mt-2">
        {{ text('这个 TF 没有可用的 motif 映射（motif-only 运行或未映射）。', 'No motif mapping for this TF (motif-only run or unmapped).') }}
      </div>
      <div v-if="promoter" class="small text-muted mt-1">
        {{ text('靶基因 promoter 上的 motif 位点见下方；点击图中的位点可查看序列和原始 evidence。', 'Motif sites on the target promoter are also shown below; click a site in either figure to inspect sequence and raw evidence.') }}
      </div>
    </template>
  </div>
</template>

<style scoped>
.rt-tf-evidence {
  border: 1px solid #e2e8f0;
  border-radius: 0.65rem;
  padding: 0.85rem;
  background: #fff;
}
.rt-tf-canvas {
  position: relative;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  background: #fbfdff;
}
.rt-tf-scroll {
  overflow-x: auto;
  overflow-y: hidden;
}
.rt-tf-axis,
.rt-tf-axis-light {
  stroke: #cbd5e1;
  stroke-width: 1;
}
.rt-tf-axis-light { stroke: #e2e8f0; }
.rt-tf-tick { stroke: #94a3b8; stroke-width: 1; }
.rt-tf-ruler { fill: #64748b; font-size: 10px; }
.rt-tf-row-label { fill: #475569; font-size: 11px; font-weight: 600; }
.rt-tf-gene-line,
.rt-tf-intron { stroke: #94a3b8; stroke-width: 1.4; }
.rt-tf-chevron { fill: none; stroke: #94a3b8; stroke-width: 1; }
.rt-tf-exon { fill: #99f6e4; stroke: #0f766e; stroke-width: 1; cursor: pointer; }
.rt-tf-cds { fill: #0f766e; stroke: #115e59; stroke-width: 1; cursor: pointer; }
.rt-tf-utr { fill: #fcd34d; stroke: #b45309; stroke-width: 0.8; cursor: pointer; }
.rt-tf-arrow { fill: #475569; }
.rt-tf-strand { fill: #64748b; font-size: 10px; }
.rt-tf-empty { fill: #94a3b8; font-size: 10px; }
.rt-tf-domain-label { fill: #fff; font-size: 9px; pointer-events: none; }
.rt-tf-site { fill: #64748b; cursor: pointer; }
.rt-tf-site-plus { fill: #0f766e; }
.rt-tf-site-minus { fill: #b45309; }
.rt-tf-site-selected { stroke: #111827; stroke-width: 1.5; }
.rt-tf-swatch { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 4px; vertical-align: -1px; }
.rt-tf-swatch-exon { background: #99f6e4; border: 1px solid #0f766e; }
.rt-tf-swatch-cds { background: #0f766e; }
.rt-tf-swatch-utr { background: #fcd34d; border: 1px solid #b45309; }
.rt-tf-swatch-domain { background: #c2410c; }
.rt-tf-tooltip {
  position: absolute;
  z-index: 5;
  max-width: 320px;
  padding: 0.4rem 0.55rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.4rem;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 6px 18px rgba(15, 23, 42, 0.12);
  pointer-events: none;
}
.rt-tf-detail {
  padding: 0.45rem 0.6rem;
  background: #f8fafc;
  border-radius: 0.4rem;
}
</style>

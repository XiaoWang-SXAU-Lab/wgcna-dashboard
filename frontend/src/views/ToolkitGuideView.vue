<script setup lang="ts">
import { ref } from 'vue'
import { locale } from '../utils/i18n'
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
type GuidePanel = 'workflow' | 'eggnog' | 'blast' | 'regutrace'
const activePanel = ref<GuidePanel>('workflow')

const panels: { id: GuidePanel; icon: string; zh: string; en: string }[] = [
  { id: 'workflow', icon: 'bi-diagram-3', zh: '注释工作流', en: 'Annotation workflow' },
  { id: 'eggnog', icon: 'bi-table', zh: 'eggNOG 转换', en: 'eggNOG converter' },
  { id: 'blast', icon: 'bi-search', zh: 'BLAST 合并', en: 'BLAST merge' },
  { id: 'regutrace', icon: 'bi-bezier2', zh: 'ReguTrace', en: 'ReguTrace' },
]

const steps = [
  { n: 1, icon: 'bi-file-earmark-text', title: ['准备蛋白序列', 'Prepare protein sequences'], color: '#0ea5e9' },
  { n: 2, icon: 'bi-cloud-arrow-up', title: ['跑 eggNOG-mapper', 'Run eggNOG-mapper'], color: '#8b5cf6' },
  { n: 3, icon: 'bi-search', title: ['（可选）BLAST 拿 symbol', 'Optional: BLAST for symbols'], color: '#f59e0b' },
  { n: 4, icon: 'bi-tools', title: ['工具箱转换', 'Convert in toolkit'], color: '#10b981' },
  { n: 5, icon: 'bi-shuffle', title: ['（可选）BLAST 合并', 'Optional: BLAST merge'], color: '#ec4899' },
  { n: 6, icon: 'bi-diagram-3', title: ['导入分析项目', 'Import & analyze'], color: '#6366f1' },
]
</script>

<template>
  <div class="guide mx-auto" style="max-width: 1120px">
    <div class="mb-4 text-center">
      <h3 class="mb-1">{{ text('工具箱详细指南', 'Toolkit detailed guide') }}</h3>
      <p class="text-muted mb-0">{{ text('按工具选择说明。注释整理和 TF–target 预测是不同任务，各自有独立输入、步骤和结果。', 'Choose a toolkit to see its inputs, workflow, and outputs. Gene annotation and TF–target prediction are separate tasks.') }}</p>
    </div>

    <nav class="guide-tabs mb-4" aria-label="Toolkit guide panels">
      <button v-for="panel in panels" :key="panel.id" type="button" class="guide-tab"
        :class="{ active: activePanel === panel.id }" :aria-pressed="activePanel === panel.id"
        @click="activePanel = panel.id">
        <i :class="['bi', panel.icon]"></i>{{ text(panel.zh, panel.en) }}
      </button>
    </nav>

    <!-- pipeline -->
    <div v-if="activePanel === 'workflow'" class="flow mb-4">
      <template v-for="(s, i) in steps" :key="s.n">
        <div class="flow-node">
          <div class="flow-icon" :style="{ background: s.color }"><i :class="['bi', s.icon]"></i></div>
          <div class="flow-label">{{ text(s.title[0], s.title[1]) }}</div>
        </div>
        <div v-if="i < steps.length - 1" class="flow-arrow"><i class="bi bi-arrow-right"></i></div>
      </template>
    </div>

    <!-- step 1 -->
    <section v-if="activePanel === 'workflow'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#0ea5e9">1</span><h5 class="mb-0">{{ text('准备蛋白序列', 'Prepare protein sequences') }}</h5></div>
      <p>{{ text('从基因组注释中提取每个基因的蛋白序列，保存为 FASTA（.faa / .fasta）。这是后续 eggNOG 和 BLAST 的输入。', 'Extract protein sequences per gene into a FASTA file; this is the input for eggNOG and BLAST.') }}</p>
      <pre class="code"><code>>gene1
MKV...
>gene2
MAA...</code></pre>
    </section>

    <!-- step 2 -->
    <section v-if="activePanel === 'workflow'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#8b5cf6">2</span><h5 class="mb-0">{{ text('跑 eggNOG-mapper（生成 GO / KEGG / 描述）', 'Run eggNOG-mapper (GO / KEGG / descriptions)') }}</h5></div>
      <p>{{ text('用在线版或服务器版，得到 .annotations 文件。关键参数：', 'Use the online or server version to get the .annotations file. Key parameters:') }}</p>
      <ul>
        <li><code>--itype proteins</code>{{ text('，输入蛋白 FASTA', ': protein FASTA input') }}</li>
        <li><code>--tax_scope Viridiplantae</code>{{ text('（植物；其它 kingdom 选对应分类）', ' (plants; pick the matching clade for other kingdoms)') }}</li>
        <li><code>--go_evidence both</code>{{ text('（否则 GO 太少）', ' (otherwise GO terms are too few)') }}</li>
        <li><code>--target_orthologs all</code>{{ text('（非模式物种不要用 one2one）', ' (avoid one2one for non-model species)') }}</li>
        <li><code>-e 1e-3</code>{{ text('、', ', ') }}<code>--seed_ortholog_evalue 1e-3</code></li>
      </ul>
      <p class="small text-muted mb-1">{{ text('在线版：', 'Online:') }} <a href="http://eggnog-mapper.embl.de/" target="_blank" rel="noopener noreferrer">http://eggnog-mapper.embl.de/</a></p>
      <pre class="code"><code>emapper.py -i Tartary.faa --itype proteins -o tartary \
  --cpu 32 --tax_scope Viridiplantae --sensmode sensitive \
  -e 1e-3 --seed_ortholog_evalue 1e-3 --target_orthologs all \
  --go_evidence both --pfam_realign none --dbmem --override</code></pre>
      <p class="small">{{ text('下载/保留', 'Keep') }} <code>*.emapper.annotations</code>{{ text('（TSV）。', ' (TSV).') }}</p>
    </section>

    <!-- step 3 -->
    <section v-if="activePanel === 'blast'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#f59e0b">3</span><h5 class="mb-0">{{ text('（可选）BLAST 拿 Gene symbol / name', 'Optional: BLAST for gene symbols / names') }}</h5></div>
      <p>{{ text('eggNOG 的 Preferred_name 是"同源转移"的预测名。要更可靠的 symbol/gene_name，把蛋白 BLAST 到同 kingdom 的模式物种：', 'eggNOG Preferred_name is a transferred prediction. For more reliable symbols, BLAST to a model organism of the same kingdom:') }}</p>
      <ul>
        <li>{{ text('植物：拟南芥 TAIR / 水稻 RAP-DB', 'Plants: TAIR / RAP-DB') }}</li>
        <li>{{ text('动物：人 HGNC / 小鼠 MGI / 斑马鱼 ZFIN', 'Animals: HGNC / MGI / ZFIN') }}</li>
        <li>{{ text('真菌：SGD / PomBase；细菌：EcoCyc / MicroScope', 'Fungi: SGD / PomBase; Bacteria: EcoCyc / MicroScope') }}</li>
        <li>{{ text('跨物种通用：UniProt Swiss-Prot', 'Cross-species: UniProt Swiss-Prot') }}</li>
      </ul>
      <pre class="code"><code>diamond blastp -d tair.dmnd -q Tartary.faa -o tair_hits.tsv \
  --outfmt 6 -e 1e-5 --max-target-seqs 1 --threads 32</code></pre>
      <p class="small mb-0">{{ text('输出 outfmt 6（12 列：qseqid sseqid pident length mismatch gapopen qstart qend sstart send evalue bitscore）。', 'Output is outfmt 6 (12 columns).') }}</p>
    </section>

    <!-- step 4 -->
    <section v-if="activePanel === 'eggnog'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#10b981">4</span><h5 class="mb-0">{{ text('工具箱 → eggNOG 注释转换', 'Toolkit → eggNOG converter') }}</h5></div>
      <p>{{ text('导航栏「工具箱」→「eggNOG 注释转换」→ 上传 .annotations → Kingdom 选对应分类（植物选 Plants）→ 转换，得到三张表：', 'Toolkit → eggNOG converter → upload .annotations → choose kingdom → convert to get:') }}</p>
      <div class="row g-2 small">
        <div class="col-md-4"><div class="mini-card"><b>annotation.tsv</b><div class="text-muted">gene_id / gene_name / symbol / description</div></div></div>
        <div class="col-md-4"><div class="mini-card"><b>gene2go.tsv</b><div class="text-muted">gene_id / go_id</div></div></div>
        <div class="col-md-4"><div class="mini-card"><b>gene2kegg.tsv</b><div class="text-muted">gene_id / ko_id</div></div></div>
      </div>
    </section>

    <!-- step 5 -->
    <section v-if="activePanel === 'blast'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#ec4899">5</span><h5 class="mb-0">{{ text('（可选）工具箱 → BLAST 注释合并', 'Optional: Toolkit → BLAST merge') }}</h5></div>
      <p>{{ text('上传 BLAST 结果（outfmt 6）+ 参考注释表（id → symbol/gene_name/description），可再上传第 4 步的 annotation.tsv 作为基础表；合并后得到"官方名优先、eggNOG 补缺"的 annotation.tsv。', 'Upload the BLAST result + reference annotation (+ optional base annotation.tsv); get a merged annotation with official names preferred.') }}</p>
      <pre class="code"><code>{{ text('# 参考注释表示例（TSV）', '# reference annotation example (TSV)') }}
gene_id	symbol	gene_name	description
AT1G01010	NAC001	NAC domain containing protein 1	NAC transcription factor</code></pre>
    </section>

    <!-- step 6 -->
    <section v-if="activePanel === 'workflow'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#6366f1">6</span><h5 class="mb-0">{{ text('导入分析项目', 'Import into an analysis project') }}</h5></div>
      <p>{{ text('新建/打开项目 → 数据导入页 → 分别上传：', 'New/open a project → Data import → upload:') }}</p>
      <ul>
        <li><b>expression</b>{{ text('：表达矩阵', ': expression matrix') }}</li>
        <li><b>trait</b>{{ text('：表型（可选）', ': trait table (optional)') }}</li>
        <li><b>sample_info</b>{{ text('：样本信息（可选，用于着色/性状来源）', ': sample information (optional; used for coloring / trait source)') }}</li>
        <li><b>annotation</b>{{ text('：上一步的 annotation.tsv', ': the annotation.tsv from the previous step') }}</li>
        <li><b>gene2go</b> / <b>gene2kegg</b>{{ text('：第 4 步的 gene2go.tsv / gene2kegg.tsv', ': the gene2go.tsv / gene2kegg.tsv from step 4') }}</li>
      </ul>
      <p class="mb-2">{{ text('之后即可按 过滤 → 软阈值 → 网络 → 模块-性状 → 模块解析（含 GO/KEGG 富集）跑完整流程。', 'Then run filter → soft threshold → network → module-trait → module explorer (with GO/KEGG enrichment).') }}</p>
      <p class="mb-0">{{ text('如需检查连接对样本变化的敏感程度，可展开模块网络下方的「边稳定性（可选）」，查看已选连接在有效重采样中超过 TOM 阈值的比例。', 'To check how sensitive selected edges are to changes in the samples, expand Edge stability (optional) below the module network. It shows how often each edge exceeds the TOM threshold in valid resamples.') }}</p>
    </section>

    <!-- ReguTrace workflow and architecture -->
    <section v-if="activePanel === 'regutrace'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#0f766e">RT</span><h5 class="mb-0">ReguTrace technical workflow</h5></div>
      <p>{{ text('植物流程使用基因组、GFF、CDS 或蛋白 FASTA 和注释表。先鉴定本地 TF，再用同家族双向最佳蛋白命中转移 JASPAR 代表矩阵，最后扫描启动子。', 'The plant workflow takes a genome, GFF, CDS or protein FASTA, and annotation. It identifies local TFs, transfers representative JASPAR matrices using same-family reciprocal protein best hits, and scans promoters.') }}</p>
      <p class="small text-muted">{{ text('Pack 同时带有两类资源：一类用于从蛋白结构域找 TF，另一类用于把 TF 映射到参考 motif。鉴定出来的 Gene ID 来自用户自己的序列/GFF，不是从拟南芥或其他 donor 名字复制来的。', 'A pack carries two distinct resource groups: TF domain models/rules and reference TF-to-motif evidence. The identified Gene ID comes from the user’s own sequence/GFF; it is not copied from an Arabidopsis or other donor name.') }}</p>
      <div class="roadmap-wrap" role="img" aria-label="ReguTrace technical roadmap from genome inputs through TF identification and motif mapping to promoter evidence">
        <svg class="roadmap" viewBox="0 0 1200 470" xmlns="http://www.w3.org/2000/svg">
          <defs>
            <marker id="rt-arrow" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto"><path d="M0,0 L0,6 L9,3 z" fill="#64748b"/></marker>
            <marker id="rt-arrow-teal" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto"><path d="M0,0 L0,6 L9,3 z" fill="#0f766e"/></marker>
          </defs>
          <g fill="none" stroke="#64748b" stroke-width="3" marker-end="url(#rt-arrow)">
            <path d="M220 150 H260"/><path d="M470 150 H510"/><path d="M720 150 H760"/><path d="M970 150 H1010"/>
            <path d="M120 220 V265 H865 V220"/>
          </g>
          <g fill="none" stroke="#0f766e" stroke-width="3" marker-end="url(#rt-arrow-teal)">
            <path d="M430 310 V220"/><path d="M610 310 V220"/><path d="M850 310 V220"/>
          </g>
          <g font-family="Arial, sans-serif" text-anchor="middle">
            <rect x="20" y="90" width="200" height="130" rx="16" fill="#eff6ff" stroke="#60a5fa" stroke-width="2"/>
            <text x="120" y="123" font-size="18" font-weight="700" fill="#1e3a8a">USER INPUTS</text>
            <text x="120" y="154" font-size="15" fill="#334155">Protein / CDS FASTA</text>
            <text x="120" y="180" font-size="15" fill="#334155">Genome FASTA + GFF3</text>
            <text x="120" y="204" font-size="12" fill="#64748b">Gene annotations (TSV / eggNOG)</text>

            <rect x="260" y="90" width="210" height="130" rx="16" fill="#f0fdf4" stroke="#4ade80" stroke-width="2"/>
            <text x="365" y="123" font-size="18" font-weight="700" fill="#166534">TF IDENTIFICATION</text>
            <text x="365" y="154" font-size="15" fill="#334155">HMM domain search</text>
            <text x="365" y="180" font-size="15" fill="#334155">Taxon-specific family rules</text>
            <text x="365" y="204" font-size="12" fill="#64748b">Output: local TF Gene IDs</text>

            <rect x="510" y="90" width="210" height="130" rx="16" fill="#f5f3ff" stroke="#a78bfa" stroke-width="2"/>
            <text x="615" y="123" font-size="18" font-weight="700" fill="#5b21b6">TF → MOTIF MAP</text>
            <text x="615" y="154" font-size="15" fill="#334155">Same-family protein RBH</text>
            <text x="615" y="180" font-size="15" fill="#334155">Transfer representative PFM</text>
            <text x="615" y="204" font-size="12" fill="#64748b">Keep equally supported candidates</text>

            <rect x="760" y="90" width="210" height="130" rx="16" fill="#fffbeb" stroke="#fbbf24" stroke-width="2"/>
            <text x="865" y="123" font-size="18" font-weight="700" fill="#92400e">PROMOTER SCAN</text>
            <text x="865" y="154" font-size="15" fill="#334155">Extract promoters from GFF3</text>
            <text x="865" y="180" font-size="15" fill="#334155">Scan transferred JASPAR motifs</text>
            <text x="865" y="204" font-size="12" fill="#64748b">Report candidate sites</text>

            <rect x="1010" y="90" width="170" height="130" rx="16" fill="#ecfeff" stroke="#22d3ee" stroke-width="2"/>
            <text x="1095" y="123" font-size="18" font-weight="700" fill="#155e75">OUTPUTS</text>
            <text x="1095" y="154" font-size="14" fill="#334155">TF catalogue</text>
            <text x="1095" y="178" font-size="14" fill="#334155">Trace + evidence</text>
            <text x="1095" y="202" font-size="12" fill="#64748b">TSV + SQLite</text>

            <rect x="390" y="310" width="500" height="110" rx="16" fill="#f0fdfa" stroke="#2dd4bf" stroke-width="2"/>
            <text x="640" y="345" font-size="18" font-weight="700" fill="#115e59">VERSIONED REFERENCE PACK</text>
            <text x="640" y="375" font-size="15" fill="#334155">TF HMMs + family rules · reference TF proteins · TF–motif map · PFM library</text>
            <text x="640" y="401" font-size="12" fill="#64748b">A pack supplies reference evidence; it does not replace the user’s genome or gene IDs.</text>
          </g>
          <text x="600" y="455" text-anchor="middle" font-family="Arial, sans-serif" font-size="13" fill="#475569">A motif hit is a computational candidate (evidence level D), not proof of in-vivo binding or regulation.</text>
        </svg>
      </div>
      <div class="row g-2 small mt-2">
        <div class="col-md-6"><div class="mini-card"><b>{{ text('TF 鉴定', 'TF identification') }}</b><div class="text-muted">{{ text('蛋白结构域 HMM + 类群家族规则；这一步不需要 promoter motif。', 'Protein-domain HMMs plus taxon-specific family rules; promoter motifs are not used at this stage.') }}</div></div></div>
        <div class="col-md-6"><div class="mini-card"><b>{{ text('靶标候选', 'Target candidates') }}</b><div class="text-muted">{{ text('只有成功映射到 motif 的 TF 才能扫描启动子。映射缺失或有并列时会如实报告。', 'Only TFs mapped to motifs can scan promoters. Missing mappings and ties are reported explicitly.') }}</div></div></div>
      </div>
      <div class="alert alert-info small mt-3 mb-0">
        {{ text('ReguTrace 页面默认使用 reference pack；手动扫描收在“高级”里，不会自动鉴定 TF gene，结果标记为 motif-only，不能直接作为 WGCNA prior。当前 Windows beta 内置植物、酵母、线虫和果蝇参考 pack。物种在同一 kingdom 不代表 TF/motif 覆盖相同；pack 的规则和 motif 覆盖范围请按具体 pack 查看。没有参考 pack 的类群，手动 motif-only scan 不会自动鉴定 TF。', 'The ReguTrace page defaults to reference-pack mode. Manual scan is under “Advanced”, does not identify TF genes by itself, is marked motif-only, and is not a WGCNA prior. The current Windows beta bundles plant, yeast, nematode, and Drosophila packs. Sharing a kingdom does not guarantee the same TF or motif coverage. Check the selected pack’s scope. Without a suitable pack, a manual motif-only scan does not identify TFs.') }}
      </div>
    </section>

    <!-- ReguTrace motif library -->
    <section v-if="activePanel === 'regutrace'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#0f766e">M</span><h5 class="mb-0">{{ text('ReguTrace motif 库从哪里来？', 'Where do ReguTrace motif libraries come from?') }}</h5></div>
      <p>{{ text('参考 pack 模式会按物种提供 TF 鉴定规则、参考 TF–motif 映射和 motif 矩阵；当前 Windows beta 内置植物、酵母、线虫和果蝇 pack。手动扫描模式接受 JASPAR PFM（>ID name + A/C/G/T 矩阵），并内置多个类群的 motif 库；其它类群可从下表寻找矩阵来源。', 'Reference-pack mode provides taxon-specific TF rules, reference TF–motif mappings, and motif matrices; the current Windows beta bundles plant, yeast, nematode, and Drosophila packs. Manual scan mode accepts JASPAR PFM text (>ID name + A/C/G/T rows) and includes motif libraries for several taxa; use the sources below for other taxa.') }}</p>
      <div class="table-responsive">
        <table class="table table-sm align-middle small mb-2">
          <thead><tr><th>{{ text('类群', 'Taxon') }}</th><th>{{ text('推荐来源', 'Recommended sources') }}</th><th>{{ text('说明', 'Notes') }}</th></tr></thead>
          <tbody>
            <tr><td>{{ text('通用真核', 'Eukaryotes') }}</td><td><a href="https://jaspar.elixir.no/downloads/" target="_blank" rel="noopener noreferrer">JASPAR downloads</a> · <a href="https://cisbp.ccbr.utoronto.ca/" target="_blank" rel="noopener noreferrer">CIS-BP</a></td><td>{{ text('JASPAR 可按 taxon 下载 JASPAR 格式；CIS-BP 多为 MEME/TRANSFAC，需要转换。', 'JASPAR offers per-taxon JASPAR-format downloads; CIS-BP is often MEME/TRANSFAC and needs conversion.') }}</td></tr>
            <tr><td>{{ text('植物', 'Plants') }}</td><td>JASPAR plants · CIS-BP plants · PlantTFDB</td><td>{{ text('内置库即 JASPAR plants；PlantTFDB/PlantRegMap 站点目前可能不可用。', 'The built-in library is JASPAR plants; PlantTFDB/PlantRegMap availability varies.') }}</td></tr>
            <tr><td>{{ text('动物', 'Animals') }}</td><td>JASPAR vertebrates / insects / nematodes / urochordates · <a href="https://hocomoco14.autosome.org/downloads_v14" target="_blank" rel="noopener noreferrer">HOCOMOCO</a> · <a href="http://uniprobe.org/" target="_blank" rel="noopener noreferrer">UniPROBE</a></td><td>{{ text('HOCOMOCO 覆盖人/小鼠，提供 JASPAR/MEME/TRANSFAC 等格式。', 'HOCOMOCO covers human/mouse and provides JASPAR/MEME/TRANSFAC formats.') }}</td></tr>
            <tr><td>{{ text('真菌', 'Fungi') }}</td><td>JASPAR fungi · <a href="http://yetfasco.ccbr.utoronto.ca/" target="_blank" rel="noopener noreferrer">YeTFaSCo</a> · CIS-BP fungi</td><td>{{ text('YeTFaSCo 主要覆盖 S. cerevisiae；其它真菌优先 JASPAR/CIS-BP。', 'YeTFaSCo focuses on S. cerevisiae; use JASPAR/CIS-BP for other fungi.') }}</td></tr>
            <tr><td>{{ text('细菌', 'Bacteria') }}</td><td><a href="https://regulondb.ccg.unam.mx/" target="_blank" rel="noopener noreferrer">RegulonDB</a> · <a href="https://www.prodoric.de/" target="_blank" rel="noopener noreferrer">PRODORIC</a> · <a href="http://www.collectf.org/" target="_blank" rel="noopener noreferrer">CollecTF</a></td><td>{{ text('多为结合位点/矩阵或比对结果，需要整理成 PFM；RegulonDB 以 E. coli 为主。', 'Often binding sites/alignments rather than PFMs; RegulonDB focuses on E. coli.') }}</td></tr>
            <tr><td>{{ text('古菌', 'Archaea') }}</td><td>PRODORIC · <a href="http://bioinformatics.zj.cn/archaeatf/" target="_blank" rel="noopener noreferrer">ArchaeaTF</a> · <a href="https://entraf.iimas.unam.mx/" target="_blank" rel="noopener noreferrer">ENTRAF</a></td><td>{{ text('没有像 JASPAR 那样统一的古菌 motif 库，覆盖较稀疏。', 'No unified JASPAR-like archaeal motif collection; coverage is sparse.') }}</td></tr>
            <tr><td>{{ text('病毒 / 原生生物', 'Viruses / protists') }}</td><td>{{ text('宿主 JASPAR/CIS-BP；原生生物另查 VEuPathDB/PlasmoDB', 'Host JASPAR/CIS-BP; for protists also VEuPathDB/PlasmoDB') }}</td><td>{{ text('没有统一的病毒 TF motif 库；病毒调控位点通常用宿主 TF 矩阵近似。', 'No unified viral TF motif library; viral regulatory sites are usually approximated with host TF matrices.') }}</td></tr>
          </tbody>
        </table>
      </div>
      <p class="small text-muted mb-0">{{ text('引用：JASPAR 2024, Nucleic Acids Research, doi:10.1093/nar/gkad1059；CIS-BP, Weirauch et al., Cell 2014；HOCOMOCO 2024, doi:10.1093/nar/gkad1077。', 'Citations: JASPAR 2024, Nucleic Acids Research, doi:10.1093/nar/gkad1059; CIS-BP, Weirauch et al., Cell 2014; HOCOMOCO 2024, doi:10.1093/nar/gkad1077.') }}</p>
    </section>

    <!-- format reference -->
    <section v-if="activePanel !== 'regutrace'" class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#64748b">?</span><h5 class="mb-0">{{ text('输入/输出格式速查', 'Format cheat sheet') }}</h5></div>
      <div class="row g-3 small">
        <div class="col-md-6"><b>annotation.tsv</b><pre class="code"><code>gene_id	gene_name	symbol	description
AT1G01010	NAC001	NAC001	NAC domain...</code></pre></div>
        <div class="col-md-6"><b>gene2go.tsv</b><pre class="code"><code>gene_id	go_id
AT1G01010	GO:0006351</code></pre></div>
        <div class="col-md-6"><b>gene2kegg.tsv</b><pre class="code"><code>gene_id	ko_id
AT1G01010	K14400</code></pre></div>
        <div class="col-md-6"><b>BLAST outfmt 6</b><pre class="code"><code>qseqid	sseqid	pident	length	mismatch	gapopen	qstart	qend	sstart	send	evalue	bitscore
gene1	AT1G01010	88.5	400	...</code></pre></div>
      </div>
    </section>

    <div v-if="activePanel !== 'regutrace'" class="text-center text-muted small mt-4">{{ text('提示：所有文件都用 UTF-8 编码、制表符（Tab）分隔。', 'Tip: all files are UTF-8, tab-separated.') }}</div>
  </div>
</template>

<style scoped>
.guide { color: #223747; }
.guide-tabs { display: flex; flex-wrap: wrap; gap: 8px; justify-content: center; }
.guide-tab { border: 1px solid #cbd5e1; background: #fff; color: #334155; border-radius: 999px; padding: 8px 16px; font-size: 14px; }
.guide-tab i { margin-right: 7px; }
.guide-tab.active { background: #0f766e; border-color: #0f766e; color: #fff; box-shadow: 0 4px 12px rgba(15, 118, 110, .2); }
.roadmap-wrap { overflow-x: auto; border: 1px solid #dce5ec; background: #fff; border-radius: 12px; padding: 8px; }
.roadmap { display: block; width: 100%; min-width: 900px; height: auto; }
.flow { display: flex; align-items: center; justify-content: center; flex-wrap: wrap; gap: 6px; background: #fff; border: 1px solid #dce5ec; border-radius: 14px; padding: 18px 12px; }
.flow-node { display: flex; flex-direction: column; align-items: center; width: 108px; text-align: center; }
.flow-icon { width: 46px; height: 46px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #fff; font-size: 20px; box-shadow: 0 6px 14px rgba(0, 0, 0, 0.12); }
.flow-label { font-size: 12px; margin-top: 8px; line-height: 1.3; color: #33495c; }
.flow-arrow { color: #9fb3c3; font-size: 18px; }
.step-card { background: #fff; border: 1px solid #e2e9f0; border-radius: 14px; padding: 18px 20px; margin-bottom: 14px; box-shadow: 0 4px 14px rgba(15, 23, 42, 0.05); }
.step-head { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; }
.step-badge { width: 30px; height: 30px; border-radius: 50%; color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.code { background: #0f172a; color: #e2e8f0; border-radius: 8px; padding: 12px 14px; font-size: 12.5px; overflow-x: auto; margin: 10px 0; }
.mini-card { background: #f7fafc; border: 1px solid #e2e9f0; border-radius: 10px; padding: 10px 12px; }
</style>

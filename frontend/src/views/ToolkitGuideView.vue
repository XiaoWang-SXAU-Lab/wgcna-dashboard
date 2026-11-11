<script setup lang="ts">
import { locale } from '../utils/i18n'
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en

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
  <div class="guide mx-auto" style="max-width: 980px">
    <div class="mb-4 text-center">
      <h3 class="mb-1">{{ text('从零生成基因注释：完整指引', 'From scratch to gene annotation: full guide') }}</h3>
      <p class="text-muted mb-0">{{ text('适用于任意物种（以苦荞等非模式植物为例），最终产出 annotation.tsv / gene2go.tsv / gene2kegg.tsv 三张表。', 'Works for any species (e.g. non-model plants like tartary buckwheat); the final output is the three tables.') }}</p>
    </div>

    <!-- pipeline -->
    <div class="flow mb-4">
      <template v-for="(s, i) in steps" :key="s.n">
        <div class="flow-node">
          <div class="flow-icon" :style="{ background: s.color }"><i :class="['bi', s.icon]"></i></div>
          <div class="flow-label">{{ text(s.title[0], s.title[1]) }}</div>
        </div>
        <div v-if="i < steps.length - 1" class="flow-arrow"><i class="bi bi-arrow-right"></i></div>
      </template>
    </div>

    <!-- step 1 -->
    <section class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#0ea5e9">1</span><h5 class="mb-0">{{ text('准备蛋白序列', 'Prepare protein sequences') }}</h5></div>
      <p>{{ text('从基因组注释中提取每个基因的蛋白序列，保存为 FASTA（.faa / .fasta）。这是后续 eggNOG 和 BLAST 的输入。', 'Extract protein sequences per gene into a FASTA file; this is the input for eggNOG and BLAST.') }}</p>
      <pre class="code"><code>>gene1
MKV...
>gene2
MAA...</code></pre>
    </section>

    <!-- step 2 -->
    <section class="step-card">
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
    <section class="step-card">
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
    <section class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#10b981">4</span><h5 class="mb-0">{{ text('工具箱 → eggNOG 注释转换', 'Toolkit → eggNOG converter') }}</h5></div>
      <p>{{ text('导航栏「工具箱」→「eggNOG 注释转换」→ 上传 .annotations → Kingdom 选对应分类（植物选 Plants）→ 转换，得到三张表：', 'Toolkit → eggNOG converter → upload .annotations → choose kingdom → convert to get:') }}</p>
      <div class="row g-2 small">
        <div class="col-md-4"><div class="mini-card"><b>annotation.tsv</b><div class="text-muted">gene_id / gene_name / symbol / description</div></div></div>
        <div class="col-md-4"><div class="mini-card"><b>gene2go.tsv</b><div class="text-muted">gene_id / go_id</div></div></div>
        <div class="col-md-4"><div class="mini-card"><b>gene2kegg.tsv</b><div class="text-muted">gene_id / ko_id</div></div></div>
      </div>
    </section>

    <!-- step 5 -->
    <section class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#ec4899">5</span><h5 class="mb-0">{{ text('（可选）工具箱 → BLAST 注释合并', 'Optional: Toolkit → BLAST merge') }}</h5></div>
      <p>{{ text('上传 BLAST 结果（outfmt 6）+ 参考注释表（id → symbol/gene_name/description），可再上传第 4 步的 annotation.tsv 作为基础表；合并后得到"官方名优先、eggNOG 补缺"的 annotation.tsv。', 'Upload the BLAST result + reference annotation (+ optional base annotation.tsv); get a merged annotation with official names preferred.') }}</p>
      <pre class="code"><code>{{ text('# 参考注释表示例（TSV）', '# reference annotation example (TSV)') }}
gene_id	symbol	gene_name	description
AT1G01010	NAC001	NAC domain containing protein 1	NAC transcription factor</code></pre>
    </section>

    <!-- step 6 -->
    <section class="step-card">
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

    <!-- ReguTrace motif library -->
    <section class="step-card">
      <div class="step-head"><span class="step-badge" style="background:#0f766e">M</span><h5 class="mb-0">{{ text('ReguTrace motif 库从哪里来？', 'Where do ReguTrace motif libraries come from?') }}</h5></div>
      <p>{{ text('ReguTrace 只接受 JASPAR PFM 文本（>ID name + A/C/G/T 矩阵）。内置库是 JASPAR 2024 CORE plants non-redundant（805 个 motif，CC BY 4.0）；其它物种族需要从下面的来源下载并转换成 JASPAR PFM 后上传。', 'ReguTrace accepts JASPAR PFM text only (>ID name + A/C/G/T rows). The built-in library is JASPAR 2024 CORE plants non-redundant (805 motifs, CC BY 4.0); other taxa need to be downloaded from the sources below and converted to JASPAR PFM before upload.') }}</p>
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
    <section class="step-card">
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

    <div class="text-center text-muted small mt-4">{{ text('提示：所有文件都用 UTF-8 编码、制表符（Tab）分隔。', 'Tip: all files are UTF-8, tab-separated.') }}</div>
  </div>
</template>

<style scoped>
.guide { color: #223747; }
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

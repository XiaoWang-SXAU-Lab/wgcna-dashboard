<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useProjectStore } from '../stores/project'
import { locale } from '../utils/i18n'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const parent = ref(''), name = ref(''), openPath = ref('')
const actionBusy = ref(false)

async function act(fn: () => Promise<unknown>) {
  actionBusy.value = true; store.error = ''
  try { await fn() } catch (e) { store.error = String(e) } finally { actionBusy.value = false }
}
async function pick(target: 'parent' | 'open') {
  const res = await fetch('/api/folder-picker'); if (!res.ok) throw new Error(await res.text())
  const data = await res.json(); if (target === 'parent') parent.value = data.dir; else openPath.value = data.dir
}
onMounted(() => { void act(() => store.loadRecent()) })
</script>

<template>
  <div class="project-workspace">
    <div class="d-flex align-items-center justify-content-between gap-3 mb-3 flex-wrap">
      <div><h3 class="mb-1">{{ text('分析项目', 'Analysis projects') }}</h3><p class="text-muted mb-0">{{ text('保存研究问题、数据和分析进度，重新打开后继续。', 'Save your question, data and progress. Reopen a project to continue.') }}</p></div>
      <span class="badge text-bg-light border">{{ text('探索版', 'Experimental') }}</span>
    </div>
    <div v-if="store.error" role="alert" class="alert alert-danger">{{ store.error }}</div>
    <template v-if="!store.current">
      <div class="row g-3">
        <section class="col-lg-6"><div class="card h-100 p-4">
          <h5>{{ text('新建项目', 'New project') }}</h5>
          <label class="form-label mt-2" for="project-name">{{ text('项目名称', 'Project name') }}</label><input id="project-name" v-model="name" class="form-control" />
          <label class="form-label mt-3" for="project-parent">{{ text('保存到文件夹', 'Save in folder') }}</label>
          <div class="input-group"><input id="project-parent" v-model="parent" class="form-control" /><button class="btn btn-outline-secondary" @click="act(() => pick('parent'))">{{ text('选择', 'Browse') }}</button></div>
          <small class="text-muted mt-2">{{ text('将在此文件夹下创建以项目命名的子目录。', 'A new subfolder will be created with the project name.') }}</small>
          <button class="btn btn-primary mt-3" :disabled="actionBusy || !name.trim() || !parent.trim()" @click="act(() => store.create(parent, name))">{{ text('创建项目', 'Create project') }}</button>
        </div></section>
        <section class="col-lg-6"><div class="card h-100 p-4">
          <h5>{{ text('打开项目', 'Open project') }}</h5>
          <label class="form-label mt-2" for="project-open">{{ text('项目文件夹', 'Project folder') }}</label>
          <div class="input-group"><input id="project-open" v-model="openPath" class="form-control" /><button class="btn btn-outline-secondary" @click="act(() => pick('open'))">{{ text('选择', 'Browse') }}</button></div>
          <button class="btn btn-outline-primary mt-3" :disabled="actionBusy || !openPath.trim()" @click="act(() => store.open(openPath))">{{ text('打开并继续', 'Open and continue') }}</button>
          <h6 class="mt-4">{{ text('最近项目', 'Recent projects') }}</h6>
          <button v-for="item in store.recent" :key="item.path" class="btn btn-light text-start border mb-2" :disabled="actionBusy" @click="act(() => store.open(item.path))"><b>{{ item.name }}</b><small class="d-block text-muted text-break">{{ item.path }}</small></button>
          <small v-if="!store.recent.length" class="text-muted">{{ text('暂无项目。', 'No recent projects.') }}</small>
        </div></section>
      </div>
    </template>
    <template v-else>
      <section class="card p-3 mb-3">
        <div class="d-flex align-items-center flex-wrap gap-2"><h5 class="mb-0 me-auto">{{ store.current.state.name }}</h5>
          <span role="status" class="small" :class="store.dirty ? 'text-warning' : 'text-success'">{{ store.saving ? text('保存中…', 'Saving…') : store.dirty ? text('有未保存修改', 'Unsaved changes') : text('已保存', 'Saved') }}</span>
          <button class="btn btn-sm btn-outline-primary" :disabled="store.busy || actionBusy" @click="act(store.flush)">{{ text('保存', 'Save') }}</button>
          <button class="btn btn-sm btn-outline-secondary" :disabled="store.busy || actionBusy" @click="act(store.close)">{{ text('关闭项目', 'Close project') }}</button>
        </div>
        <small class="text-muted text-break mt-1">{{ store.current.path }}</small>
      </section>
      <details class="card p-3 mb-3"><summary class="fw-semibold">{{ text('研究问题卡', 'Research question') }}</summary>
        <fieldset :disabled="store.busy || actionBusy" class="row g-3 mt-1">
          <label class="col-12">{{ text('想解决什么生物学问题？', 'What biological question are you studying?') }}<textarea v-model="store.card.question" class="form-control mt-1" rows="2"></textarea></label>
          <label class="col-md-6">{{ text('假设', 'Hypothesis') }}<textarea v-model="store.card.hypothesis" class="form-control mt-1"></textarea></label>
          <label class="col-md-6">{{ text('物种与组织', 'Organism and tissue') }}<input v-model="store.card.organism" class="form-control mt-1" /></label>
          <label class="col-md-6">{{ text('实验因素、重复、配对与批次', 'Design, replicates, pairing and batches') }}<textarea v-model="store.card.design" class="form-control mt-1"></textarea></label>
          <label class="col-md-6">{{ text('已知限制与待确认信息', 'Limitations and open questions') }}<textarea v-model="store.card.limitations" class="form-control mt-1"></textarea></label>
        </fieldset>
      </details>
      <p class="text-muted small">{{ text('数据导入、软阈值、网络与模块等分析步骤请使用顶部导航进入对应页面。', 'Use the top navigation to enter each analysis step (data import, soft threshold, network, and more).') }}</p>
    </template>
  </div>
</template>

<style scoped>
fieldset { border: 0; min-width: 0; padding: 0; }
label { font-size: .875rem; }
summary { cursor: pointer; }
</style>

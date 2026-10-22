<script setup lang="ts">
import { computed, ref } from 'vue'
import { marked } from 'marked'
import { chatStream, getAIConfig, saveAIConfig } from '../api/client'
import { t } from '../utils/i18n'

const props = defineProps<{ errorCtx?: string }>()

function renderMarkdown(text: string): string {
  return marked.parse(text) as string
}

// AI 还在「思考」（流式开始但还没吐出第一个 token）时显示动画。
const isThinking = computed(() => {
  if (!streaming.value) return false
  const last = messages.value[messages.value.length - 1]
  return last?.role === 'assistant' && !last.content.trim()
})

const open = ref(false)
const showSettings = ref(false)
const messages = ref<{ role: 'user' | 'assistant'; content: string }[]>([])
const input = ref('')
const streaming = ref(false)
const configured = ref(false)

// settings
const provider = ref('deepseek')
const apiKey = ref('')

const PROVIDERS = [
  { id: 'deepseek', name: 'DeepSeek' },
  { id: 'openai', name: 'OpenAI' },
  { id: 'anthropic', name: 'Anthropic' },
  { id: 'ollama', name: 'Ollama (本地)' },
  { id: 'custom', name: '自定义' },
]

async function loadConfig() {
  try {
    const c = await getAIConfig()
    provider.value = c.provider || 'deepseek'
    configured.value = c.configured
  } catch {
    /* ignore */
  }
}

async function saveKey() {
  await saveAIConfig({ provider: provider.value, api_key: apiKey.value })
  apiKey.value = ''
  await loadConfig()
  showSettings.value = false
}

async function send() {
  const q = input.value.trim()
  input.value = ''
  await sendQuestion(q, props.errorCtx ?? '')
}

async function sendQuestion(q: string, errCtx: string) {
  if (!q.trim() || streaming.value) return
  messages.value.push({ role: 'user', content: q })
  streaming.value = true
  const reply = { role: 'assistant' as const, content: '' }
  messages.value.push(reply)
  try {
    await chatStream(q, errCtx, (d) => {
      reply.content += d
    })
  } catch (e) {
    reply.content = e instanceof Error ? e.message : String(e)
  } finally {
    streaming.value = false
  }
}

function openPanel() {
  open.value = !open.value
  if (open.value) loadConfig()
}

// 供外部（错误诊断按钮）调用：打开面板并带错误上下文提问。
function analyzeError(err: string) {
  open.value = true
  loadConfig()
  sendQuestion(t('ai.promptError') + '\n\n' + err, err)
}

defineExpose({ analyzeError })
</script>

<template>
  <button class="ai-tab" @click="openPanel" :title="open ? t('ai.collapse') : t('ai.title')">
    <i :class="['bi', open ? 'bi-chevron-right' : 'bi-robot']"></i>
  </button>

  <div v-if="open" class="chat">
    <header class="d-flex justify-content-between align-items-center px-3 py-2 border-bottom bg-white">
      <span class="fw-semibold">{{ t('ai.title') }}</span>
      <span class="d-flex gap-1">
        <button class="btn btn-sm btn-outline-secondary" @click="showSettings = !showSettings"><i class="bi bi-gear"></i></button>
        <button class="btn btn-sm btn-outline-secondary" @click="open = false"><i class="bi bi-x-lg"></i></button>
      </span>
    </header>

    <div v-if="showSettings" class="px-3 py-2 border-bottom bg-light">
      <label class="form-label small mb-1">{{ t('ai.provider') }}</label>
      <select class="form-select form-select-sm mb-2" v-model="provider">
        <option v-for="p in PROVIDERS" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
      <label class="form-label small mb-1">{{ t('ai.apiKey') }}</label>
      <input class="form-control form-control-sm mb-2" v-model="apiKey" type="password" placeholder="sk-..." />
      <div class="d-flex align-items-center gap-2">
        <button class="btn btn-sm btn-primary" :disabled="!apiKey" @click="saveKey">{{ t('ai.save') }}</button>
        <span class="text-muted small">{{ configured ? t('ai.configured') + ' ✓' : t('ai.notConfigured') }}</span>
      </div>
    </div>

    <div class="messages d-flex flex-column flex-grow-1 overflow-auto px-3 py-2">
      <div v-if="messages.length === 0" class="text-muted small text-center mt-4">{{ t('ai.empty') }}</div>
      <div v-for="(m, i) in messages" :key="i" :class="['msg', m.role]">
        <div v-if="m.role === 'assistant'" class="md-body" v-html="renderMarkdown(m.content)"></div>
        <template v-else>{{ m.content }}</template>
      </div>
      <div v-if="isThinking" class="thinking align-self-start text-muted small">
        <span class="spinner-border spinner-border-sm me-2"></span>{{ t('ai.thinking') }}
      </div>
    </div>

    <footer class="d-flex gap-2 px-2 py-2 border-top">
      <input
        class="form-control form-control-sm"
        v-model="input"
        :placeholder="streaming ? t('ai.replying') : t('ai.placeholder')"
        @keyup.enter="send"
      />
      <button class="btn btn-sm btn-primary" :disabled="!input.trim() || streaming" @click="send">{{ t('ai.send') }}</button>
    </footer>
  </div>
</template>

<style scoped>
.ai-tab {
  position: fixed;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  z-index: 1041;
  width: 24px;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #dee2e6;
  border-right: none;
  border-radius: 10px 0 0 10px;
  background: #fff;
  color: #3182bd;
  cursor: pointer;
  box-shadow: -2px 0 6px rgba(0, 0, 0, 0.08);
}
.chat {
  position: fixed;
  right: 0;
  top: 3.6rem;
  bottom: 0;
  width: 400px;
  border: 1px solid #dee2e6;
  border-right: none;
  border-radius: 12px 0 0 12px;
  background: #fff;
  display: flex;
  flex-direction: column;
  box-shadow: -8px 0 30px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  z-index: 1040;
}
.msg {
  max-width: 85%;
  padding: 0.45rem 0.65rem;
  border-radius: 10px;
  font-size: 0.875rem;
  white-space: pre-wrap;
  word-break: break-word;
}
.msg.user {
  align-self: flex-end;
  background: #3182bd;
  color: #fff;
  margin-bottom: 0.4rem;
}
.msg.assistant {
  align-self: flex-start;
  background: #f1f5f9;
  margin-bottom: 0.4rem;
}
.thinking {
  padding: 0.45rem 0.65rem;
  margin-bottom: 0.4rem;
}
/* markdown 富文本（v-html 注入，需 :deep 穿透 scoped）*/
.md-body :deep(p) { margin: 0.25rem 0; }
.md-body :deep(pre) { background: #f6f8fa; padding: 0.5rem; border-radius: 6px; overflow-x: auto; font-size: 0.8rem; }
.md-body :deep(code) { background: #eef1f4; padding: 0.1rem 0.3rem; border-radius: 4px; font-size: 0.8rem; }
.md-body :deep(pre code) { background: none; padding: 0; }
.md-body :deep(ul), .md-body :deep(ol) { margin: 0.25rem 0; padding-left: 1.2rem; }
.md-body :deep(h1), .md-body :deep(h2), .md-body :deep(h3), .md-body :deep(h4) { font-size: 0.95rem; font-weight: 600; margin: 0.5rem 0 0.25rem; }
.md-body :deep(strong) { font-weight: 600; }
.md-body :deep(a) { color: #3182bd; }
.md-body :deep(table) { border-collapse: collapse; margin: 0.25rem 0; }
.md-body :deep(th), .md-body :deep(td) { border: 1px solid #dee2e6; padding: 0.2rem 0.4rem; font-size: 0.8rem; }
</style>

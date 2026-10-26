<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
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

// --- 技能（skill）：在输入框输入 @ 或 / 触发下拉选择 ---
interface Skill { id: string; prompt: string }
const SKILLS: Skill[] = [
  {
    id: 'literature',
    prompt: '【任务：文献检索与解读】请结合领域文献背景解读用户的分析结果：1) 概括用户当前关注的结果（模块/通路/hub 基因）；2) 给出相关生物学背景（涉及的通路、基因功能、已知研究）；3) 解释结果与用户生物学问题之间的联系，并给出进一步验证建议。',
  },
  {
    id: 'interpret',
    prompt: '【任务：结果解读】请解读用户当前的分析结果（模块、富集通路、hub 基因、网络结构），说明其生物学意义。',
  },
  {
    id: 'background',
    prompt: '【任务：背景介绍】请介绍与用户当前结果相关的生物学背景（通路、机制、研究现状）。',
  },
  {
    id: 'troubleshoot',
    prompt: '【任务：报错诊断】请诊断用户遇到的报错，定位原因并给出可操作的修改建议。',
  },
]

const skillTrigger = computed(() => {
  const m = input.value.match(/(?:^|\s)[@/]([\w-]*)$/)
  return m ? m[1].toLowerCase() : null
})

const filteredSkills = computed(() => {
  const q = skillTrigger.value
  if (q === null) return []
  return SKILLS.filter((s) => s.id.includes(q) || t('ai.skill.' + s.id).toLowerCase().includes(q))
})

function pickSkill(s: Skill) {
  input.value = input.value.replace(/[@/][\w-]*$/, '@' + s.id + ' ')
}

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
  if (!q || streaming.value) return
  input.value = ''
  // 解析技能标签：@skillName … → 前置技能提示词
  let question = q
  const m = q.match(/^@([\w-]+)(?:\s+|$)/)
  if (m) {
    const skill = SKILLS.find((s) => s.id === m[1])
    if (skill) {
      const rest = q.slice(m[0].length).trim()
      question = skill.prompt + (rest ? '\n\n用户补充：' + rest : '')
    }
  }
  await sendQuestion(question, props.errorCtx ?? '')
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

// Enter = 换行；Cmd/Ctrl+Enter = 发送。
function onEnter(e: KeyboardEvent) {
  if (e.metaKey || e.ctrlKey) {
    e.preventDefault()
    send()
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

// --- 折叠时随机弹气泡提醒 ---
const BUBBLES = ['ai.bubble.hello', 'ai.bubble.interpret', 'ai.bubble.skill', 'ai.bubble.tip']
const bubble = ref('')
let bubbleTimer: ReturnType<typeof setInterval> | undefined
let bubbleHideTimer: ReturnType<typeof setTimeout> | undefined

function popBubble() {
  if (open.value || streaming.value) return
  bubble.value = BUBBLES[Math.floor(Math.random() * BUBBLES.length)]
  if (bubbleHideTimer) clearTimeout(bubbleHideTimer)
  bubbleHideTimer = setTimeout(() => { bubble.value = '' }, 3800)
}

onMounted(() => {
  bubbleTimer = setInterval(popBubble, 11000)
  setTimeout(popBubble, 3000)
})

onBeforeUnmount(() => {
  if (bubbleTimer) clearInterval(bubbleTimer)
  if (bubbleHideTimer) clearTimeout(bubbleHideTimer)
})

defineExpose({ analyzeError })
</script>

<template>
  <button class="ai-tab" @click="openPanel" :title="open ? t('ai.collapse') : t('ai.title')">
    <span class="ai-mascot" :class="open ? 'sitting' : 'lying'">🐱</span>
  </button>

  <div v-if="!open && bubble" class="ai-bubble">{{ t(bubble) }}</div>

  <div v-if="open" class="chat">
    <header class="d-flex justify-content-between align-items-center px-3 py-2 border-bottom bg-white">
      <span class="fw-semibold d-flex align-items-center gap-2"><span class="ai-mascot sitting">🐱</span>{{ t('ai.title') }}</span>
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

    <footer class="px-2 py-2 border-top position-relative">
      <div v-if="filteredSkills.length" class="skill-menu">
        <button v-for="s in filteredSkills" :key="s.id" class="skill-item" type="button" @mousedown.prevent="pickSkill(s)">
          <span class="fw-semibold"><i class="bi bi-stars me-1"></i>{{ t('ai.skill.' + s.id) }}</span>
          <span class="text-muted small">{{ t('ai.skill.' + s.id + 'Desc') }}</span>
        </button>
      </div>
      <div class="d-flex gap-2 align-items-start">
        <textarea
          class="form-control form-control-sm"
          v-model="input"
          rows="2"
          :placeholder="streaming ? t('ai.replying') : t('ai.placeholder')"
          @keydown.enter="onEnter"
        ></textarea>
        <button class="btn btn-sm btn-primary" :disabled="!input.trim() || streaming" @click="send">{{ t('ai.send') }}</button>
      </div>
      <div class="text-muted small mt-1 px-1">{{ t('ai.enterHint') }}</div>
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
  width: 26px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #dee2e6;
  border-right: none;
  border-radius: 10px 0 0 10px;
  background: #fff;
  cursor: pointer;
  box-shadow: -2px 0 6px rgba(0, 0, 0, 0.08);
  overflow: hidden;
}
.ai-mascot {
  font-size: 1.25rem;
  line-height: 1;
  display: inline-block;
}
.ai-mascot.lying {
  animation: ai-lying 3.4s ease-in-out infinite;
}
.ai-mascot.sitting {
  animation: ai-sitting 3.4s ease-in-out infinite;
}
@keyframes ai-lying {
  0%, 100% { transform: rotate(90deg) scale(1); }
  50% { transform: rotate(90deg) scale(1.14); }
}
@keyframes ai-sitting {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.08); }
}
.ai-bubble {
  position: fixed;
  right: 36px;
  top: 50%;
  transform: translateY(-50%);
  background: #fff;
  border: 1px solid #dee2e6;
  border-radius: 12px;
  padding: 0.45rem 0.75rem;
  font-size: 0.8rem;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.14);
  z-index: 1042;
  white-space: nowrap;
  animation: ai-bubble-in 0.25s ease;
}
@keyframes ai-bubble-in {
  from { opacity: 0; transform: translateY(-50%) translateX(10px); }
  to { opacity: 1; transform: translateY(-50%) translateX(0); }
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
.skill-menu {
  position: absolute;
  bottom: 100%;
  left: 0.5rem;
  right: 0.5rem;
  background: #fff;
  border: 1px solid #dee2e6;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
  max-height: 240px;
  overflow-y: auto;
  z-index: 10;
}
.skill-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  width: 100%;
  padding: 0.4rem 0.6rem;
  border: none;
  background: none;
  text-align: left;
  cursor: pointer;
}
.skill-item:hover {
  background: #f1f5f9;
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

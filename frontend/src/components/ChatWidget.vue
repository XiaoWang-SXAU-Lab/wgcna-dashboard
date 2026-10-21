<script setup lang="ts">
import { ref } from 'vue'
import { chatStream, getAIConfig, saveAIConfig } from '../api/client'

const props = defineProps<{ errorCtx?: string }>()

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
  sendQuestion(`请分析下面这个报错，告诉我哪里出错了、怎么改：\n\n${err}`, err)
}

defineExpose({ analyzeError })
</script>

<template>
  <button class="fab" @click="openPanel">🤖</button>

  <div v-if="open" class="chat">
    <header class="chat-header">
      <span>AI 领航员</span>
      <span class="actions">
        <button class="ghost" @click="showSettings = !showSettings">⚙️</button>
        <button class="ghost" @click="open = false">✕</button>
      </span>
    </header>

    <div v-if="showSettings" class="settings">
      <label>
        厂商
        <select v-model="provider">
          <option v-for="p in PROVIDERS" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </label>
      <label>
        API Key
        <input v-model="apiKey" type="password" placeholder="sk-..." />
      </label>
      <button :disabled="!apiKey" @click="saveKey">保存（首次输入后固化）</button>
      <p class="hint">{{ configured ? '已配置 ✓' : '未配置' }}</p>
    </div>

    <div class="messages">
      <div v-if="messages.length === 0" class="empty">
        问我 WGCNA 原理、操作步骤、参数含义，或让我分析报错。
      </div>
      <div v-for="(m, i) in messages" :key="i" :class="['msg', m.role]">{{ m.content }}</div>
    </div>

    <footer class="chat-input">
      <input
        v-model="input"
        :placeholder="streaming ? '回复中…' : '输入问题…'"
        @keyup.enter="send"
      />
      <button :disabled="!input.trim() || streaming" @click="send">发送</button>
    </footer>
  </div>
</template>

<style scoped>
.fab {
  position: fixed;
  right: 1.2rem;
  bottom: 1.2rem;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  font-size: 1.5rem;
  border: none;
  background: #3182bd;
  color: #fff;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}
.chat {
  position: fixed;
  right: 1.2rem;
  bottom: 5rem;
  width: 380px;
  height: 520px;
  border: 1px solid #e3e3e3;
  border-radius: 12px;
  background: #fff;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.15);
  overflow: hidden;
}
.chat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.6rem 0.8rem;
  background: #f7fafc;
  font-weight: 600;
}
.actions .ghost {
  border: none;
  background: none;
  cursor: pointer;
  font-size: 1rem;
}
.settings {
  padding: 0.6rem 0.8rem;
  border-bottom: 1px solid #eee;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  font-size: 0.85rem;
}
.settings label {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  align-items: center;
}
.settings input,
.settings select {
  flex: 1;
  padding: 0.25rem;
}
.hint {
  color: #888;
  font-size: 0.75rem;
  margin: 0;
}
.messages {
  flex: 1;
  overflow-y: auto;
  padding: 0.8rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.empty {
  color: #aaa;
  font-size: 0.85rem;
  text-align: center;
  margin-top: 2rem;
}
.msg {
  max-width: 85%;
  padding: 0.5rem 0.7rem;
  border-radius: 10px;
  font-size: 0.9rem;
  white-space: pre-wrap;
  word-break: break-word;
}
.msg.user {
  align-self: flex-end;
  background: #3182bd;
  color: #fff;
}
.msg.assistant {
  align-self: flex-start;
  background: #f1f5f9;
}
.chat-input {
  display: flex;
  gap: 0.4rem;
  padding: 0.6rem;
  border-top: 1px solid #eee;
}
.chat-input input {
  flex: 1;
  padding: 0.4rem;
}
.chat-input button {
  padding: 0.4rem 0.8rem;
}
</style>

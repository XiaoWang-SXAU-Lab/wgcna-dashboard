<script setup lang="ts">
import { ref } from 'vue'

// 默认展开；sidebar 与结果页同处一个 flex 行，折叠/展开时结果页自适应伸缩。
const collapsed = ref(false)
</script>

<template>
  <div class="sd-wrap d-flex" style="min-height: calc(100vh - 3.6rem)">
    <aside
      class="sd-aside flex-shrink-0 overflow-hidden"
      :style="{ width: collapsed ? '0px' : '320px', transition: 'width 0.3s cubic-bezier(0.4, 0, 0.2, 1)' }"
    >
      <div class="sd-card">
        <slot name="sidebar" />
      </div>
    </aside>

    <div class="sd-main flex-grow-1 position-relative" style="min-width: 0">
      <button class="sd-tab" @click="collapsed = !collapsed" :title="collapsed ? '展开参数' : '折叠参数'">
        <i :class="['bi', collapsed ? 'bi-chevron-right' : 'bi-chevron-left']"></i>
      </button>
      <div class="sd-inner">
        <slot />
      </div>
    </div>
  </div>
</template>

<style scoped>
.sd-card {
  height: 100%;
  width: 320px;
  background: #fff;
  border-right: 1px solid #dee2e6;
  padding: 0.75rem;
  overflow-y: auto;
}
.sd-tab {
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  z-index: 20;
  width: 18px;
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #dee2e6;
  border-left: none;
  border-radius: 0 10px 10px 0;
  background: #fff;
  color: #008080;
  cursor: pointer;
  box-shadow: 2px 0 6px rgba(0, 0, 0, 0.08);
}
.sd-inner {
  height: 100%;
  padding-left: 26px;
}
</style>

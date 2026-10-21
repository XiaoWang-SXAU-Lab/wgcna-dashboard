<script setup lang="ts">
import { ref } from 'vue'

const collapsed = ref(false)
</script>

<template>
  <div class="d-flex align-items-start">
    <aside
      class="overflow-hidden flex-shrink-0"
      :style="{ width: collapsed ? '0px' : '300px', transition: 'width .3s cubic-bezier(0.4, 0, 0.2, 1)' }"
    >
      <div class="bg-white border rounded p-2" :style="{ width: '300px' }">
        <slot name="sidebar" />
      </div>
    </aside>

    <div class="flex-grow-1 position-relative" style="min-width: 0">
      <button class="sd-tab" @click="collapsed = !collapsed" :title="collapsed ? '展开' : '折叠'">
        <i :class="['bi', collapsed ? 'bi-chevron-right' : 'bi-chevron-left']"></i>
      </button>
      <div class="sd-main">
        <slot />
      </div>
    </div>
  </div>
</template>

<style scoped>
.sd-tab {
  position: absolute; left: 0; top: 8px; z-index: 20;
  width: 18px; height: 44px;
  display: flex; align-items: center; justify-content: center;
  border: 1px solid #dee2e6; border-left: none;
  border-radius: 0 9px 9px 0;
  background: #fff; color: #008080; cursor: pointer;
  box-shadow: 1px 0 4px rgba(0,0,0,0.08);
}
.sd-main { padding-left: 26px; }
</style>

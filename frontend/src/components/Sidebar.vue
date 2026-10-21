<script setup lang="ts">
</script>

<template>
  <aside class="sidebar d-flex flex-column p-3">
    <div class="brand mb-4 d-flex align-items-center gap-2">
      <i class="bi bi-hexagon-fill fs-2 text-primary"></i>
      <div>
        <div class="brand-title fw-bold fs-5">WGCNA Dashboard</div>
        <div class="brand-sub small">加权基因共表达网络分析</div>
      </div>
    </div>

    <ul class="nav flex-column gap-1">
      <li class="nav-item">
        <router-link class="nav-link" to="/">
          <i class="bi bi-house-door me-2"></i>主页
        </router-link>
      </li>
      <li class="nav-item">
        <router-link class="nav-link" to="/analysis">
          <i class="bi bi-graph-up-arrow me-2"></i>分析工作台
        </router-link>
      </li>
    </ul>

    <div class="sidebar-foot mt-auto small d-flex align-items-center gap-2">
      <i class="bi bi-robot"></i>
      <span>AI 领航员 · 多厂商 LLM</span>
    </div>
  </aside>
</template>

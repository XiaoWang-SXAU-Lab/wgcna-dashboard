<script setup lang="ts">
import { ref } from 'vue'
import OneStepPanel from '../components/OneStepPanel.vue'
import StepByStepPanel from '../components/StepByStepPanel.vue'

const oneStep = ref(true)
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-items-center justify-content-between gap-2 mb-3">
      <h4 class="mb-0 fw-semibold">分析工作台</h4>
      <div class="form-check form-switch fs-6 mb-0">
        <input class="form-check-input" type="checkbox" role="switch" id="modeSwitch" v-model="oneStep" />
        <label class="form-check-label" for="modeSwitch">
          <i :class="['bi', oneStep ? 'bi-lightning-charge-fill' : 'bi-list-steps', 'me-1']"></i>
          {{ oneStep ? 'One-step 一键分析' : 'Step-by-step 分步分析' }}
        </label>
      </div>
    </div>

    <OneStepPanel v-if="oneStep" />
    <StepByStepPanel v-else />
  </div>
</template>

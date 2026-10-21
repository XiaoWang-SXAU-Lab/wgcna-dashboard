<script setup lang="ts">
import OneStepPanel from '../components/OneStepPanel.vue'
import { t } from '../utils/i18n'
</script>

<template>
  <div>
    <h4 class="mb-3">{{ t('os.title') }}</h4>
    <OneStepPanel />
  </div>
</template>

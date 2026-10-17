<script setup lang="ts">
defineProps<{ title: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <div class="overlay" @mousedown.self="emit('close')">
    <div class="modal" role="dialog" aria-modal="true">
      <div class="modal-head">
        <h3>{{ title }}</h3>
        <button class="btn ghost" aria-label="Zapri" @click="emit('close')">✕</button>
      </div>
      <div class="modal-body"><slot /></div>
      <div class="modal-foot"><slot name="footer" /></div>
    </div>
  </div>
</template>

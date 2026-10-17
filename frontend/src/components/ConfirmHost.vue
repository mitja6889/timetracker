<script setup lang="ts">
import { storeToRefs } from 'pinia'
import AppModal from './AppModal.vue'
import { useConfirmStore } from '../stores/confirm'

const store = useConfirmStore()
const { open, options } = storeToRefs(store)

function cancel() {
  store.answer(false)
}

function confirm() {
  store.answer(true)
}
</script>

<template>
  <AppModal
    v-if="open && options"
    :title="options.title ?? 'Potrditev'"
    @close="cancel"
  >
    <p class="confirm-message">{{ options.message }}</p>
    <template #footer>
      <button type="button" class="btn ghost" @click="cancel">
        {{ options.cancelLabel ?? 'Prekliči' }}
      </button>
      <button
        type="button"
        class="btn"
        :class="options.variant === 'danger' ? 'danger' : 'primary'"
        @click="confirm"
      >
        {{ options.confirmLabel ?? 'Potrdi' }}
      </button>
    </template>
  </AppModal>
</template>

<style scoped>
.confirm-message {
  margin: 0;
  line-height: 1.5;
  white-space: pre-wrap;
}
</style>

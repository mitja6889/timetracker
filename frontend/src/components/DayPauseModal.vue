<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api, ApiError } from '../api'
import AppModal from './AppModal.vue'
import { fromLocalInput } from '../utils'

const props = defineProps<{ dayKey: string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const form = reactive({
  startAt: `${props.dayKey}T12:00`,
  endAt: `${props.dayKey}T13:00`,
  note: '',
})
const error = ref('')
const busy = ref(false)

async function save() {
  error.value = ''
  busy.value = true
  try {
    await api.createDayPause(props.dayKey, {
      startAt: fromLocalInput(form.startAt),
      endAt: fromLocalInput(form.endAt),
      note: form.note.trim() || undefined,
    })
    emit('saved')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Shranjevanje ni uspelo'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppModal :title="`Pavza — ${dayKey}`" @close="emit('close')">
    <form id="pause-form" class="form" @submit.prevent="save">
      <label>
        Začetek pavze
        <input v-model="form.startAt" type="datetime-local" required />
      </label>
      <label>
        Konec pavze
        <input v-model="form.endAt" type="datetime-local" required />
      </label>
      <label>
        Opomba (neobvezno)
        <input v-model="form.note" maxlength="200" />
      </label>
      <p class="muted small">Pavza se odšteje od delovnega časa izbranega dne.</p>
      <p v-if="error" class="error">{{ error }}</p>
    </form>
    <template #footer>
      <button type="button" class="btn ghost" @click="emit('close')">Prekliči</button>
      <button type="submit" form="pause-form" class="btn primary" :disabled="busy">Shrani</button>
    </template>
  </AppModal>
</template>

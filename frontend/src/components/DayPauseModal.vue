<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api, ApiError } from '../api'
import AppModal from './AppModal.vue'

const props = defineProps<{ dayKey: string }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const form = reactive({
  minutes: 15,
  note: '',
})
const error = ref('')
const busy = ref(false)

async function save() {
  error.value = ''
  const minutes = Math.floor(Number(form.minutes))
  if (!Number.isFinite(minutes) || minutes < 1) {
    error.value = 'Vnesite vsaj 1 minuto'
    return
  }
  busy.value = true
  try {
    await api.createDayPause(props.dayKey, {
      minutes,
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
        Trajanje pavze (minute)
        <input
          v-model.number="form.minutes"
          type="number"
          min="1"
          max="1440"
          step="1"
          inputmode="numeric"
          required
        />
      </label>
      <label>
        Opomba (neobvezno)
        <input v-model="form.note" maxlength="200" />
      </label>
      <p class="muted small">To število minut se odšteje od delovnega časa izbranega dne.</p>
      <p v-if="error" class="error">{{ error }}</p>
    </form>
    <template #footer>
      <button type="button" class="btn ghost" @click="emit('close')">Prekliči</button>
      <button type="submit" form="pause-form" class="btn primary" :disabled="busy">Shrani</button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api, ApiError, type UserRow } from '../api'
import { useAuth } from '../stores/auth'
import AppModal from './AppModal.vue'

const props = defineProps<{
  users: UserRow[]
  defaultDate?: string
}>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const auth = useAuth()
const canPickUser = computed(() => auth.has('entries.edit_all'))

const pad2 = (n: number) => String(n).padStart(2, '0')
const todayLocal = () => {
  const d = new Date()
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}
const nowTimeLocal = () => {
  const d = new Date()
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

const form = reactive({
  userId: '',
  type: 'start' as 'start' | 'stop',
  date: props.defaultDate ?? todayLocal(),
  time: nowTimeLocal(),
})
const error = ref('')
const busy = ref(false)
const nextHint = ref<'start' | 'stop' | null>(null)

async function refreshHint() {
  try {
    const h = await api.timeEventHint(canPickUser.value && form.userId ? form.userId : undefined)
    nextHint.value = h.nextType
    form.type = h.nextType
  } catch {
    nextHint.value = null
  }
}

watch(
  () => form.userId,
  () => {
    void refreshHint()
  },
)

void refreshHint()

async function save() {
  error.value = ''
  busy.value = true
  try {
    await api.createTimeEvent({
      userId: canPickUser.value && form.userId ? form.userId : undefined,
      type: form.type,
      date: form.date,
      time: form.time,
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
  <AppModal title="Ročni dogodek" @close="emit('close')">
    <form id="manual-event-form" class="form" @submit.prevent="save">
      <label v-if="canPickUser">
        Uporabnik
        <select v-model="form.userId">
          <option value="">Jaz ({{ auth.me?.name }})</option>
          <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }} ({{ u.username }})</option>
        </select>
      </label>
      <label>
        Tip dogodka
        <select v-model="form.type">
          <option value="start">Start</option>
          <option value="stop">Stop</option>
        </select>
      </label>
      <p v-if="nextHint" class="muted small">
        Po zaporedju dogodkov je pričakovan naslednji tip:
        <strong>{{ nextHint === 'start' ? 'Start' : 'Stop' }}</strong>.
      </p>
      <div class="grid2">
        <label>
          Datum
          <input v-model="form.date" type="date" required />
        </label>
        <label>
          Čas
          <input v-model="form.time" type="time" required />
        </label>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
    </form>
    <template #footer>
      <button type="button" class="btn ghost" @click="emit('close')">Prekliči</button>
      <button type="submit" form="manual-event-form" class="btn primary" :disabled="busy">Shrani</button>
    </template>
  </AppModal>
</template>

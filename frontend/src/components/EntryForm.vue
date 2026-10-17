<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api, ApiError, type Entry, type EntryPayload, type UserRow } from '../api'
import { fromLocalInput, toLocalInput } from '../utils'
import AppModal from './AppModal.vue'

const props = defineProps<{
  entry: Entry | null
  users: UserRow[]
  canPickUser: boolean
}>()
const emit = defineEmits<{ (e: 'saved'): void; (e: 'close'): void }>()

const form = reactive({
  userId: '',
  description: props.entry?.description ?? '',
  startedAt: toLocalInput(props.entry?.startedAt ?? null),
  endedAt: toLocalInput(props.entry?.endedAt ?? null),
  pauses: (props.entry?.pauses ?? []).map((p) => ({
    start: toLocalInput(p.start),
    end: toLocalInput(p.end),
  })),
})
const error = ref('')
const busy = ref(false)

function addPause() {
  form.pauses.push({ start: form.startedAt, end: form.startedAt })
}

async function save() {
  error.value = ''
  if (!form.startedAt || !form.endedAt) {
    error.value = 'Začetek in konec sta obvezna'
    return
  }
  for (const p of form.pauses) {
    if (!p.start || !p.end) {
      error.value = 'Vse pavze morajo imeti začetek in konec'
      return
    }
  }
  const payload: EntryPayload = {
    description: form.description,
    startedAt: fromLocalInput(form.startedAt),
    endedAt: fromLocalInput(form.endedAt),
    pauses: form.pauses.map((p) => ({ start: fromLocalInput(p.start), end: fromLocalInput(p.end) })),
  }
  if (!props.entry && form.userId) payload.userId = form.userId

  busy.value = true
  try {
    if (props.entry) await api.updateEntry(props.entry.id, payload)
    else await api.createEntry(payload)
    emit('saved')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka pri shranjevanju'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppModal :title="entry ? 'Uredi vnos' : 'Nov vnos'" @close="emit('close')">
    <form id="entry-form" class="form" @submit.prevent="save">
      <label v-if="!entry && canPickUser">
        Uporabnik
        <select v-model="form.userId">
          <option value="">Jaz</option>
          <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }} ({{ u.username }})</option>
        </select>
      </label>
      <label>
        Opis
        <input v-model="form.description" maxlength="200" />
      </label>
      <div class="grid2">
        <label>
          Začetek
          <input v-model="form.startedAt" type="datetime-local" required />
        </label>
        <label>
          Konec
          <input v-model="form.endedAt" type="datetime-local" required />
        </label>
      </div>

      <div class="pauses">
        <div class="row between">
          <strong>Pavze</strong>
          <button type="button" class="btn ghost" @click="addPause">+ Dodaj pavzo</button>
        </div>
        <div v-for="(p, i) in form.pauses" :key="i" class="grid2 pause-row">
          <input v-model="p.start" type="datetime-local" required />
          <div class="row">
            <input v-model="p.end" type="datetime-local" required />
            <button type="button" class="btn ghost" aria-label="Odstrani pavzo" @click="form.pauses.splice(i, 1)">✕</button>
          </div>
        </div>
        <p v-if="!form.pauses.length" class="muted small">Brez pavz.</p>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
    </form>
    <template #footer>
      <button class="btn ghost" type="button" @click="emit('close')">Prekliči</button>
      <button class="btn primary" form="entry-form" :disabled="busy">Shrani</button>
    </template>
  </AppModal>
</template>

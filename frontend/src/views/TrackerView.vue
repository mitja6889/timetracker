<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, ApiError, type Entry, type TimerState } from '../api'
import { useAuth } from '../stores/auth'
import { calcWorkSeconds, formatDateTime, formatDuration } from '../utils'

const auth = useAuth()

const entry = ref<Entry | null>(null)
const description = ref('')
const recent = ref<Entry[]>([])
const error = ref('')
const busy = ref(false)

// razlika med strežniško in odjemalčevo uro, da števec kaže pravilno tudi ob zamaknjeni uri
const offsetMs = ref(0)
const tick = ref(Date.now())
let timer: number | undefined

const elapsed = computed(() => (entry.value ? calcWorkSeconds(entry.value, tick.value + offsetMs.value) : 0))
const status = computed(() => entry.value?.status ?? 'idle')

function apply(state: TimerState) {
  entry.value = state.entry
  offsetMs.value = new Date(state.now).getTime() - Date.now()
  if (state.entry) description.value = state.entry.description
}

async function loadRecent() {
  if (!auth.me) return
  const page = await api.entries({
    page: 1,
    pageSize: 5,
    sort: 'startedAt',
    order: 'desc',
    userId: auth.me.id,
    status: 'stopped',
  })
  recent.value = page.items
}

async function run(fn: () => Promise<TimerState>, reload = false) {
  error.value = ''
  busy.value = true
  try {
    apply(await fn())
    if (reload) {
      description.value = ''
      await loadRecent()
    }
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka'
    // stanje se je morda spremenilo v drugem zavihku
    try {
      apply(await api.timerCurrent())
    } catch {
      /* ignore */
    }
  } finally {
    busy.value = false
  }
}

const start = () => run(() => api.timerStart(description.value))
const pause = () => run(() => api.timerPause())
const resume = () => run(() => api.timerResume())
const stop = () => run(() => api.timerStop(), true)

onMounted(async () => {
  timer = window.setInterval(() => (tick.value = Date.now()), 1000)
  try {
    apply(await api.timerCurrent())
    await loadRecent()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka pri nalaganju'
  }
})
onUnmounted(() => window.clearInterval(timer))
</script>

<template>
  <div class="card tracker" :class="status">
    <div class="clock">{{ formatDuration(elapsed) }}</div>
    <div class="state-label">
      <span v-if="status === 'idle'">Časovnik miruje</span>
      <span v-else-if="status === 'running'">● Teče</span>
      <span v-else>⏸ Pavza</span>
    </div>

    <input
      v-model="description"
      class="desc"
      maxlength="200"
      placeholder="Na čem delate? (neobvezno)"
      :disabled="status !== 'idle' || busy"
      @keyup.enter="status === 'idle' && start()"
    />

    <div class="buttons">
      <button v-if="status === 'idle'" class="btn primary big" :disabled="busy" @click="start">▶ Start</button>
      <template v-else>
        <button v-if="status === 'running'" class="btn warn big" :disabled="busy" @click="pause">⏸ Pavza</button>
        <button v-else class="btn primary big" :disabled="busy" @click="resume">▶ Nadaljuj</button>
        <button class="btn danger big" :disabled="busy" @click="stop">■ Stop</button>
      </template>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
  </div>

  <div class="card">
    <div class="row between wrap page-head">
      <h3>Zadnji vnosi</h3>
      <RouterLink to="/entries">Vsi vnosi →</RouterLink>
    </div>
    <div v-if="recent.length" class="table-wrap">
      <table class="table table-stack">
        <thead>
          <tr><th>Opis</th><th>Začetek</th><th>Konec</th><th class="num">Trajanje</th></tr>
        </thead>
        <tbody>
          <tr v-for="e in recent" :key="e.id">
            <td data-label="Opis">{{ e.description || '—' }}</td>
            <td data-label="Začetek">{{ formatDateTime(e.startedAt) }}</td>
            <td data-label="Konec">{{ formatDateTime(e.endedAt) }}</td>
            <td class="num" data-label="Trajanje">{{ formatDuration(e.workSeconds) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else class="muted">Še ni zaključenih vnosov.</p>
  </div>
</template>

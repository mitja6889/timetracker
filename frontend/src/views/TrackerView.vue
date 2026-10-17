<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, ApiError, type TimerState } from '../api'
import { formatDuration } from '../utils'

const state = ref<TimerState | null>(null)
const error = ref('')
const busy = ref(false)

const offsetMs = ref(0)
const tick = ref(Date.now())
let timer: number | undefined

const running = computed(() => state.value?.running ?? false)
const elapsed = computed(() => {
  if (!state.value?.running || !state.value.runningSince) return 0
  const start = new Date(state.value.runningSince).getTime()
  const now = tick.value + offsetMs.value
  return Math.max(0, Math.floor((now - start) / 1000))
})

function apply(s: TimerState) {
  state.value = s
  offsetMs.value = new Date(s.now).getTime() - Date.now()
}

async function run(fn: () => Promise<TimerState>) {
  error.value = ''
  busy.value = true
  try {
    apply(await fn())
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka'
    try {
      apply(await api.timerCurrent())
    } catch {
      /* ignore */
    }
  } finally {
    busy.value = false
  }
}

const start = () => run(() => api.timerStart())
const stop = () => run(() => api.timerStop())

onMounted(async () => {
  timer = window.setInterval(() => (tick.value = Date.now()), 1000)
  try {
    apply(await api.timerCurrent())
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka pri nalaganju'
  }
})
onUnmounted(() => window.clearInterval(timer))
</script>

<template>
  <div class="card tracker" :class="running ? 'running' : 'idle'">
    <div class="clock">{{ formatDuration(elapsed) }}</div>
    <div class="state-label">
      <span v-if="!running">Časovnik miruje — Start zabeleži dogodek v bazo</span>
      <span v-else>● Teče (trenutna seja)</span>
    </div>

    <div class="buttons">
      <button v-if="!running" type="button" class="btn primary big" :disabled="busy" @click="start">▶ Start</button>
      <button v-else type="button" class="btn danger big" :disabled="busy" @click="stop">■ Stop</button>
    </div>
    <p class="muted small center">Stop shrani dogodek <strong>stop</strong> + čas. Pavze dodajte na strani Vnosi po dnevih.</p>
    <p v-if="error" class="error">{{ error }}</p>
  </div>
</template>

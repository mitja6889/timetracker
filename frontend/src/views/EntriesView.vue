<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, ApiError, type TimeDay, type UserRow } from '../api'
import DayPauseModal from '../components/DayPauseModal.vue'
import ManualEventModal from '../components/ManualEventModal.vue'
import { useConfirm } from '../stores/confirm'
import { useAuth } from '../stores/auth'
import { formatDateTime, formatDayGroupLabel, formatDuration, eventTypeLabel } from '../utils'

const auth = useAuth()
const confirm = useConfirm()
const canViewAll = computed(() => auth.has('entries.view_all'))
const canAddPause = computed(() => auth.has('timer.use') || auth.has('entries.edit_own'))
const canAddEvent = computed(
  () => auth.has('timer.use') || auth.has('entries.edit_own') || auth.has('entries.edit_all'),
)
const canDeleteEvent = canAddEvent

const pad2 = (n: number) => String(n).padStart(2, '0')
const todayLocal = () => {
  const d = new Date()
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}
const monthStartLocal = () => {
  const d = new Date()
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-01`
}

const range = reactive({ from: monthStartLocal(), to: todayLocal() })
const filterUserId = ref('')
const users = ref<UserRow[]>([])

const days = ref<TimeDay[]>([])
const rangeWorkSeconds = ref(0)
const loading = ref(false)
const error = ref('')

const expandedDays = ref<Set<string>>(new Set())
const pauseDayKey = ref<string | null>(null)
const showEventModal = ref(false)
const eventDefaultDate = ref<string | undefined>(undefined)

const todayKey = computed(() => todayLocal())

function isDayExpanded(key: string) {
  return expandedDays.value.has(key)
}

function toggleDay(key: string) {
  const next = new Set(expandedDays.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedDays.value = next
}

function dayLabel(key: string) {
  return formatDayGroupLabel(key, todayKey.value)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.timeDays({
      from: range.from,
      to: range.to,
      userId: canViewAll.value && filterUserId.value ? filterUserId.value : undefined,
    })
    days.value = res.days
    rangeWorkSeconds.value = res.rangeWorkSeconds
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka pri nalaganju'
  } finally {
    loading.value = false
  }
}

watch([() => range.from, () => range.to, filterUserId], load)

function openPause(dayKey: string, ev: Event) {
  ev.stopPropagation()
  pauseDayKey.value = dayKey
}

function openEventModal(dayKey?: string) {
  eventDefaultDate.value = dayKey
  showEventModal.value = true
}

async function removeEvent(id: string) {
  if (!(await confirm({ message: 'Izbrišem dogodek?', variant: 'danger', confirmLabel: 'Izbriši' }))) return
  try {
    await api.deleteTimeEvent(id)
    await load()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Brisanje ni uspelo'
  }
}

async function removePause(id: string) {
  if (!(await confirm({ message: 'Izbrišem pavzo?', variant: 'danger', confirmLabel: 'Izbriši' }))) return
  try {
    await api.deleteDayPause(id)
    await load()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Brisanje ni uspelo'
  }
}

onMounted(async () => {
  if (canViewAll.value || auth.has('entries.edit_all')) {
    try {
      users.value = await api.users()
    } catch {
      /* optional */
    }
  }
  await load()
})
</script>

<template>
  <div class="card entries-page">
    <div class="row between wrap page-head">
      <h2>Beleženje po dnevih</h2>
      <button v-if="canAddEvent" type="button" class="btn primary" @click="openEventModal()">+ Dogodek</button>
    </div>

    <section class="range-bar range-bar-compact" aria-label="Obdobje">
      <div class="range-bar-fields">
        <label class="range-field">
          <span>Od</span>
          <input v-model="range.from" type="date" />
        </label>
        <label class="range-field">
          <span>Do</span>
          <input v-model="range.to" type="date" />
        </label>
      </div>
      <div class="range-summary">
        <strong class="range-total">{{ formatDuration(rangeWorkSeconds) }}</strong>
        <span class="muted small">skupaj v obdobju (dogodki − pavze)</span>
      </div>
    </section>

    <div v-if="canViewAll" class="filters-panel">
      <label class="sort-control">
        <span class="muted small">Uporabnik</span>
        <select v-model="filterUserId">
          <option value="">Jaz / privzeto</option>
          <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
        </select>
      </label>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="entry-day-rows" :class="{ loading }">
      <p v-if="!loading && !days.length" class="muted center entries-empty">Ni dogodkov v obdobju.</p>

      <section
        v-for="day in days"
        :key="day.dayKey"
        class="entry-day-row"
        :class="{ expanded: isDayExpanded(day.dayKey) }"
      >
        <div class="entry-day-summary-row">
          <button
            type="button"
            class="entry-day-summary"
            :aria-expanded="isDayExpanded(day.dayKey)"
            @click="toggleDay(day.dayKey)"
          >
            <span class="entry-chevron" aria-hidden="true">›</span>
            <span class="entry-day-summary-body">
              <span class="entry-day-label">{{ dayLabel(day.dayKey) }}</span>
              <span class="entry-day-meta muted small">
                {{ day.events.length }} dogodkov · {{ day.pauses.length }} pavz
              </span>
            </span>
            <span class="entry-row-duration">{{ formatDuration(day.workSeconds) }}</span>
          </button>
          <div class="day-row-actions">
            <button
              v-if="canAddEvent"
              type="button"
              class="btn ghost day-pause-btn"
              @click.stop="openEventModal(day.dayKey)"
            >
              + Dogodek
            </button>
            <button
              v-if="canAddPause"
              type="button"
              class="btn ghost day-pause-btn"
              @click="openPause(day.dayKey, $event)"
            >
              Pavza
            </button>
          </div>
        </div>

        <div v-show="isDayExpanded(day.dayKey)" class="entry-day-children">
          <div class="day-detail-block">
            <h4 class="day-detail-title">Dogodki</h4>
            <ul v-if="day.events.length" class="event-list">
              <li v-for="ev in day.events" :key="ev.id" class="event-list-row">
                <span>
                  <strong>{{ eventTypeLabel(ev.type) }}</strong>
                  {{ formatDateTime(ev.at) }}
                </span>
                <button
                  v-if="canDeleteEvent"
                  type="button"
                  class="btn ghost danger-text btn-xs"
                  @click="removeEvent(ev.id)"
                >
                  ✕
                </button>
              </li>
            </ul>
            <p v-else class="muted small">Brez dogodkov.</p>
          </div>
          <div class="day-detail-block">
            <h4 class="day-detail-title">Pavze (odštete)</h4>
            <ul v-if="day.pauses.length" class="pause-list-detailed">
              <li v-for="p in day.pauses" :key="p.id">
                <span>{{ p.minutes }} min</span>
                <span v-if="p.note" class="muted small"> · {{ p.note }}</span>
                <button type="button" class="btn ghost danger-text btn-xs" @click="removePause(p.id)">✕</button>
              </li>
            </ul>
            <p v-else class="muted small">Brez pavz.</p>
          </div>
        </div>
      </section>
    </div>

    <DayPauseModal v-if="pauseDayKey" :day-key="pauseDayKey" @close="pauseDayKey = null" @saved="pauseDayKey = null; load()" />
    <ManualEventModal
      v-if="showEventModal"
      :users="users"
      :default-date="eventDefaultDate"
      @close="showEventModal = false"
      @saved="showEventModal = false; load()"
    />
  </div>
</template>

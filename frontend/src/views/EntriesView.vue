<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, ApiError, type Entry, type EntriesPage, type UserRow } from '../api'
import EntryForm from '../components/EntryForm.vue'
import { useAuth } from '../stores/auth'
import {
  dayStartISO,
  formatDateTime,
  formatDayGroupLabel,
  formatDuration,
  formatTime,
  localDayKey,
  nextDayStartISO,
} from '../utils'

const auth = useAuth()

const canViewAll = computed(() => auth.has('entries.view_all'))
const canCreate = computed(() => auth.has('entries.edit_own') || auth.has('entries.edit_all'))

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
const filters = reactive({ q: '', status: '', userId: '' })
const showFilters = ref(false)

const sort = ref('startedAt')
const order = ref<'asc' | 'desc'>('desc')
const page = ref(1)
const pageSize = ref(10)

const data = ref<EntriesPage | null>(null)
const users = ref<UserRow[]>([])
const loading = ref(false)
const error = ref('')

const editing = ref<Entry | null>(null)
const showForm = ref(false)
const expanded = ref<Set<string>>(new Set())

const totalPages = computed(() => Math.max(1, Math.ceil((data.value?.total ?? 0) / pageSize.value)))
const rangeWorkSeconds = computed(() => data.value?.rangeWorkSeconds ?? 0)
const hasRange = computed(() => Boolean(range.from || range.to))
const todayKey = computed(() => todayLocal())

const activeFilterCount = computed(() => {
  let n = 0
  if (filters.q.trim()) n++
  if (filters.status) n++
  if (filters.userId) n++
  return n
})

interface DayGroup {
  key: string
  label: string
  totalSeconds: number
  entries: Entry[]
}

const groupedEntries = computed((): DayGroup[] => {
  const items = data.value?.items ?? []
  const byDay = new Map<string, Entry[]>()
  for (const e of items) {
    const key = localDayKey(e.startedAt)
    const list = byDay.get(key)
    if (list) list.push(e)
    else byDay.set(key, [e])
  }

  const keys = [...byDay.keys()].sort((a, b) => {
    if (sort.value === 'startedAt') {
      return order.value === 'asc' ? a.localeCompare(b) : b.localeCompare(a)
    }
    return b.localeCompare(a)
  })

  return keys.map((key) => {
    const entries = byDay.get(key) ?? []
    return {
      key,
      label: formatDayGroupLabel(key, todayKey.value),
      totalSeconds: entries.reduce((s, e) => s + e.workSeconds, 0),
      entries,
    }
  })
})

function isExpanded(id: string) {
  return expanded.value.has(id)
}

function toggleExpanded(id: string) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

// zaščita pred zaostalimi odgovori pri hitrem spreminjanju filtrov
let seq = 0
async function load() {
  const mine = ++seq
  loading.value = true
  error.value = ''
  expanded.value = new Set()
  try {
    const res = await api.entries({
      page: page.value,
      pageSize: pageSize.value,
      sort: sort.value,
      order: order.value,
      q: filters.q.trim() || undefined,
      status: filters.status || undefined,
      userId: canViewAll.value ? filters.userId || undefined : undefined,
      from: range.from ? dayStartISO(range.from) : undefined,
      to: range.to ? nextDayStartISO(range.to) : undefined,
    })
    if (mine !== seq) return
    data.value = res
    if (res.items.length === 0 && page.value > 1) {
      page.value = Math.max(1, Math.ceil(res.total / pageSize.value))
      await load()
    }
  } catch (e) {
    if (mine === seq) error.value = e instanceof ApiError ? e.message : 'Napaka pri nalaganju'
  } finally {
    if (mine === seq) loading.value = false
  }
}

function resetAndLoad() {
  page.value = 1
  load()
}

let debounce: number | undefined
watch(
  () => filters.q,
  () => {
    window.clearTimeout(debounce)
    debounce = window.setTimeout(resetAndLoad, 300)
  },
)
watch([() => filters.status, () => filters.userId, pageSize], resetAndLoad)
watch([() => range.from, () => range.to], resetAndLoad)
watch(sort, resetAndLoad)

function toggleOrder() {
  order.value = order.value === 'asc' ? 'desc' : 'asc'
  resetAndLoad()
}

function clearFilters() {
  filters.q = ''
  filters.status = ''
  filters.userId = ''
}

function toggleFilters() {
  showFilters.value = !showFilters.value
}

function goto(p: number) {
  page.value = Math.min(Math.max(1, p), totalPages.value)
  load()
}

function openCreate() {
  editing.value = null
  showForm.value = true
}
function openEdit(e: Entry) {
  editing.value = e
  showForm.value = true
}
function onSaved() {
  showForm.value = false
  load()
}

async function remove(e: Entry) {
  if (!window.confirm(`Izbrišem vnos "${e.description || 'brez opisa'}" (${formatDateTime(e.startedAt)})?`)) return
  try {
    await api.deleteEntry(e.id)
    await load()
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : 'Brisanje ni uspelo'
  }
}

const statusLabel: Record<string, string> = { running: 'Teče', paused: 'Pavza', stopped: 'Zaključen' }

const sortOptions = [
  { value: 'startedAt', label: 'Datum začetka' },
  { value: 'endedAt', label: 'Datum konca' },
  { value: 'description', label: 'Opis' },
  { value: 'status', label: 'Status' },
]

onMounted(async () => {
  if (canViewAll.value || auth.has('entries.edit_all')) {
    try {
      users.value = await api.users()
    } catch {
      /* seznam uporabnikov ni nujen */
    }
  }
  await load()
})
</script>

<template>
  <div class="card entries-page">
    <div class="row between wrap page-head">
      <h2>Vnosi <small v-if="!canViewAll" class="muted">(moji)</small></h2>
      <button v-if="canCreate" type="button" class="btn primary" @click="openCreate">+ Nov</button>
    </div>

    <section class="range-bar range-bar-compact" aria-label="Obdobje in vsota časa">
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
        <strong class="range-total">{{ hasRange ? formatDuration(rangeWorkSeconds) : '—' }}</strong>
        <span v-if="hasRange" class="muted small">{{ data?.total ?? 0 }} vnosov v obdobju</span>
      </div>
    </section>

    <div class="entries-toolbar row wrap">
      <button
        type="button"
        class="btn ghost filters-toggle"
        :aria-expanded="showFilters"
        @click="toggleFilters"
      >
        Filtri
        <span v-if="activeFilterCount" class="filter-badge">{{ activeFilterCount }}</span>
      </button>
      <label class="sort-control">
        <span class="muted small">Razvrsti</span>
        <select v-model="sort">
          <option v-for="o in sortOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </label>
      <button type="button" class="btn ghost sort-order-btn" :aria-label="order === 'desc' ? 'Padajoče' : 'Naraščajoče'" @click="toggleOrder">
        {{ order === 'desc' ? '↓' : '↑' }}
      </button>
    </div>

    <div v-show="showFilters" class="filters-panel">
      <div class="filters">
        <input v-model="filters.q" placeholder="Išči po opisu…" />
        <select v-model="filters.status">
          <option value="">Vsi statusi</option>
          <option value="running">Teče</option>
          <option value="paused">Pavza</option>
          <option value="stopped">Zaključen</option>
        </select>
        <select v-if="canViewAll" v-model="filters.userId">
          <option value="">Vsi uporabniki</option>
          <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
        </select>
        <button type="button" class="btn ghost" @click="clearFilters">Počisti</button>
      </div>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="entries-list" :class="{ loading }">
      <p v-if="data && !data.items.length" class="muted center entries-empty">Ni vnosov za izbrano obdobje.</p>

      <section v-for="group in groupedEntries" :key="group.key" class="entry-day-group">
        <header class="entry-day-head">
          <h3 class="entry-day-label">{{ group.label }}</h3>
          <span class="entry-day-stats muted small">
            {{ group.entries.length }} · {{ formatDuration(group.totalSeconds) }}
          </span>
        </header>

        <div class="entry-rows">
          <article
            v-for="e in group.entries"
            :key="e.id"
            class="entry-row"
            :class="{ expanded: isExpanded(e.id) }"
          >
            <button
              type="button"
              class="entry-row-summary"
              :aria-expanded="isExpanded(e.id)"
              @click="toggleExpanded(e.id)"
            >
              <span class="entry-chevron" aria-hidden="true">›</span>
              <span class="entry-row-body">
                <span class="entry-row-title">{{ e.description || 'Brez opisa' }}</span>
                <span class="entry-row-meta">
                  <span>{{ formatTime(e.startedAt) }}–{{ formatTime(e.endedAt) }}</span>
                  <span class="badge" :class="e.status">{{ statusLabel[e.status] }}</span>
                </span>
                <span v-if="canViewAll" class="entry-row-user muted small">{{ e.name }}</span>
              </span>
              <span class="entry-row-duration">{{ formatDuration(e.workSeconds) }}</span>
            </button>

            <div v-show="isExpanded(e.id)" class="entry-row-detail">
              <dl class="detail-grid">
                <template v-if="canViewAll">
                  <dt>Uporabnik</dt>
                  <dd>{{ e.name }} <span class="muted">({{ e.username }})</span></dd>
                </template>
                <dt>Opis</dt>
                <dd>{{ e.description || '—' }}</dd>
                <dt>Začetek</dt>
                <dd>{{ formatDateTime(e.startedAt) }}</dd>
                <dt>Konec</dt>
                <dd>{{ formatDateTime(e.endedAt) }}</dd>
                <dt>Delovni čas</dt>
                <dd>{{ formatDuration(e.workSeconds) }}</dd>
                <dt>Status</dt>
                <dd><span class="badge" :class="e.status">{{ statusLabel[e.status] }}</span></dd>
                <dt>Pavze</dt>
                <dd>
                  <ul v-if="e.pauses.length" class="pause-list">
                    <li v-for="(p, i) in e.pauses" :key="i">
                      {{ formatDateTime(p.start) }} – {{ p.end ? formatDateTime(p.end) : '…' }}
                    </li>
                  </ul>
                  <span v-else class="muted">Brez pavz</span>
                </dd>
              </dl>
              <div v-if="e.canEdit || e.canDelete" class="entry-row-actions">
                <button v-if="e.canEdit" type="button" class="btn ghost" @click.stop="openEdit(e)">Uredi</button>
                <button v-if="e.canDelete" type="button" class="btn ghost danger-text" @click.stop="remove(e)">
                  Izbriši
                </button>
              </div>
            </div>
          </article>
        </div>
      </section>
    </div>

    <div class="pager pager-compact">
      <div class="row pager-nav">
        <button type="button" class="btn ghost" :disabled="page <= 1" @click="goto(page - 1)">Prejšnja</button>
        <span class="pager-pos">{{ page }} / {{ totalPages }}</span>
        <button type="button" class="btn ghost" :disabled="page >= totalPages" @click="goto(page + 1)">Naslednja</button>
      </div>
      <label class="pager-size">
        <span class="muted small">Na stran</span>
        <select v-model.number="pageSize">
          <option :value="10">10</option>
          <option :value="25">25</option>
          <option :value="50">50</option>
        </select>
      </label>
    </div>
  </div>

  <EntryForm
    v-if="showForm"
    :entry="editing"
    :users="users"
    :can-pick-user="auth.has('entries.edit_all')"
    @close="showForm = false"
    @saved="onSaved"
  />
</template>

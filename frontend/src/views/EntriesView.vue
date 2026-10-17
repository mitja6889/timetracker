<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, ApiError, type Entry, type EntriesPage, type UserRow } from '../api'
import EntryForm from '../components/EntryForm.vue'
import { useAuth } from '../stores/auth'
import { dayStartISO, formatDateTime, formatDuration, nextDayStartISO } from '../utils'

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

const totalPages = computed(() => Math.max(1, Math.ceil((data.value?.total ?? 0) / pageSize.value)))
const rangeWorkSeconds = computed(() => data.value?.rangeWorkSeconds ?? 0)
const hasRange = computed(() => Boolean(range.from || range.to))

const activeFilterCount = computed(() => {
  let n = 0
  if (filters.q.trim()) n++
  if (filters.status) n++
  if (filters.userId) n++
  return n
})

// zaščita pred zaostalimi odgovori pri hitrem spreminjanju filtrov
let seq = 0
async function load() {
  const mine = ++seq
  loading.value = true
  error.value = ''
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

function sortBy(col: string) {
  if (sort.value === col) order.value = order.value === 'asc' ? 'desc' : 'asc'
  else {
    sort.value = col
    order.value = col === 'description' ? 'asc' : 'desc'
  }
  resetAndLoad()
}
const arrow = (col: string) => (sort.value === col ? (order.value === 'asc' ? ' ▲' : ' ▼') : '')

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
      <h2>Vnosi časa <small v-if="!canViewAll" class="muted">(samo moji)</small></h2>
      <button v-if="canCreate" class="btn primary" @click="openCreate">+ Nov vnos</button>
    </div>

    <div class="entries-toolbar">
      <button
        type="button"
        class="btn ghost filters-toggle"
        :aria-expanded="showFilters"
        @click="toggleFilters"
      >
        {{ showFilters ? 'Skrij filtre' : 'Prikaži filtre' }}
        <span v-if="activeFilterCount" class="filter-badge">{{ activeFilterCount }}</span>
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
          <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }} ({{ u.username }})</option>
        </select>
        <button type="button" class="btn ghost" @click="clearFilters">Počisti filtre</button>
      </div>
    </div>

    <section class="range-bar" aria-label="Obdobje in vsota časa">
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
        <span class="muted small">Skupni delovni čas v obdobju</span>
        <strong class="range-total">{{ hasRange ? formatDuration(rangeWorkSeconds) : '—' }}</strong>
        <span v-if="hasRange" class="muted small">{{ data?.total ?? 0 }} vnosov</span>
      </div>
    </section>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="table-wrap table-scroll" :class="{ loading }">
      <table class="table entries-table">
        <thead>
          <tr>
            <th v-if="canViewAll">Uporabnik</th>
            <th class="sortable" @click="sortBy('description')">Opis{{ arrow('description') }}</th>
            <th class="sortable" @click="sortBy('startedAt')">Začetek{{ arrow('startedAt') }}</th>
            <th class="sortable" @click="sortBy('endedAt')">Konec{{ arrow('endedAt') }}</th>
            <th class="num">Pavze</th>
            <th class="num">Delovni čas</th>
            <th class="sortable" @click="sortBy('status')">Status{{ arrow('status') }}</th>
            <th class="actions-col">Akcije</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in data?.items ?? []" :key="e.id">
            <td v-if="canViewAll">{{ e.name }} <small class="muted">{{ e.username }}</small></td>
            <td class="desc-col">{{ e.description || '—' }}</td>
            <td>{{ formatDateTime(e.startedAt) }}</td>
            <td>{{ formatDateTime(e.endedAt) }}</td>
            <td class="num">{{ e.pauses.length }}</td>
            <td class="num">{{ formatDuration(e.workSeconds) }}</td>
            <td><span class="badge" :class="e.status">{{ statusLabel[e.status] }}</span></td>
            <td class="actions">
              <button v-if="e.canEdit" type="button" class="btn ghost" @click="openEdit(e)">Uredi</button>
              <button v-if="e.canDelete" type="button" class="btn ghost danger-text" @click="remove(e)">Izbriši</button>
            </td>
          </tr>
          <tr v-if="data && !data.items.length">
            <td :colspan="canViewAll ? 8 : 7" class="muted center">Ni vnosov za izbrano obdobje in filtre.</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pager">
      <span class="muted pager-summary">Stran {{ page }} / {{ totalPages }}</span>
      <div class="row">
        <select v-model.number="pageSize">
          <option :value="10">10 / stran</option>
          <option :value="25">25 / stran</option>
          <option :value="50">50 / stran</option>
        </select>
        <button type="button" class="btn ghost" :disabled="page <= 1" @click="goto(page - 1)">‹</button>
        <span>{{ page }} / {{ totalPages }}</span>
        <button type="button" class="btn ghost" :disabled="page >= totalPages" @click="goto(page + 1)">›</button>
      </div>
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

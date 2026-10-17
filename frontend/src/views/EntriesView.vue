<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, ApiError, type Entry, type EntriesPage, type UserRow } from '../api'
import EntryForm from '../components/EntryForm.vue'
import { useAuth } from '../stores/auth'
import { dayStartISO, formatDateTime, formatDuration, nextDayStartISO } from '../utils'

const auth = useAuth()

const canViewAll = computed(() => auth.has('entries.view_all'))
const canCreate = computed(() => auth.has('entries.edit_own') || auth.has('entries.edit_all'))

const filters = reactive({ q: '', status: '', userId: '', from: '', to: '' })
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
const pageWork = computed(() => (data.value?.items ?? []).reduce((s, e) => s + e.workSeconds, 0))

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
      from: filters.from ? dayStartISO(filters.from) : undefined,
      to: filters.to ? nextDayStartISO(filters.to) : undefined,
    })
    if (mine !== seq) return
    data.value = res
    // če je zadnja stran po brisanju prazna, skoči nazaj
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
watch([() => filters.status, () => filters.userId, () => filters.from, () => filters.to, pageSize], resetAndLoad)

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
  filters.from = ''
  filters.to = ''
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
  <div class="card">
    <div class="row between wrap page-head">
      <h2>Vnosi časa <small v-if="!canViewAll" class="muted">(samo moji)</small></h2>
      <button v-if="canCreate" class="btn primary" @click="openCreate">+ Nov vnos</button>
    </div>

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
      <label class="inline">Od <input v-model="filters.from" type="date" /></label>
      <label class="inline">Do <input v-model="filters.to" type="date" /></label>
      <button class="btn ghost" @click="clearFilters">Počisti</button>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="table-wrap" :class="{ loading }">
      <table class="table table-stack">
        <thead>
          <tr>
            <th v-if="canViewAll">Uporabnik</th>
            <th class="sortable" @click="sortBy('description')">Opis{{ arrow('description') }}</th>
            <th class="sortable" @click="sortBy('startedAt')">Začetek{{ arrow('startedAt') }}</th>
            <th class="sortable" @click="sortBy('endedAt')">Konec{{ arrow('endedAt') }}</th>
            <th class="num">Pavze</th>
            <th class="num">Delovni čas</th>
            <th class="sortable" @click="sortBy('status')">Status{{ arrow('status') }}</th>
            <th>Akcije</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in data?.items ?? []" :key="e.id">
            <td v-if="canViewAll" data-label="Uporabnik">{{ e.name }} <small class="muted">{{ e.username }}</small></td>
            <td data-label="Opis">{{ e.description || '—' }}</td>
            <td data-label="Začetek">{{ formatDateTime(e.startedAt) }}</td>
            <td data-label="Konec">{{ formatDateTime(e.endedAt) }}</td>
            <td class="num" data-label="Pavze">{{ e.pauses.length }}</td>
            <td class="num" data-label="Delovni čas">{{ formatDuration(e.workSeconds) }}</td>
            <td data-label="Status"><span class="badge" :class="e.status">{{ statusLabel[e.status] }}</span></td>
            <td class="actions" data-label="Akcije">
              <button v-if="e.canEdit" class="btn ghost" @click="openEdit(e)">Uredi</button>
              <button v-if="e.canDelete" class="btn ghost danger-text" @click="remove(e)">Izbriši</button>
            </td>
          </tr>
          <tr v-if="data && !data.items.length">
            <td :colspan="canViewAll ? 8 : 7" class="muted center table-empty">Ni vnosov za izbrane filtre.</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pager">
      <span class="muted pager-summary">
        Skupaj {{ data?.total ?? 0 }} vnosov · delovni čas na strani: {{ formatDuration(pageWork) }}
      </span>
      <div class="row">
        <select v-model.number="pageSize">
          <option :value="10">10 / stran</option>
          <option :value="25">25 / stran</option>
          <option :value="50">50 / stran</option>
        </select>
        <button class="btn ghost" :disabled="page <= 1" @click="goto(page - 1)">‹</button>
        <span>{{ page }} / {{ totalPages }}</span>
        <button class="btn ghost" :disabled="page >= totalPages" @click="goto(page + 1)">›</button>
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

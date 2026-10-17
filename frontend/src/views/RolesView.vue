<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, ApiError, type PermissionInfo, type Role } from '../api'
import AppModal from '../components/AppModal.vue'
import { useConfirm } from '../stores/confirm'
import { useAuth } from '../stores/auth'

const auth = useAuth()
const confirm = useConfirm()

const roles = ref<Role[]>([])
const permissions = ref<PermissionInfo[]>([])
const error = ref('')

const showForm = ref(false)
const editing = ref<Role | null>(null)
const formError = ref('')
const busy = ref(false)
const form = reactive({ name: '', description: '', permissions: [] as string[] })

const permLabel = (key: string) => permissions.value.find((p) => p.key === key)?.label ?? key

async function load() {
  error.value = ''
  try {
    ;[roles.value, permissions.value] = await Promise.all([api.roles(), api.permissions()])
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka pri nalaganju'
  }
}

function openCreate() {
  editing.value = null
  Object.assign(form, { name: '', description: '', permissions: ['timer.use'] })
  formError.value = ''
  showForm.value = true
}

function openEdit(r: Role) {
  editing.value = r
  Object.assign(form, { name: r.name, description: r.description, permissions: [...r.permissions] })
  formError.value = ''
  showForm.value = true
}

async function save() {
  formError.value = ''
  busy.value = true
  try {
    const body = { name: form.name, description: form.description, permissions: form.permissions }
    if (editing.value) {
      await api.updateRole(editing.value.id, body)
      // pravice prijavljenega uporabnika so se morda spremenile
      await auth.refresh()
    } else {
      await api.createRole(body)
    }
    showForm.value = false
    await load()
  } catch (e) {
    formError.value = e instanceof ApiError ? e.message : 'Napaka pri shranjevanju'
  } finally {
    busy.value = false
  }
}

async function remove(r: Role) {
  if (
    !(await confirm({
      message: `Izbrišem rolo "${r.name}"?`,
      variant: 'danger',
      confirmLabel: 'Izbriši',
    }))
  )
    return
  try {
    await api.deleteRole(r.id)
    await load()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Brisanje ni uspelo'
  }
}

onMounted(load)
</script>

<template>
  <div class="card">
    <div class="row between page-head">
      <h2>Role in pravice</h2>
      <button class="btn primary" @click="openCreate">+ Nova role</button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="table-wrap">
      <table class="table table-stack">
        <thead>
          <tr><th>Ime</th><th>Opis</th><th>Pravice</th><th class="num">Uporabniki</th><th>Akcije</th></tr>
        </thead>
        <tbody>
          <tr v-for="r in roles" :key="r.id">
            <td data-label="Ime">{{ r.name }} <span v-if="r.system" class="badge running">sistemska</span></td>
            <td data-label="Opis">{{ r.description || '—' }}</td>
            <td data-label="Pravice">
              <span v-for="p in r.permissions" :key="p" class="chip">{{ permLabel(p) }}</span>
              <span v-if="!r.permissions.length" class="muted">brez pravic</span>
            </td>
            <td class="num" data-label="Uporabniki">{{ r.userCount }}</td>
            <td class="actions" data-label="Akcije">
              <button class="btn ghost" @click="openEdit(r)">Uredi</button>
              <button class="btn ghost danger-text" :disabled="r.system" @click="remove(r)">Izbriši</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <AppModal v-if="showForm" :title="editing ? 'Uredi rolo' : 'Nova role'" @close="showForm = false">
    <form id="role-form" class="form" @submit.prevent="save">
      <label>
        Ime
        <input v-model="form.name" :disabled="editing?.system" minlength="2" maxlength="50" required />
      </label>
      <label>
        Opis
        <input v-model="form.description" />
      </label>
      <fieldset :disabled="editing?.system">
        <legend>Pravice</legend>
        <label v-for="p in permissions" :key="p.key" class="check">
          <input v-model="form.permissions" type="checkbox" :value="p.key" /> {{ p.label }}
        </label>
        <p v-if="editing?.system" class="muted small">Pravice sistemske role ni mogoče spreminjati.</p>
      </fieldset>
      <p v-if="formError" class="error">{{ formError }}</p>
    </form>
    <template #footer>
      <button class="btn ghost" type="button" @click="showForm = false">Prekliči</button>
      <button class="btn primary" form="role-form" :disabled="busy">Shrani</button>
    </template>
  </AppModal>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, ApiError, type Role, type UserRow } from '../api'
import AppModal from '../components/AppModal.vue'
import { useAuth } from '../stores/auth'
import { formatDateTime } from '../utils'

const auth = useAuth()

const users = ref<UserRow[]>([])
const roles = ref<Role[]>([])
const error = ref('')

const showForm = ref(false)
const editingId = ref<string | null>(null)
const formError = ref('')
const busy = ref(false)
const form = reactive({ username: '', name: '', password: '', roleId: '', active: true })

async function load() {
  error.value = ''
  try {
    ;[users.value, roles.value] = await Promise.all([api.users(), api.roles()])
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Napaka pri nalaganju'
  }
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { username: '', name: '', password: '', roleId: roles.value[0]?.id ?? '', active: true })
  formError.value = ''
  showForm.value = true
}

function openEdit(u: UserRow) {
  editingId.value = u.id
  Object.assign(form, {
    username: u.username,
    name: u.name,
    password: '',
    roleId: u.roleId ?? '',
    active: u.active ?? true,
  })
  formError.value = ''
  showForm.value = true
}

async function save() {
  formError.value = ''
  busy.value = true
  try {
    if (editingId.value) {
      await api.updateUser(editingId.value, {
        name: form.name,
        roleId: form.roleId,
        active: form.active,
        password: form.password || undefined,
      })
      // lahko smo spremenili svojo rolo
      if (editingId.value === auth.me?.id) await auth.refresh()
    } else {
      await api.createUser({
        username: form.username,
        name: form.name,
        password: form.password,
        roleId: form.roleId,
        active: form.active,
      })
    }
    showForm.value = false
    await load()
  } catch (e) {
    formError.value = e instanceof ApiError ? e.message : 'Napaka pri shranjevanju'
  } finally {
    busy.value = false
  }
}

async function remove(u: UserRow) {
  if (!window.confirm(`Izbrišem uporabnika "${u.username}" in vse njegove vnose časa?`)) return
  try {
    await api.deleteUser(u.id)
    await load()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Brisanje ni uspelo'
  }
}

onMounted(load)
</script>

<template>
  <div class="card">
    <div class="row between">
      <h2>Uporabniki</h2>
      <button class="btn primary" @click="openCreate">+ Nov uporabnik</button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <table class="table">
      <thead>
        <tr><th>Uporabniško ime</th><th>Ime</th><th>Role</th><th>Status</th><th>Ustvarjen</th><th></th></tr>
      </thead>
      <tbody>
        <tr v-for="u in users" :key="u.id">
          <td>{{ u.username }}</td>
          <td>{{ u.name }}</td>
          <td>{{ u.roleName }}</td>
          <td><span class="badge" :class="u.active ? 'stopped' : 'paused'">{{ u.active ? 'Aktiven' : 'Onemogočen' }}</span></td>
          <td>{{ formatDateTime(u.createdAt ?? null) }}</td>
          <td class="actions">
            <button class="btn ghost" @click="openEdit(u)">Uredi</button>
            <button class="btn ghost danger-text" :disabled="u.id === auth.me?.id" @click="remove(u)">Izbriši</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <AppModal v-if="showForm" :title="editingId ? 'Uredi uporabnika' : 'Nov uporabnik'" @close="showForm = false">
    <form id="user-form" class="form" @submit.prevent="save">
      <label>
        Uporabniško ime
        <input v-model="form.username" :disabled="!!editingId" pattern="[a-z0-9._-]{3,32}" required
               title="3-32 znakov: male črke, številke, . _ -" />
      </label>
      <label>
        Ime in priimek
        <input v-model="form.name" required />
      </label>
      <label>
        {{ editingId ? 'Novo geslo (pustite prazno za ohranitev)' : 'Geslo (vsaj 8 znakov)' }}
        <input v-model="form.password" type="password" minlength="8" autocomplete="new-password" :required="!editingId" />
      </label>
      <label>
        Role
        <select v-model="form.roleId" required>
          <option v-for="r in roles" :key="r.id" :value="r.id">{{ r.name }}</option>
        </select>
      </label>
      <label class="check"><input v-model="form.active" type="checkbox" /> Aktiven</label>
      <p v-if="formError" class="error">{{ formError }}</p>
    </form>
    <template #footer>
      <button class="btn ghost" type="button" @click="showForm = false">Prekliči</button>
      <button class="btn primary" form="user-form" :disabled="busy">Shrani</button>
    </template>
  </AppModal>
</template>

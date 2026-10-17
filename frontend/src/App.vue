<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuth } from './stores/auth'

const auth = useAuth()
const router = useRouter()

function logout() {
  auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div v-if="auth.isLoggedIn" class="shell">
    <header class="topbar">
      <div class="brand">⏱ Beleženje časa</div>
      <nav>
        <RouterLink v-if="auth.has('timer.use')" to="/">Štoparica</RouterLink>
        <RouterLink to="/entries">Vnosi</RouterLink>
        <RouterLink v-if="auth.has('users.manage')" to="/users">Uporabniki</RouterLink>
        <RouterLink v-if="auth.has('roles.manage')" to="/roles">Role</RouterLink>
      </nav>
      <div class="who">
        <span>{{ auth.me?.name }} <small class="muted">({{ auth.me?.roleName }})</small></span>
        <button class="btn ghost" @click="logout">Odjava</button>
      </div>
    </header>
    <main class="content">
      <RouterView />
    </main>
  </div>
  <RouterView v-else />
</template>

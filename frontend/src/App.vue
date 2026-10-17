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
      <nav class="topnav" aria-label="Glavna navigacija">
        <RouterLink v-if="auth.has('timer.use')" to="/">Štoparica</RouterLink>
        <RouterLink to="/entries">Vnosi</RouterLink>
        <RouterLink v-if="auth.has('users.manage')" to="/users">Uporabniki</RouterLink>
        <RouterLink v-if="auth.has('roles.manage')" to="/roles">Role</RouterLink>
      </nav>
      <div class="who">
        <span class="who-name">{{ auth.me?.name }}</span>
        <span class="who-role muted">{{ auth.me?.roleName }}</span>
        <button class="btn ghost" type="button" @click="logout">Odjava</button>
      </div>
    </header>

    <nav class="tabbar" aria-label="Mobilna navigacija">
      <RouterLink v-if="auth.has('timer.use')" to="/" class="tabbar-item">
        <span class="tabbar-icon" aria-hidden="true">⏱</span>
        <span class="tabbar-label">Štoparica</span>
      </RouterLink>
      <RouterLink to="/entries" class="tabbar-item">
        <span class="tabbar-icon" aria-hidden="true">📋</span>
        <span class="tabbar-label">Vnosi</span>
      </RouterLink>
      <RouterLink v-if="auth.has('users.manage')" to="/users" class="tabbar-item">
        <span class="tabbar-icon" aria-hidden="true">👤</span>
        <span class="tabbar-label">Uporabniki</span>
      </RouterLink>
      <RouterLink v-if="auth.has('roles.manage')" to="/roles" class="tabbar-item">
        <span class="tabbar-icon" aria-hidden="true">🔐</span>
        <span class="tabbar-label">Role</span>
      </RouterLink>
    </nav>

    <main class="content">
      <RouterView />
    </main>
  </div>
  <RouterView v-else />
</template>

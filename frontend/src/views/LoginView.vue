<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ApiError } from '../api'
import { useAuth } from '../stores/auth'

const auth = useAuth()
const router = useRouter()

const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(username.value, password.value)
    router.push({ name: auth.has('timer.use') ? 'tracker' : 'entries' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Prijava ni uspela'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <form class="card login" @submit.prevent="submit">
      <h1>⏱ Beleženje časa</h1>
      <p class="muted">Prijavite se za nadaljevanje</p>
      <label>
        Uporabniško ime
        <input v-model="username" autocomplete="username" autofocus required />
      </label>
      <label>
        Geslo
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <p v-if="error" class="error">{{ error }}</p>
      <button class="btn primary" :disabled="busy">{{ busy ? 'Prijavljam…' : 'Prijava' }}</button>
    </form>
  </div>
</template>

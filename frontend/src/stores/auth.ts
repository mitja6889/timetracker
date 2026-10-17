import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, tokenStore, type Me } from '../api'

export const useAuth = defineStore('auth', () => {
  const me = ref<Me | null>(null)
  const ready = ref(false)

  const isLoggedIn = computed(() => me.value !== null)

  function has(perm: string): boolean {
    return me.value?.permissions.includes(perm) ?? false
  }

  async function login(username: string, password: string) {
    const res = await api.login(username, password)
    tokenStore.set(res.token)
    me.value = res.me
  }

  /** Ob zagonu aplikacije: če je žeton shranjen, naloži uporabnika. */
  async function restore() {
    if (tokenStore.get()) {
      try {
        me.value = await api.me()
      } catch {
        tokenStore.clear()
        me.value = null
      }
    }
    ready.value = true
  }

  /** Osveži pravice (npr. po spremembi role). */
  async function refresh() {
    try {
      me.value = await api.me()
    } catch {
      /* 401 obravnava handler */
    }
  }

  function logout() {
    tokenStore.clear()
    me.value = null
  }

  return { me, ready, isLoggedIn, has, login, restore, refresh, logout }
})

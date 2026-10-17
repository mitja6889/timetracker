import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'
import { setUnauthorizedHandler } from './api'
import { useAuth } from './stores/auth'
import './style.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

// Pretečena/neveljavna seja -> odjava in na prijavo
setUnauthorizedHandler(() => {
  const auth = useAuth()
  auth.logout()
  if (router.currentRoute.value.name !== 'login') router.push({ name: 'login' })
})

app.mount('#app')

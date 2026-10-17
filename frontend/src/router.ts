import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from './stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    perm?: string
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('./views/LoginView.vue'), meta: { public: true } },
    { path: '/', name: 'tracker', component: () => import('./views/TrackerView.vue'), meta: { perm: 'timer.use' } },
    { path: '/entries', name: 'entries', component: () => import('./views/EntriesView.vue') },
    { path: '/users', name: 'users', component: () => import('./views/UsersView.vue'), meta: { perm: 'users.manage' } },
    { path: '/roles', name: 'roles', component: () => import('./views/RolesView.vue'), meta: { perm: 'roles.manage' } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuth()
  if (!auth.ready) await auth.restore()

  if (to.meta.public) {
    return auth.isLoggedIn ? { name: auth.has('timer.use') ? 'tracker' : 'entries' } : true
  }
  if (!auth.isLoggedIn) return { name: 'login' }

  if (to.meta.perm && !auth.has(to.meta.perm)) {
    // brez pravice za štoparico pristane na seznamu vnosov
    return to.name === 'entries' ? true : { name: 'entries' }
  }
  return true
})

import { createRouter, createWebHistory } from 'vue-router'
import { useGatewayStore } from '../store/gateway'

const protectedRoutes = ['sessions', 'settings']

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      redirect: { name: 'sessions' },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/sessions',
      name: 'sessions',
      component: () => import('../views/SessionsView.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
    },
    {
      path: '/help',
      name: 'help',
      component: () => import('../views/HelpView.vue'),
    },
  ],
})

router.beforeEach((to) => {
  const gateway = useGatewayStore()
  if (protectedRoutes.includes(to.name as string) && !gateway.token) {
    return { name: 'login' }
  }
  if (to.name === 'login' && gateway.token) {
    return { name: 'sessions' }
  }
})

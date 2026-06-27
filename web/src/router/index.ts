import { createRouter, createWebHistory } from 'vue-router'
import { useGatewayStore } from '../store/gateway'

const skipAuthGuardRoutes = [
  'home',
  'login',
  'register',
  'verify-email',
  'forgot-password',
  'google-callback',
  'reset-password',
]

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('../views/HomeView.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('../views/RegisterView.vue'),
    },
    {
      path: '/verify-email',
      name: 'verify-email',
      component: () => import('../views/VerifyEmailView.vue'),
    },
    {
      path: '/forgot-password',
      name: 'forgot-password',
      component: () => import('../views/ForgotPasswordView.vue'),
    },
    {
      path: '/auth/google/callback',
      name: 'google-callback',
      component: () => import('../views/GoogleCallbackView.vue'),
    },
    {
      path: '/reset-password',
      name: 'reset-password',
      component: () => import('../views/ResetPasswordView.vue'),
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

router.beforeEach(async (to) => {
  const gateway = useGatewayStore()
  const routeName = to.name as string

  if (skipAuthGuardRoutes.includes(routeName)) {
    return
  }

  await gateway.initializeAuth()
  if (!gateway.authenticated) {
    return { name: 'login' }
  }
})

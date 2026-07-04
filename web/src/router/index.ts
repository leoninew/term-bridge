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
  'cloud-oauth-start',
  'cloud-oauth-callback',
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
      path: '/connect',
      name: 'connect',
      component: () => import('../views/ConnectView.vue'),
    },
    {
      path: '/cloud/oauth/start',
      alias: '/cloud/connect/start',
      name: 'cloud-oauth-start',
      component: () => import('../views/CloudConnectStartView.vue'),
    },
    {
      path: '/cloud/oauth/callback',
      alias: '/cloud/connect/callback',
      name: 'cloud-oauth-callback',
      component: () => import('../views/CloudConnectCallbackView.vue'),
    },
    {
      path: '/oauth2/authorize',
      alias: '/cloud/connect/authorize',
      name: 'cloud-oauth-authorize',
      component: () => import('../views/CloudConnectAuthorizeView.vue'),
    },
    {
      path: '/agent/dashboard',
      name: 'agent-dashboard',
      component: () => import('../views/AgentDashboardView.vue'),
    },
    {
      path: '/agent/sessions',
      name: 'agent-sessions',
      component: () => import('../views/AgentSessionsView.vue'),
    },
    {
      path: '/cloud/dashboard',
      name: 'cloud-dashboard',
      component: () => import('../views/CloudDashboardView.vue'),
    },
    {
      path: '/cloud/devices/:deviceId/sessions',
      name: 'cloud-sessions',
      component: () => import('../views/CloudSessionsView.vue'),
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
  if (gateway.capabilities?.mode === 'local') {
    return
  }
  if (!gateway.authenticated) {
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }
})

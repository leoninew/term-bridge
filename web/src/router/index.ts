import { createRouter, createWebHistory } from 'vue-router'
import { useAppModeStore, type RouteMode } from '../store/appMode'
import { useGatewayStore } from '../store/gateway'

const agentAuthWhitelistRoutes = ['home']

const cloudAuthWhitelistRoutes = [
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
      meta: { mode: 'both' },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('../views/RegisterView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/verify-email',
      name: 'verify-email',
      component: () => import('../views/VerifyEmailView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/forgot-password',
      name: 'forgot-password',
      component: () => import('../views/ForgotPasswordView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/auth/google/callback',
      name: 'google-callback',
      component: () => import('../views/GoogleCallbackView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/reset-password',
      name: 'reset-password',
      component: () => import('../views/ResetPasswordView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/connect',
      name: 'connect',
      component: () => import('../views/ConnectView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/agent/dashboard',
      name: 'agent-dashboard',
      component: () => import('../views/AgentDashboardView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/agent/sessions',
      name: 'agent-sessions',
      component: () => import('../views/AgentSessionsView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/cloud/dashboard',
      name: 'cloud-dashboard',
      component: () => import('../views/CloudDashboardView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/cloud/devices/:deviceId/sessions',
      name: 'cloud-sessions',
      component: () => import('../views/CloudSessionsView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/help',
      name: 'help',
      component: () => import('../views/HelpView.vue'),
      meta: { mode: 'cloud' },
    },
  ],
})

router.beforeEach(async (to) => {
  const gateway = useGatewayStore()
  const appMode = useAppModeStore()
  const routeName = to.name as string
  const routeMode = (to.meta.mode as RouteMode | undefined) ?? 'both'

  if (!appMode.allowsRouteMode(routeMode)) {
    return { name: appMode.dashboardRouteName() }
  }
  appMode.activateRouteMode(routeMode)

  const authWhitelistRoutes =
    appMode.effectiveMode === 'agent' ? agentAuthWhitelistRoutes : cloudAuthWhitelistRoutes
  if (authWhitelistRoutes.includes(routeName)) {
    return
  }

  await gateway.initializeAuth()
  if (!gateway.authenticated) {
    if (appMode.effectiveMode === 'agent') {
      return { name: 'home' }
    }
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }
})

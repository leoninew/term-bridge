import { createRouter, createWebHistory } from 'vue-router'
import { useAppModeStore, type RouteMode } from '../store/appMode'
import { useGatewayStore } from '../store/gateway'

const cloudAccountAuthRoutes = [
  'cloud-login',
  'cloud-register',
  'cloud-verify-email',
  'cloud-forgot-password',
  'cloud-google-callback',
  'cloud-oauth-authorize',
  'cloud-reset-password',
]

const agentAuthWhitelistRoutes = ['agent-oauth-callback']

const cloudAuthWhitelistRoutes = [...cloudAccountAuthRoutes]

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: () => {
        const appMode = useAppModeStore()
        return { name: appMode.dashboardRouteName() }
      },
    },
    {
      path: '/login',
      name: 'cloud-login',
      component: () => import('../views/cloud/LoginView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/register',
      name: 'cloud-register',
      component: () => import('../views/cloud/RegisterView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/verify-email',
      name: 'cloud-verify-email',
      component: () => import('../views/cloud/VerifyEmailView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/forgot-password',
      name: 'cloud-forgot-password',
      component: () => import('../views/cloud/ForgotPasswordView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/auth/google/callback',
      name: 'cloud-google-callback',
      component: () => import('../views/cloud/GoogleCallbackView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/oauth2/authorize',
      name: 'cloud-oauth-authorize',
      component: () => import('../views/cloud/OAuthAuthorizeView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/reset-password',
      name: 'cloud-reset-password',
      component: () => import('../views/cloud/ResetPasswordView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/agent/connect',
      name: 'agent-connect',
      component: () => import('../views/agent/ConnectView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/agent/dashboard',
      name: 'agent-dashboard',
      component: () => import('../views/agent/DashboardView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/agent/oauth/callback',
      name: 'agent-oauth-callback',
      component: () => import('../views/agent/OAuthCallbackView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/agent/sessions',
      name: 'agent-sessions',
      component: () => import('../views/agent/SessionsView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/cloud/dashboard',
      name: 'cloud-dashboard',
      component: () => import('../views/cloud/DashboardView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/cloud/devices/:deviceId/sessions',
      name: 'cloud-sessions',
      component: () => import('../views/cloud/SessionsView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/settings',
      name: 'cloud-settings',
      component: () => import('../views/cloud/SettingsView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/help',
      name: 'cloud-help',
      component: () => import('../views/cloud/HelpView.vue'),
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
      return routeName === 'agent-dashboard' ? undefined : { name: 'agent-dashboard' }
    }
    return {
      name: 'cloud-login',
      query: { redirect: to.fullPath },
    }
  }
})

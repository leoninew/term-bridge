import { createRouter, createWebHistory } from 'vue-router'
import type { AgentMode } from '../config'
import { useGatewayStore } from '../store/gateway'
import { useRuntimeConfigStore } from '../store/runtimeConfig'

const cloudAccountAuthRoutes = [
  'cloud-login',
  'cloud-register',
  'cloud-verify-email',
  'cloud-forgot-password',
  'cloud-google-callback',
  'cloud-oauth-authorize',
  'cloud-reset-password',
]

const homeRoute = 'home'

const agentAuthWhitelistRoutes = [homeRoute, 'agent-oauth-callback']

const cloudAuthWhitelistRoutes = [homeRoute, ...cloudAccountAuthRoutes]

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: homeRoute,
      component: () => import('../views/HomeView.vue'),
      meta: { mode: 'hybrid' },
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
      path: '/agent/oauth/callback',
      name: 'agent-oauth-callback',
      component: () => import('../views/agent/OAuthCallbackView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/sessions',
      name: 'agent-sessions',
      component: () => import('../views/agent/SessionsView.vue'),
      meta: { mode: 'agent' },
    },
    {
      path: '/dashboard',
      name: 'cloud-dashboard',
      component: () => import('../views/cloud/DashboardView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/devices/:deviceId/sessions',
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
  const runtimeConfig = useRuntimeConfigStore()
  const routeName = to.name as string
  const routeMode = (to.meta.mode as AgentMode | undefined) ?? 'hybrid'
  const configuredMode = runtimeConfig.config.agent.mode

  if (routeMode !== 'hybrid' && configuredMode !== 'hybrid' && configuredMode !== routeMode) {
    return { name: homeRoute }
  }
  if (routeMode === 'agent' || routeMode === 'cloud') {
    runtimeConfig.switchMode(routeMode)
  }

  const authWhitelistRoutes =
    runtimeConfig.view.mode === 'agent' ? agentAuthWhitelistRoutes : cloudAuthWhitelistRoutes
  if (authWhitelistRoutes.includes(routeName)) {
    return
  }

  await gateway.initializeAuth()
  if (!gateway.authenticated) {
    if (runtimeConfig.view.mode === 'agent') {
      return { name: homeRoute }
    }
    return {
      name: 'cloud-login',
      query: { redirect: to.fullPath },
    }
  }
})

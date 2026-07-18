import { createRouter, createWebHistory } from 'vue-router'
import type { LocalMode } from '../config'
import { useCloudAuthStore } from '../store/cloudAuth'
import { useRuntimeConfigStore } from '../store/runtimeConfig'

const cloudAccountAuthRoutes = [
  'cloud-login',
  'cloud-register',
  'cloud-verify-email',
  'cloud-forgot-password',
  'cloud-external-oauth-callback',
  'cloud-external-github-callback',
  'cloud-oauth-authorize',
  'cloud-reset-password',
]

const homeRoute = 'home'

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
      path: '/oauth2/google/callback',
      name: 'cloud-external-oauth-callback',
      component: () => import('../views/cloud/ExternalOAuthCallbackView.vue'),
      props: { provider: 'google' },
      meta: { mode: 'cloud' },
    },
    {
      path: '/oauth2/github/callback',
      name: 'cloud-external-github-callback',
      component: () => import('../views/cloud/ExternalOAuthCallbackView.vue'),
      props: { provider: 'github' },
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
      path: '/change-password',
      name: 'cloud-change-password',
      component: () => import('../views/cloud/ChangePasswordView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/local/connect',
      name: 'local-connect',
      component: () => import('../views/local/ConnectView.vue'),
      meta: { mode: 'local' },
    },
    {
      path: '/oauth/callback',
      name: 'local-oauth-callback',
      component: () => import('../views/local/OAuthCallbackView.vue'),
      meta: { mode: 'local' },
    },
    {
      path: '/sessions',
      name: 'local-sessions',
      component: () => import('../views/local/SessionsView.vue'),
      meta: { mode: 'local' },
    },
    {
      path: '/workspaces/:workspaceId/code',
      name: 'local-workspace-code',
      component: () => import('../views/local/WorkspaceCodeView.vue'),
      meta: { mode: 'local' },
    },
    {
      path: '/workspaces/:workspaceId/files',
      name: 'local-workspace-files',
      component: () => import('../views/local/WorkspaceFilesView.vue'),
      meta: { mode: 'local' },
    },
    {
      path: '/workspaces/:workspaceId/git',
      name: 'local-workspace-git',
      component: () => import('../views/local/WorkspaceGitView.vue'),
      meta: { mode: 'local' },
    },
    {
      path: '/shortcuts',
      name: 'local-shortcuts',
      component: () => import('../views/local/ShortcutsView.vue'),
      meta: { mode: 'local' },
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
      path: '/devices/:deviceId/workspaces/:workspaceId/code',
      name: 'cloud-workspace-code',
      component: () => import('../views/cloud/WorkspaceCodeView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/devices/:deviceId/workspaces/:workspaceId/files',
      name: 'cloud-workspace-files',
      component: () => import('../views/cloud/WorkspaceFilesView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/devices/:deviceId/workspaces/:workspaceId/git',
      name: 'cloud-workspace-git',
      component: () => import('../views/cloud/WorkspaceGitView.vue'),
      meta: { mode: 'cloud' },
    },
    {
      path: '/devices/:deviceId/shortcuts',
      name: 'cloud-shortcuts',
      component: () => import('../views/cloud/ShortcutsView.vue'),
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
  const cloudAuth = useCloudAuthStore()
  const runtimeConfig = useRuntimeConfigStore()
  const routeName = to.name as string
  const routeMode = (to.meta.mode as LocalMode | undefined) ?? 'hybrid'
  const configuredMode = runtimeConfig.config.local.mode

  if (routeMode !== 'hybrid' && configuredMode !== 'hybrid' && configuredMode !== routeMode) {
    return { name: homeRoute }
  }
  if (routeMode === 'local' || routeMode === 'cloud') {
    runtimeConfig.switchMode(routeMode)
  }

  if (runtimeConfig.view.mode === 'local' || cloudAuthWhitelistRoutes.includes(routeName)) {
    return
  }

  await cloudAuth.initialize()
  if (!cloudAuth.authenticated) {
    return {
      name: 'cloud-login',
      query: { redirect: to.fullPath },
    }
  }
  if (routeName === 'cloud-change-password' && cloudAuth.user?.provider !== 'email') {
    return { name: homeRoute }
  }
})

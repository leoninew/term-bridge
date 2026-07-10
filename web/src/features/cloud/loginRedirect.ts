import {
  readLocalStorageValue,
  removeLocalStorageValue,
  writeLocalStorageValue,
} from '../../store/storage'

export const cloudLoginRedirectKey = 'termbridge.cloud.login_redirect'

export function storeCloudLoginRedirect(value: string | null) {
  const redirect = safeCloudLoginRedirect(value)
  if (redirect) {
    writeLocalStorageValue(cloudLoginRedirectKey, redirect)
    return
  }
  removeLocalStorageValue(cloudLoginRedirectKey)
}

export function consumeCloudLoginRedirect(): string {
  const redirect = safeCloudLoginRedirect(readLocalStorageValue(cloudLoginRedirectKey))
  removeLocalStorageValue(cloudLoginRedirectKey)
  return redirect
}

export function authenticatedCloudLoginRedirect(
  authenticated: boolean,
  value: string | null,
): string {
  if (!authenticated) {
    return ''
  }
  return safeCloudLoginRedirect(value)
}

export function safeCloudLoginRedirect(value: string | null): string {
  return value && value.startsWith('/') && !value.startsWith('//') ? value : ''
}

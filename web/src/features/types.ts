import type {
  AuthMeResp as ProtoAuthMeResp,
  CloudOAuthAuthorizeResp,
  CloudOAuthCallbackResp as ProtoCloudOAuthCallbackResp,
  CloudOAuthStartResp,
  CloudSessionSummary as ProtoCloudSessionSummary,
  DeviceSummary as ProtoDeviceSummary,
  ListDevicesResp as ProtoListDevicesResp,
  TokenResp,
  User as ProtoUser,
} from '../gen/proto/termbridge/cloud/v1/cloud'

export type DeviceSummary = Omit<ProtoDeviceSummary, 'connected_at' | 'last_seen'> & {
  connected_at: string
  last_seen: string
}

export type CloudSessionSummary = Omit<ProtoCloudSessionSummary, 'connected_at'> & {
  connected_at: string
}

export type UserInfo = ProtoUser & {
  // proto 未覆盖 provider；待补充 cloud User proto 字段后迁移。
  provider: string
}

export type AuthCapabilities = {
  // proto 未覆盖 capabilities；待补充 AuthMeResp proto 字段后迁移。
  providers: string[]
  password_reset_enabled: boolean
  email_verification_enabled: boolean
  account_auth_enabled: boolean
  cloud_oauth_enabled: boolean
}

export type AuthMeResp = Omit<ProtoAuthMeResp, 'user' | 'cloud_session'> & {
  user?: UserInfo
  capabilities?: AuthCapabilities
  cloud_session?: CloudSessionSummary | null
}

export type CloudOAuthCallbackResp = Omit<ProtoCloudOAuthCallbackResp, 'cloud_session'> & {
  cloud_session: CloudSessionSummary
}

export type ListDevicesResp = Omit<ProtoListDevicesResp, 'items'> & {
  items: DeviceSummary[]
}

export type GoogleAuthURLResp = {
  // proto 未覆盖 Google auth URL HTTP 响应；待补充 proto 消息后迁移。
  auth_url: string
}

export type { CloudOAuthAuthorizeResp, CloudOAuthStartResp, TokenResp }

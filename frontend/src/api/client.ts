import type {
  AccessTestResult,
  AppConfig,
  CacheStats,
  LoginResponse,
  MeResponse,
  PackageDetail,
  PackageListResponse,
  PrefetchResponse,
  QueueResponse,
  SystemStats,
} from '@/types/api'

export interface PublicGuideModule {
  id: string
  enabled: boolean
}

export interface PublicGuide {
  proxy_port: string
  modules: PublicGuideModule[]
}

const TOKEN_KEY = 'mirrorhub_session'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

/** 公开页：会话失效时只清 token，不硬跳登录 */
function isPublicLocation(pathname: string): boolean {
  return (
    pathname === '/' ||
    pathname === '/guide' ||
    pathname.startsWith('/guide/') ||
    pathname === '/login' ||
    pathname.startsWith('/login/')
  )
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {})
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const res = await fetch(path, { ...init, headers })
  if (res.status === 401 && !path.includes('/login')) {
    clearToken()
    const publicAPI = path.includes('/api/v1/public/')
    if (!publicAPI && !isPublicLocation(location.pathname)) {
      location.href = '/login'
    }
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || res.statusText)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResponse>('/api/v1/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  getPublicGuide: () => request<PublicGuide>('/api/v1/public/guide'),
  logout: () => request<{ ok: boolean }>('/api/v1/logout', { method: 'POST' }),
  me: () => request<MeResponse>('/api/v1/me'),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ ok: boolean }>('/api/v1/password', {
      method: 'PUT',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    }),
  getConfig: () => request<AppConfig>('/api/v1/config'),
  putConfig: (body: unknown) =>
    request<void>('/api/v1/config', { method: 'PUT', body: JSON.stringify(body) }),
  testAccess: (body: unknown) =>
    request<AccessTestResult>('/api/v1/config/test', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  getStats: () => request<SystemStats>('/api/v1/stats'),
  getQueue: () => request<QueueResponse>('/api/v1/queue'),
  getCache: () => request<CacheStats>('/api/v1/cache'),
  clearCache: () => request<void>('/api/v1/cache', { method: 'DELETE' }),
  listPackages: (params?: {
    q?: string
    page?: number
    page_size?: number
    platform?: string
  }) => {
    const sp = new URLSearchParams()
    if (params?.q) sp.set('q', params.q)
    if (params?.page) sp.set('page', String(params.page))
    if (params?.page_size) sp.set('page_size', String(params.page_size))
    if (params?.platform) sp.set('platform', params.platform)
    const qs = sp.toString()
    return request<PackageListResponse>(`/api/v1/packages${qs ? `?${qs}` : ''}`)
  },
  getPackage: (name: string, opts?: { upstream?: boolean; platform?: string }) => {
    const sp = new URLSearchParams()
    sp.set('name', name)
    sp.set('upstream', opts?.upstream === false ? '0' : '1')
    if (opts?.platform) sp.set('platform', opts.platform)
    return request<PackageDetail>(`/api/v1/packages/detail?${sp.toString()}`)
  },
  deletePackageEntry: (key: string) =>
    request<void>(`/api/v1/packages/entry?key=${encodeURIComponent(key)}`, { method: 'DELETE' }),
  prefetch: (urls: string[], text?: string) =>
    request<PrefetchResponse>('/api/v1/prefetch', {
      method: 'POST',
      body: JSON.stringify({ urls, text: text || undefined }),
    }),
  cancelPrefetch: (id: string) =>
    request<void>(`/api/v1/prefetch?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  clearQueue: () => request<{ cleared: number }>('/api/v1/queue', { method: 'DELETE' }),
}

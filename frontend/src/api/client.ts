import { useAuthStore } from '../stores/auth'
import { createRefreshCoordinator } from './refresh-session'
import type { TokenResponse } from './types'

export class ApiError extends Error {
  status: number
  payload?: unknown

  constructor(message: string, status: number, payload?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

type ApiErrorBody = { error?: string }

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

type RequestOptions = {
  authRequired?: boolean
  skipRefresh?: boolean
  omitAuth?: boolean
}

async function readResponse(res: Response) {
  const text = await res.text()
  if (!text) return null

  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function getErrorMessage(data: unknown, status: number) {
  return data && typeof data === 'object' && (data as ApiErrorBody).error
    ? String((data as ApiErrorBody).error)
    : `请求失败 (${status})`
}

async function requestRefreshToken(refreshToken: string): Promise<TokenResponse> {
  const res = await fetch(`${API_BASE}/account/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })

  const data = await readResponse(res)
  if (!res.ok) {
    throw new ApiError(getErrorMessage(data, res.status), res.status, data)
  }

  return data as TokenResponse
}

const refreshAccessToken = createRefreshCoordinator(requestRefreshToken)

async function tryRefreshToken() {
  const auth = useAuthStore()
  return refreshAccessToken({
    getRefreshToken: () => auth.refreshToken,
    setTokenPair: (pair) => auth.setTokenPair(pair),
    clearToken: () => auth.clearToken(),
  })
}

async function requestWithAuthRetry<T>(
  path: string,
  init: { headers: Record<string, string>; body: BodyInit | null },
  options?: RequestOptions,
): Promise<T> {
  const auth = useAuthStore()

  let token = options?.omitAuth ? null : auth.token
  if (options?.authRequired && !token) {
    token = await tryRefreshToken()
    if (!token) {
      throw new ApiError('需要先登录（缺少 token）', 401)
    }
  }

  const headers: Record<string, string> = { ...init.headers }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: init.body,
  })

  const data = await readResponse(res)
  if (res.status === 401 && !options?.skipRefresh && !options?.omitAuth && auth.refreshToken) {
    const refreshedToken = await tryRefreshToken()
    if (refreshedToken) {
      return requestWithAuthRetry<T>(path, init, { ...options, skipRefresh: true })
    }
    if (!options?.authRequired) {
      return requestWithAuthRetry<T>(path, init, { ...options, skipRefresh: true, omitAuth: true })
    }
  }

  if (!res.ok) {
    if (res.status === 401) {
      auth.clearToken()
    }
    throw new ApiError(getErrorMessage(data, res.status), res.status, data)
  }

  return data as T
}

export async function postJson<T>(path: string, body: unknown, options?: { authRequired?: boolean }): Promise<T> {
  return requestWithAuthRetry<T>(
    path,
    {
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body ?? {}),
    },
    options,
  )
}

export async function postForm<T>(path: string, body: FormData, options?: { authRequired?: boolean }): Promise<T> {
  return requestWithAuthRetry<T>(path, { headers: {}, body }, options)
}

import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type { TokenResponse } from '../api/types'
import { decodeJwtPayload, type JwtPayload } from '../utils/jwt'

const LEGACY_TOKEN_KEY = 'jwt_token'
const ACCESS_TOKEN_KEY = 'jwt_access_token'
const REFRESH_TOKEN_KEY = 'jwt_refresh_token'

function readAccessToken(): string | null {
  try {
    return localStorage.getItem(ACCESS_TOKEN_KEY) ?? localStorage.getItem(LEGACY_TOKEN_KEY)
  } catch {
    return null
  }
}

function readRefreshToken(): string | null {
  try {
    return localStorage.getItem(REFRESH_TOKEN_KEY)
  } catch {
    return null
  }
}

function writeTokenPair(pair: TokenResponse) {
  localStorage.setItem(ACCESS_TOKEN_KEY, pair.access_token)
  localStorage.setItem(REFRESH_TOKEN_KEY, pair.refresh_token)
  localStorage.removeItem(LEGACY_TOKEN_KEY)
}

function removeTokenPair() {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
  localStorage.removeItem(LEGACY_TOKEN_KEY)
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(readAccessToken())
  const refreshToken = ref<string | null>(readRefreshToken())

  const isLoggedIn = computed(() => !!token.value)
  const claims = computed<JwtPayload | null>(() => (token.value ? decodeJwtPayload(token.value) : null))

  function setTokenPair(pair: TokenResponse) {
    token.value = pair.access_token
    refreshToken.value = pair.refresh_token
    writeTokenPair(pair)
  }

  function clearToken() {
    token.value = null
    refreshToken.value = null
    removeTokenPair()
  }

  function syncFromStorage() {
    token.value = readAccessToken()
    refreshToken.value = readRefreshToken()
  }

  return { token, refreshToken, isLoggedIn, claims, setTokenPair, clearToken, syncFromStorage }
})

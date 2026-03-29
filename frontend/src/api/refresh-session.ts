import type { TokenResponse } from './types'

export type RefreshAuthState = {
  getRefreshToken: () => string | null
  setTokenPair: (pair: TokenResponse) => void
  clearToken: () => void
}

export function createRefreshCoordinator(requestRefresh: (refreshToken: string) => Promise<TokenResponse>) {
  let inFlight: Promise<string | null> | null = null

  return async function refresh(auth: RefreshAuthState): Promise<string | null> {
    const refreshToken = auth.getRefreshToken()
    if (!refreshToken) {
      auth.clearToken()
      return null
    }

    if (!inFlight) {
      inFlight = (async () => {
        try {
          const pair = await requestRefresh(refreshToken)
          auth.setTokenPair(pair)
          return pair.access_token
        } catch {
          auth.clearToken()
          return null
        } finally {
          inFlight = null
        }
      })()
    }

    return inFlight
  }
}

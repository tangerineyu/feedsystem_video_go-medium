import test from 'node:test'
import assert from 'node:assert/strict'

import { createRefreshCoordinator } from '../src/api/refresh-session.ts'

test('deduplicates concurrent refresh requests and stores the returned token pair', async () => {
  let refreshCalls = 0
  const writes: Array<{ access_token: string; refresh_token: string }> = []
  const clears: string[] = []

  const refresh = createRefreshCoordinator(async (refreshToken) => {
    refreshCalls += 1
    assert.equal(refreshToken, 'refresh-1')
    await Promise.resolve()
    return {
      access_token: 'access-2',
      refresh_token: 'refresh-2',
    }
  })

  const auth = {
    getRefreshToken: () => 'refresh-1',
    setTokenPair: (pair: { access_token: string; refresh_token: string }) => {
      writes.push(pair)
    },
    clearToken: () => {
      clears.push('clear')
    },
  }

  const [first, second] = await Promise.all([refresh(auth), refresh(auth)])

  assert.equal(first, 'access-2')
  assert.equal(second, 'access-2')
  assert.equal(refreshCalls, 1)
  assert.deepEqual(writes, [{ access_token: 'access-2', refresh_token: 'refresh-2' }])
  assert.deepEqual(clears, [])
})

test('clears local session when refresh fails', async () => {
  const refresh = createRefreshCoordinator(async () => {
    throw new Error('refresh failed')
  })

  const events: string[] = []
  const result = await refresh({
    getRefreshToken: () => 'refresh-1',
    setTokenPair: () => {
      events.push('set')
    },
    clearToken: () => {
      events.push('clear')
    },
  })

  assert.equal(result, null)
  assert.deepEqual(events, ['clear'])
})

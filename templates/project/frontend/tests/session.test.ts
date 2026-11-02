import assert from 'node:assert/strict'
import test from 'node:test'
import { createSessionActions, registerLocalReset, clearLocalAuth } from '../src/store/session.ts'
import { collectPages } from '../src/utils/pagination.ts'

test('logout calls server first; failure preserves local state', async () => {
  const events: string[] = []
  let failing = true
  const session = createSessionActions({
    revoke: async () => { events.push('backend'); if (failing) throw new Error('offline') },
    rotate: async () => 'token',
    clear: () => { events.push('clear') },
    save: () => {},
    active: () => true,
  })
  await assert.rejects(session.logout(), /offline/)
  assert.deepEqual(events, ['backend'])
  failing = false
  await session.logout()
  assert.deepEqual(events, ['backend', 'backend', 'clear'])
})

test('401 cleanup is local only and cannot recursively call logout', () => {
  let resets = 0
  registerLocalReset(() => { resets++ })
  clearLocalAuth()
  assert.equal(resets, 1)
})

test('refresh completes and saves successor before logout revokes it', async () => {
  const events: string[] = []
  let token = 'old'
  let release: (value: string) => void = () => {}
  const rotated = new Promise<string>((resolve) => { release = resolve })
  const session = createSessionActions({
    revoke: async () => { events.push(`revoke:${token}`) },
    rotate: async () => { events.push('rotate'); return rotated },
    clear: () => { token = ''; events.push('clear') },
    save: (value) => { token = value; events.push('save') },
    active: () => Boolean(token),
  })
  const refresh = session.refresh()
  const logout = session.logout()
  await Promise.resolve()
  assert.deepEqual(events, ['rotate'])
  release('new')
  await Promise.all([refresh, logout])
  assert.deepEqual(events, ['rotate', 'save', 'revoke:new', 'clear'])
  await assert.rejects(session.refresh(), /会话/)
})

test('all-options loading follows bounded pages and fails on incomplete data', async () => {
  const calls: number[][] = []
  const items = await collectPages(async (page, size) => {
    calls.push([page, size])
    return { list: [page], result_total: 3 }
  })
  assert.deepEqual(items, [1, 2, 3])
  assert.deepEqual(calls, [[1, 200], [2, 200], [3, 200]])
  await assert.rejects(collectPages(async () => ({ list: [], result_total: 1 })), /重新加载/)
})

test('all-options loading rejects shifting totals and repeated identities', async () => {
  await assert.rejects(collectPages(async (page) => ({ list: [page], result_total: page === 1 ? 3 : 2 })), /重新加载/)
  await assert.rejects(collectPages(async () => ({ list: [{ id: 1 }], result_total: 2 }), (item) => item.id), /重复/)
})

test('logout before queued refresh prevents successor creation', async () => {
  let active = true
  let rotations = 0
  const session = createSessionActions({
    revoke: async () => {},
    rotate: async () => { rotations++; return 'new' },
    clear: () => { active = false },
    save: () => {},
    active: () => active,
  })
  const logout = session.logout()
  const refresh = session.refresh()
  await logout
  await assert.rejects(refresh, /会话/)
  assert.equal(rotations, 0)
})

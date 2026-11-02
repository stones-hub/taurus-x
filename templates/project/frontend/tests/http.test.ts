import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import axios from 'axios'
import typescript from 'typescript'
import * as session from '../src/store/session.ts'

function evaluateModule(path: string, imports: Record<string, unknown>, globals: Record<string, unknown> = {}) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
  const compiled = typescript.transpileModule(source.replace('import.meta.env.VITE_API_BASE_URL', 'undefined'), {
    compilerOptions: { module: typescript.ModuleKind.CommonJS, target: typescript.ScriptTarget.ES2022, esModuleInterop: true },
  }).outputText
  const module = { exports: {} as Record<string, any> }
  runInNewContext(compiled, {
    module, exports: module.exports,
    require: (name: string) => {
      if (!(name in imports)) throw new Error(`Unexpected import: ${name}`)
      return imports[name]
    },
    ...globals,
  })
  return module.exports
}

function httpHarness(code: number, transportFailure = false, cookie = 'x-csrf-token=csrf-test-marker') {
  const calls: { url?: string; method?: string; authorization?: unknown; csrf?: unknown }[] = []
  const messages: unknown[] = []
  let storedToken: string | null = 'local-session-marker'
  let resets = 0
  const localStorage = {
    getItem: () => storedToken,
    removeItem: () => { storedToken = null },
  }
  session.registerLocalReset(() => { resets++; localStorage.removeItem() })
  const mockAxios = {
    create: (options: object) => axios.create({
      ...options,
      adapter: async (config) => {
        calls.push({ url: config.url, method: config.method, authorization: config.headers.Authorization, csrf: config.headers['x-csrf-token'] })
        const response = { data: { code, data: { message: 'simulated response' } }, status: transportFailure ? code : 200, statusText: '', headers: {}, config }
        if (transportFailure) throw new axios.AxiosError('simulated transport response', 'ERR_BAD_REQUEST', config, undefined, response)
        return response
      },
    }),
  }
  const http = evaluateModule('../src/api/http.ts', {
    axios: mockAxios,
    'element-plus': { ElMessage: (message: unknown) => { messages.push(message) } },
    '../store/session': session,
  }, { localStorage, document: { cookie }, window: { location: { pathname: '/home', href: '' } } })
  const auth = evaluateModule('../src/api/auth.ts', { './http': http })
  return { http, auth, calls, messages, resets: () => resets, token: () => storedToken }
}

for (const transportFailure of [false, true]) {
  test(`401 clears local state without logout recursion (transport=${transportFailure})`, async () => {
    const harness = httpHarness(401, transportFailure)
    await assert.rejects(harness.http.request({ url: '/admin/user/current-info', method: 'get' }))
    assert.equal(harness.resets(), 1)
    assert.equal(harness.token(), null)
    assert.equal(harness.calls.length, 1)
    assert.equal(harness.calls[0].url, '/admin/user/current-info')
    assert.equal(harness.messages.length, 1)
  })

  test(`logout rejection preserves local state and sends POST (transport=${transportFailure})`, async () => {
    const harness = httpHarness(401, transportFailure)
    await assert.rejects(harness.auth.apiLogout())
    assert.equal(harness.resets(), 0)
    assert.equal(harness.token(), 'local-session-marker')
    assert.equal(harness.calls.length, 1)
    assert.equal(harness.calls[0].url, '/auth/logout')
    assert.equal(harness.calls[0].method, 'post')
    assert.equal(harness.messages.length, 1)
  })
}

test('successful logout API reaches backend adapter before coordinated local clearing', async () => {
  const harness = httpHarness(200)
  let cleared = false
  const actions = session.createSessionActions({
    revoke: harness.auth.apiLogout,
    rotate: async () => 'unused',
    save: () => {},
    active: () => true,
    clear: () => { assert.equal(harness.calls.length, 1); cleared = true },
  })
  await actions.logout()
  assert.equal(cleared, true)
  assert.equal(harness.calls[0].url, '/auth/logout')
})

test('expired cookies logout sends retained Bearer without CSRF and clears local state after success', async () => {
  const harness = httpHarness(200, false, '')
  const actions = session.createSessionActions({
    revoke: harness.auth.apiLogout,
    rotate: async () => 'unused',
    save: () => {},
    active: () => Boolean(harness.token()),
    clear: session.clearLocalAuth,
  })
  assert.equal(harness.token(), 'local-session-marker')
  await actions.logout()
  assert.equal(harness.calls.length, 1)
  assert.equal(harness.calls[0].url, '/auth/logout')
  assert.equal(harness.calls[0].method, 'post')
  assert.equal(harness.calls[0].authorization, 'Bearer local-session-marker')
  assert.equal(harness.calls[0].csrf, undefined)
  assert.equal(harness.token(), null)
  assert.equal(harness.resets(), 1)
  assert.equal(harness.messages.length, 0)
})

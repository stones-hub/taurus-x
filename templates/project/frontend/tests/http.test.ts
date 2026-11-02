import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import axios from 'axios'
import typescript from 'typescript'
import * as session from '../src/store/session.ts'
import * as loginReason from '../src/utils/login-reason.ts'

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

/**
 * localStorage 探针：任何读写都记下来。前端不再把令牌放进 localStorage，
 * 请求链路上对它的任何访问都是回归。
 */
function storageProbe() {
  const accesses: string[] = []
  return {
    accesses,
    storage: {
      getItem: (key: string) => { accesses.push(`get:${key}`); return 'legacy-token-marker' },
      setItem: (key: string) => { accesses.push(`set:${key}`) },
      removeItem: (key: string) => { accesses.push(`remove:${key}`) },
    },
  }
}

function httpHarness(code: number, transportFailure = false, cookie = 'x-csrf-token=csrf-test-marker') {
  const calls: { url?: string; method?: string; authorization?: unknown; csrf?: unknown }[] = []
  const messages: unknown[] = []
  const document = { cookie }
  const { storage, accesses } = storageProbe()
  const hint = evaluateModule('../src/utils/auth-hint.ts', {}, { document, localStorage: storage })
  let resets = 0
  // 与 store/auth.ts 的 clearLocal 一致：本地清理包括删掉登录提示（CSRF Cookie）
  session.registerLocalReset(() => { resets++; hint.clearSessionHint() })
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
  const location = { pathname: '/home', href: '' }
  const http = evaluateModule('../src/api/http.ts', {
    axios: mockAxios,
    'element-plus': { ElMessage: (message: unknown) => { messages.push(message) } },
    '../store/session': session,
    '../utils/auth-hint': hint,
    '../utils/login-reason': loginReason,
  }, { localStorage: storage, document, window: { location } })
  const auth = evaluateModule('../src/api/auth.ts', { './http': http })
  return {
    http, auth, calls, messages, location, storageAccesses: accesses,
    resets: () => resets,
    loggedIn: () => hint.hasSessionHint(document.cookie),
  }
}

for (const transportFailure of [false, true]) {
  test(`401 clears local state without logout recursion (transport=${transportFailure})`, async () => {
    const harness = httpHarness(401, transportFailure)
    assert.equal(harness.loggedIn(), true)
    await assert.rejects(harness.http.request({ url: '/admin/user/current-info', method: 'get' }))
    assert.equal(harness.resets(), 1)
    assert.equal(harness.loggedIn(), false)
    assert.equal(harness.calls.length, 1)
    assert.equal(harness.calls[0].url, '/admin/user/current-info')
    assert.equal(harness.messages.length, 1)
    assert.equal((harness.messages[0] as { message: string }).message, '登录已过期，请重新登录')
    assert.equal((harness.messages[0] as { grouping: boolean }).grouping, true)
    assert.equal(harness.location.href, '/login?reason=expired')
    assert.deepEqual(harness.storageAccesses, [])
  })

  test(`logout rejection preserves local state and sends POST (transport=${transportFailure})`, async () => {
    const harness = httpHarness(401, transportFailure)
    await assert.rejects(harness.auth.apiLogout())
    assert.equal(harness.resets(), 0)
    assert.equal(harness.loggedIn(), true)
    assert.equal(harness.calls.length, 1)
    assert.equal(harness.calls[0].url, '/auth/logout')
    assert.equal(harness.calls[0].method, 'post')
    assert.equal(harness.messages.length, 1)
    assert.equal(harness.location.href, '')
  })

  test(`login 401 shows the server message without clearing state or redirect (transport=${transportFailure})`, async () => {
    const harness = httpHarness(401, transportFailure)
    await assert.rejects(harness.auth.apiLogin({ username: 'policy-user', password: 'wrong-password' }))
    assert.equal(harness.calls.length, 1)
    assert.equal(harness.calls[0].url, '/auth/login')
    assert.equal(harness.resets(), 0)
    assert.equal(harness.loggedIn(), true)
    assert.equal(harness.messages.length, 1)
    assert.equal((harness.messages[0] as { message: string }).message, 'simulated response')
    assert.equal(harness.location.href, '')
  })

  test(`non-401 errors show the server message without redirect (transport=${transportFailure})`, async () => {
    const harness = httpHarness(500, transportFailure)
    await assert.rejects(harness.http.request({ url: '/admin/user/list', method: 'get' }))
    assert.equal(harness.resets(), 0)
    assert.equal(harness.loggedIn(), true)
    assert.equal(harness.messages.length, 1)
    assert.equal((harness.messages[0] as { message: string }).message, 'simulated response')
    assert.equal(harness.location.href, '')
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

// 原用例是「Cookie 过期后退出带着 localStorage 里的 Bearer」：现在没有 Bearer 了，
// 同一场景下退出请求既不带 Authorization 也不带 CSRF 头，成功后清掉本地登录态，且不碰 localStorage。
test('expired cookies logout sends no Authorization or CSRF and clears local state after success', async () => {
  const harness = httpHarness(200, false, '')
  const actions = session.createSessionActions({
    revoke: harness.auth.apiLogout,
    rotate: async () => 'unused',
    save: () => {},
    active: () => harness.loggedIn(),
    clear: session.clearLocalAuth,
  })
  await actions.logout()
  assert.equal(harness.calls.length, 1)
  assert.equal(harness.calls[0].url, '/auth/logout')
  assert.equal(harness.calls[0].method, 'post')
  assert.equal(harness.calls[0].authorization, undefined)
  assert.equal(harness.calls[0].csrf, undefined)
  assert.equal(harness.loggedIn(), false)
  assert.equal(harness.resets(), 1)
  assert.equal(harness.messages.length, 0)
  assert.deepEqual(harness.storageAccesses, [])
})

test('requests never carry Authorization and never touch localStorage; writes carry the CSRF header', async () => {
  const harness = httpHarness(200)
  await harness.http.request({ url: '/admin/user/list', method: 'get' })
  await harness.http.request({ url: '/admin/user/add', method: 'post', data: {} })
  await harness.auth.apiLogin({ username: 'policy-user', password: 'policy-password' })
  await harness.auth.apiRefresh()
  await harness.auth.apiLogout()
  assert.equal(harness.calls.length, 5)
  for (const call of harness.calls) {
    assert.equal(call.authorization, undefined, `${call.url} must not send Authorization`)
  }
  assert.equal(harness.calls[0].csrf, undefined)
  for (const call of harness.calls.slice(1)) {
    assert.equal(call.csrf, 'csrf-test-marker', `${call.url} must send the CSRF header`)
  }
  assert.deepEqual(harness.storageAccesses, [])
})

test('session-exempt URLs match the literal URLs used by api/auth.ts', async () => {
  const harness = httpHarness(200)
  const exempt: ReadonlySet<string> = harness.http.SESSION_EXEMPT_URLS
  await harness.auth.apiLogin({ username: 'policy-user', password: 'policy-password' })
  await harness.auth.apiLogout()
  const [login, logout] = harness.calls
  assert.ok(exempt.has(login.url ?? ''), `apiLogin url ${login.url} must be session-exempt`)
  assert.ok(exempt.has(logout.url ?? ''), `apiLogout url ${logout.url} must be session-exempt`)
  // 只有这两个：其余接口的 401 都必须按会话失效处理
  assert.deepEqual([...exempt].sort(), [login.url, logout.url].sort())
})

test('legacy localStorage token is purged and the session hint follows the CSRF cookie', () => {
  const { storage, accesses } = storageProbe()
  const document = { cookie: 'other=1; x-csrf-token=abc' }
  const hint = evaluateModule('../src/utils/auth-hint.ts', {}, { document, localStorage: storage })
  hint.purgeLegacyToken()
  assert.deepEqual(accesses, ['remove:token'])
  assert.equal(hint.hasSessionHint(), true)
  assert.equal(hint.hasSessionHint('other=1'), false)
  assert.equal(hint.hasSessionHint('not-x-csrf-token=abc'), false)
  hint.clearSessionHint()
  assert.match(document.cookie, /^x-csrf-token=; Max-Age=0; path=\/$/)
  assert.equal(hint.hasSessionHint(), false)
})

test('frontend sources never read or write a token in localStorage', () => {
  const files = [
    '../src/api/http.ts', '../src/api/auth.ts', '../src/store/auth.ts', '../src/store/session.ts',
    '../src/router/index.ts', '../src/main.ts', '../src/views/login/index.vue', '../src/views/profile/index.vue',
    '../src/views/home/index.vue', '../src/utils/relogin.ts', '../src/utils/login-reason.ts',
  ]
  for (const file of files) {
    const source = readFileSync(new URL(file, import.meta.url), 'utf8')
    assert.doesNotMatch(source, /localStorage\.(getItem|setItem)\(/, `${file} must not read or write localStorage`)
    assert.doesNotMatch(source, /Bearer/, `${file} must not build a Bearer header`)
  }
  // 唯一允许碰 localStorage 的地方：启动时删除旧令牌
  const hint = readFileSync(new URL('../src/utils/auth-hint.ts', import.meta.url), 'utf8')
  assert.doesNotMatch(hint, /localStorage\.(getItem|setItem)\(/)
})

import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import typescript from 'typescript'
import { LOGIN_REASONS, loginPath, takeLoginReason } from '../src/utils/login-reason.ts'

function historyProbe() {
  const replaced: { state: unknown; url: string }[] = []
  return {
    replaced,
    history: {
      state: { routerPosition: 3 },
      replaceState(state: unknown, _unused: string, url?: string | URL | null) { replaced.push({ state, url: String(url) }) },
    },
  }
}

test('login reason: only fixed values show a message, the parameter is always stripped', () => {
  for (const [reason, message] of Object.entries(LOGIN_REASONS)) {
    const { history, replaced } = historyProbe()
    assert.equal(takeLoginReason({ pathname: '/login', search: `?reason=${reason}`, hash: '' }, history), message)
    assert.deepEqual(replaced, [{ state: { routerPosition: 3 }, url: '/login' }])
  }
  for (const search of ['?reason=<script>alert(1)</script>', '?reason=', '?reason=toString', '?reason=__proto__']) {
    const { history, replaced } = historyProbe()
    assert.equal(takeLoginReason({ pathname: '/login', search, hash: '' }, history), null, search)
    assert.equal(replaced.length, 1, `${search} must still be stripped`)
    assert.equal(replaced[0].url, '/login')
  }
})

test('login reason: no parameter means no message and no history change; other params survive', () => {
  const { history, replaced } = historyProbe()
  assert.equal(takeLoginReason({ pathname: '/login', search: '', hash: '' }, history), null)
  assert.equal(replaced.length, 0)
  const second = historyProbe()
  assert.equal(takeLoginReason({ pathname: '/login', search: '?a=1&reason=expired&b=2', hash: '#x' }, second.history), LOGIN_REASONS.expired)
  assert.equal(second.replaced[0].url, '/login?a=1&b=2#x')
})

test('login reason paths used by redirects', () => {
  assert.equal(loginPath('expired'), '/login?reason=expired')
  assert.equal(loginPath('password_changed'), '/login?reason=password_changed')
})

function loadRelogin() {
  const source = readFileSync(new URL('../src/utils/relogin.ts', import.meta.url), 'utf8')
  const compiled = typescript.transpileModule(source, {
    compilerOptions: { module: typescript.ModuleKind.CommonJS, target: typescript.ScriptTarget.ES2022 },
  }).outputText
  const module = { exports: {} as Record<string, any> }
  runInNewContext(compiled, {
    module, exports: module.exports,
    require: (name: string) => {
      if (name !== './login-reason') throw new Error(`Unexpected import: ${name}`)
      return { loginPath }
    },
  })
  return module.exports
}

test('password change relogin clears local state before navigating to the login page with a reason', async () => {
  const { reloginAfterPasswordChange } = loadRelogin()
  const events: string[] = []
  let release: () => void = () => {}
  const navigation = new Promise<void>((resolve) => { release = resolve })
  const done = reloginAfterPasswordChange({
    clear: () => { events.push('clear') },
    navigate: (path: string) => { events.push(`navigate:${path}`); return navigation },
  })
  assert.deepEqual(events, ['clear', 'navigate:/login?reason=password_changed'])
  let settled = false
  void done.then(() => { settled = true })
  await Promise.resolve()
  assert.equal(settled, false, 'must wait for the navigation to finish')
  release()
  await done
  assert.equal(settled, true)
})

test('password change relogin does not navigate when local clearing throws', async () => {
  const { reloginAfterPasswordChange } = loadRelogin()
  let navigated = false
  await assert.rejects(reloginAfterPasswordChange({
    clear: () => { throw new Error('clear failed') },
    navigate: () => { navigated = true },
  }), /clear failed/)
  assert.equal(navigated, false)
})

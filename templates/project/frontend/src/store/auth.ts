// 认证状态（Pinia）。
//
// 不保存令牌：JWT 只在 HttpOnly Cookie 里，前端读不到。这里只保存当前用户信息；
// 「是否已登录」看 utils/auth-hint.ts 的 hasSessionHint（CSRF Cookie 是否存在），
// 最终以服务端 401 为准。
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiLogin, apiMe, apiLogout, apiRefresh } from '../api/auth'
import { createSessionActions, registerLocalReset } from './session'
import { usePermissionStore } from './permission'
import { clearSessionHint, hasSessionHint } from '../utils/auth-hint'
import type { LoginRequest, UserInfo } from '../types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserInfo | null>(null)

  async function login(payload: LoginRequest) {
    const res = await apiLogin(payload)
    user.value = res.user
    // 登录后加载菜单与按钮权限
    await usePermissionStore().load()
  }

  async function loadMe() {
    try {
      user.value = await apiMe()
    } catch {
      user.value = null
    }
  }

  function clearLocal() {
    user.value = null
    clearSessionHint()
    usePermissionStore().reset()
  }

  registerLocalReset(clearLocal)
  const { logout, refresh } = createSessionActions({
    revoke: apiLogout,
    rotate: apiRefresh,
    clear: clearLocal,
    // 刷新后新令牌与续期后的 CSRF Cookie 由后端通过 Set-Cookie 写入，前端无需保存任何东西。
    save: () => {},
    active: () => hasSessionHint(),
  })

  return { user, login, loadMe, logout, refresh, clearLocal }
})

// 认证状态（Pinia）。
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiLogin, apiMe } from '../api/auth'
import { usePermissionStore } from './permission'
import type { LoginRequest, UserInfo } from '../types'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('token') ?? '')
  const user = ref<UserInfo | null>(null)

  async function login(payload: LoginRequest) {
    const res = await apiLogin(payload)
    token.value = res.token
    localStorage.setItem('token', res.token)
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

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
    usePermissionStore().reset()
  }

  return { token, user, login, loadMe, logout }
})

// 权限状态（Pinia）：菜单树 + 按钮权限编码。
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiGetMenuButtons } from '../api/permission'
import type { MenuNode } from '../types'

export const usePermissionStore = defineStore('permission', () => {
  const menus = ref<MenuNode[]>([])
  const buttons = ref<string[]>([])
  const loaded = ref(false)

  async function load() {
    const res = await apiGetMenuButtons()
    menus.value = res.menus ?? []
    buttons.value = res.buttons ?? []
    loaded.value = true
  }

  function hasButton(code: string): boolean {
    return buttons.value.includes(code)
  }

  function reset() {
    menus.value = []
    buttons.value = []
    loaded.value = false
  }

  return { menus, buttons, loaded, load, hasButton, reset }
})

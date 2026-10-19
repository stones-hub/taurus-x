// 按钮权限指令：v-permission:system.account.permission.add
// 通过指令参数（binding.arg）传权限编码，无需引号。
// 无权限时通过 display 隐藏元素（而非移除节点），并响应权限数据异步加载。
import type { Directive } from 'vue'
import { watchEffect } from 'vue'
import { usePermissionStore } from '../store/permission'

interface PermissionElement extends HTMLElement {
  __permissionStop?: () => void
}

export const permission: Directive<PermissionElement, string> = {
  mounted(el, binding) {
    // 优先取 arg（v-permission:xxx），兼容 value（v-permission="'xxx'"）
    const code = binding.arg || binding.value
    if (!code) return
    const store = usePermissionStore()

    // watchEffect 追踪 buttons 变化：权限数据异步加载完成后自动显示/隐藏
    el.__permissionStop = watchEffect(() => {
      const has = store.hasButton(code)
      el.style.display = has ? '' : 'none'
    })
  },
  unmounted(el) {
    el.__permissionStop?.()
  },
}

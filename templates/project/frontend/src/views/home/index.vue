<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Expand, Fold, SwitchButton } from '@element-plus/icons-vue'
import SidebarMenu from '../../components/SidebarMenu.vue'
import { useAuthStore } from '../../store/auth'
import { usePermissionStore } from '../../store/permission'
import { appInitial, appTitle } from '../../utils/brand'
import type { MenuNode } from '../../types'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const perm = usePermissionStore()

const displayName = computed(() => auth.user?.realname || auth.user?.username || 'Admin')

// 顶栏展示：菜单树根级（一级菜单）
const topMenus = computed(() => perm.menus)

// 当前选中的一级菜单 code
const activeTop = ref('')

// 左侧菜单收起/展开状态
const sidebarCollapsed = ref(false)

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

// 当前一级菜单下的二级菜单
const sidebarMenus = computed<MenuNode[]>(() => {
  const top = topMenus.value.find((m) => m.code === activeTop.value)
  return top?.children ?? []
})

// 当前一级菜单标题
const activeTopName = computed(() => {
  const top = topMenus.value.find((m) => m.code === activeTop.value)
  return top?.name ?? ''
})

// 默认选中第一个一级菜单
watch(
  topMenus,
  (menus) => {
    if (menus.length > 0 && !menus.some((m) => m.code === activeTop.value)) {
      activeTop.value = menus[0].code
    }
  },
  { immediate: true },
)

// 根据当前路由定位一级菜单（打开页面时顶栏菜单聚焦过去）。
// 优先用路由 meta.activeMenu（显式声明的聚焦菜单 link），否则回退到
// 路径匹配 + (-add/-edit).html 后缀归一化（兼容未标注的旧路由）。
function findTopMenuByPath(menus: MenuNode[]): string {
  // 1）路由显式声明了聚焦菜单 → 直接按 link 精确匹配
  const active = (route.meta as { activeMenu?: string }).activeMenu
  if (active) {
    const target = normalizeLink(active)
    for (const top of menus) {
      if (top.link && normalizeLink(top.link) === target) return top.code
      // 递归查叶子
      const findLeaf = (nodes: MenuNode[]): boolean => {
        for (const n of nodes) {
          if (n.link && normalizeLink(n.link) === target) return true
          if (n.children && findLeaf(n.children)) return true
        }
        return false
      }
      if (top.children && findLeaf(top.children)) return top.code
    }
    return ''
  }

  // 2）回退：路径匹配 + -add/-edit 归一化
  const path = route.path
  const candidates = [path]
  const normalized = path.replace(/-(add|edit)\.html$/, '.html')
  if (normalized !== path) {
    candidates.push(normalized)
  }

  for (const top of menus) {
    for (const c of candidates) {
      if (top.link && normalizeLink(top.link) === c) {
        return top.code
      }
    }
    const findLeaf = (nodes: MenuNode[]): boolean => {
      for (const n of nodes) {
        for (const c of candidates) {
          if (n.link && normalizeLink(n.link) === c) return true
        }
        if (n.children && findLeaf(n.children)) return true
      }
      return false
    }
    if (top.children && findLeaf(top.children)) {
      return top.code
    }
  }
  return ''
}

// 路由变化或菜单加载后，自动聚焦到对应的一级菜单
watch(
  [() => route.path, topMenus],
  () => {
    if (!topMenus.value || topMenus.value.length === 0) return
    const matched = findTopMenuByPath(topMenus.value)
    if (matched && matched !== activeTop.value) {
      activeTop.value = matched
    }
  },
  { immediate: true },
)

// 将多页路径（page/xxx.html）规范化为绝对路径后跳转
function normalizeLink(link?: string): string {
  if (!link) return ''
  return link.startsWith('/') ? link : '/' + link
}

function onTopSelect(item: MenuNode) {
  // 一级是叶子（有 link）→ 直接跳转
  if (item.link && (!item.children || item.children.length === 0)) {
    router.push(normalizeLink(item.link))
    return
  }
  // 一级是目录 → 切换侧栏，并自动展开
  activeTop.value = item.code
  sidebarCollapsed.value = false
}

function onLogout() {
  auth.logout()
  router.replace('/login')
}

// 进入个人中心（已有的 profile 路由，不是新页面）
function onProfile() {
  router.push({ name: 'profile' })
}

onMounted(() => {
  if (!auth.user) {
    auth.loadMe()
  }
  if (!perm.loaded) {
    perm.load().catch(() => {})
  }
})
</script>

<template>
  <div class="layout">
    <!-- 顶栏：logo + 一级菜单 + 用户 -->
    <header class="topbar">
      <div class="topbar-brand">
        <span class="brand-mark">{{ appInitial }}</span>
        <span class="brand-text">{{ appTitle }}</span>
        <el-button
          v-if="sidebarMenus.length > 0"
          text
          :icon="sidebarCollapsed ? Expand : Fold"
          :title="sidebarCollapsed ? '展开菜单' : '收起菜单'"
          class="sidebar-toggle-btn"
          @click="toggleSidebar"
        />
      </div>
      <nav class="topnav">
        <div
          v-for="item in topMenus"
          :key="item.code"
          class="topnav-item"
          :class="{ active: item.code === activeTop }"
          @click="onTopSelect(item)"
        >
          {{ item.name }}
        </div>
      </nav>
      <div class="topbar-right">
        <!--
          个人中心入口：复用已有的 profile 路由，不新增页面、不新增菜单权限。
          用原生 <button> 而不是给 <div> 挂 @click —— 这样 Tab 能聚焦、
          Enter/Space 能触发，无障碍与键盘操作都是浏览器默认行为，不用自己补 keydown。
        -->
        <button type="button" class="user-entry" title="个人中心" aria-label="个人中心" @click="onProfile">
          <span class="user-name">{{ displayName }}</span>
          <span class="avatar">{{ displayName.charAt(0).toUpperCase() }}</span>
        </button>
        <el-button text :icon="SwitchButton" @click="onLogout">退出</el-button>
      </div>
    </header>

    <!-- 主体：侧栏二级菜单 + 内容 -->
    <div class="body">
      <aside v-if="!sidebarCollapsed && sidebarMenus.length > 0" class="sidebar">
        <div class="sidebar-title">{{ activeTopName }}</div>
        <nav class="sidebar-menu">
          <SidebarMenu :menus="sidebarMenus" />
        </nav>
      </aside>

      <div class="main">
        <div class="content">
          <router-view />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

/* ===== 顶栏 ===== */
.topbar {
  height: 60px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 0 20px;
  background: #ffffff;
  border-bottom: 1px solid var(--border-color);
}
.topbar-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.sidebar-toggle-btn {
  padding: 6px;
  font-size: 16px;
  color: #4b5563;
}
.sidebar-toggle-btn:hover {
  color: #6366f1;
}
.brand-mark {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  color: #fff;
  border-radius: 9px;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
}
.brand-text {
  font-size: 16px;
  font-weight: 700;
  color: #1f2937;
  letter-spacing: 1px;
}
.topnav {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 4px;
  height: 100%;
  overflow-x: auto;
  scrollbar-width: none;
}
.topnav::-webkit-scrollbar {
  display: none;
}
.topnav-item {
  height: 100%;
  display: flex;
  align-items: center;
  padding: 0 18px;
  font-size: 13px;
  color: #4b5563;
  cursor: pointer;
  white-space: nowrap;
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
}
.topnav-item:hover {
  color: #6366f1;
}
.topnav-item.active {
  color: #6366f1;
  font-weight: 600;
  border-bottom-color: #6366f1;
}
.topbar-right {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
}
/* 个人中心入口：按钮只做「可点可聚焦」的容器，视觉沿用原来的用户名 + 头像 */
.user-entry {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 2px;
  border: none;
  border-radius: 20px;
  background: transparent;
  cursor: pointer;
  font: inherit;
}
.user-entry:hover .user-name {
  color: #6366f1;
}
.user-entry:focus-visible {
  outline: 2px solid #6366f1;
  outline-offset: 2px;
}
.user-name {
  font-size: 13px;
  color: #4b5563;
}
.avatar {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 600;
  color: #fff;
  border-radius: 50%;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
}

/* ===== 主体 ===== */
.body {
  flex: 1;
  display: flex;
  min-height: 0;
}

/* 侧栏 */
.sidebar {
  width: 230px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: #ffffff;
  border-right: 1px solid var(--border-color);
}
.sidebar-title {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 18px;
  font-size: 13px;
  font-weight: 600;
  color: #9ca3af;
  letter-spacing: 1px;
  border-bottom: 1px solid #f3f4f6;
}
.sidebar-menu {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 10px 12px;
}

/* 内容 */
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--content-bg);
}
.content {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
}
</style>

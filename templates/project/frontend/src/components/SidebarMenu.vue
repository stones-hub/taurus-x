<script setup lang="ts">
// 递归侧边栏菜单：无图标，显示层级线，目录可折叠，叶子点击跳转。
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { MenuNode } from '../types'

defineOptions({ name: 'SidebarMenu' })

const props = defineProps<{
  menus: MenuNode[]
  level?: number
}>()

const route = useRoute()
const router = useRouter()

// 折叠状态（默认展开）
const collapsed = ref<Set<string>>(new Set())

function isLeaf(item: MenuNode): boolean {
  return !item.children || item.children.length === 0
}

function isActive(item: MenuNode): boolean {
  if (!item.link) return false
  const target = normalizeLink(item.link)
  // 1）路由显式声明聚焦菜单（meta.activeMenu）→ 精确匹配
  const active = (route.meta as { activeMenu?: string }).activeMenu
  if (active && normalizeLink(active) === target) return true
  // 2）回退：当前路径精确匹配 + 编辑/新增页归一化匹配（xxx-add.html → xxx.html）
  const current = route.path
  if (current === target) return true
  const normalized = current.replace(/-(add|edit)\.html$/, '.html')
  return normalized === target
}

function normalizeLink(link?: string): string {
  if (!link) return ''
  return link.startsWith('/') ? link : '/' + link
}

function onLeafClick(item: MenuNode) {
  if (item.link) {
    router.push(normalizeLink(item.link))
  }
}

// 用节点对象自身作为折叠 key（code 唯一）
function toggleFold(code: string) {
  const next = new Set(collapsed.value)
  if (next.has(code)) next.delete(code)
  else next.add(code)
  collapsed.value = next
}

function indentPx(): number {
  return (props.level ?? 0) * 18
}
</script>

<template>
  <div class="sidebar-tree">
    <template v-for="item in menus" :key="item.code">
      <!-- 目录（可折叠：整行点击） -->
      <div v-if="!isLeaf(item)" class="tree-node">
        <div class="tree-row group-row" :style="{ paddingLeft: indentPx() + 'px' }" @click="toggleFold(item.code)">
          <span class="fold-btn">{{ collapsed.has(item.code) ? '▸' : '▾' }}</span>
          <span class="node-label group">{{ item.name }}</span>
        </div>
        <div v-if="!collapsed.has(item.code)" class="tree-children">
          <SidebarMenu :menus="item.children" :level="(level ?? 0) + 1" />
        </div>
      </div>
      <!-- 叶子（跳转） -->
      <div
        v-else
        class="tree-row leaf"
        :class="{ active: isActive(item) }"
        :style="{ paddingLeft: indentPx() + 18 + 'px' }"
        @click="onLeafClick(item)"
      >
        <span class="node-label">{{ item.name }}</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.sidebar-tree {
  display: flex;
  flex-direction: column;
}
.tree-row {
  display: flex;
  align-items: center;
  height: 38px;
  padding-right: 12px;
  border-radius: 6px;
  cursor: pointer;
  color: #4b5563;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tree-row:hover {
  background: #f3f4f6;
}
.fold-btn {
  flex: 0 0 20px;
  width: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #9ca3af;
  font-size: 14px;
  user-select: none;
}
.node-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-label.group {
  font-weight: 600;
  color: #374151;
}
/* 叶子选中态 */
.tree-row.leaf.active {
  background: #eef2ff;
  color: #6366f1;
  font-weight: 500;
}
/* 层级线：子级容器左侧竖线 */
.tree-children {
  position: relative;
  margin-left: 9px;
  padding-left: 12px;
  border-left: 1px solid #e5e7eb;
}
</style>

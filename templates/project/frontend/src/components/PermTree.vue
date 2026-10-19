<script setup lang="ts">
// 权限树递归组件：父级纵向、叶子横向铺开。
// 缩进用 level 内联计算（非 CSS 嵌套），保证每一层 checkbox 起点精确对齐。
import { ref } from 'vue'
import type { EditRolePermissionTree } from '../types'

const props = defineProps<{
  nodes: EditRolePermissionTree[]
  selectedIds: Set<number>
  disabled?: boolean
  level?: number
}>()

const emit = defineEmits<{
  (e: 'toggle', id: number, checked: boolean): void
}>()

const FOLD_W = 28 // 折叠按钮/占位宽度
const INDENT = 26 // 每层缩进宽度

// 折叠状态（响应式 Set）
const collapsed = ref<Set<number>>(new Set())

function hasChildren(node: EditRolePermissionTree): boolean {
  return Boolean(node.children && node.children.length > 0)
}

function isChecked(id: number): boolean {
  return props.selectedIds.has(id)
}

function isCollapsed(id: number): boolean {
  return collapsed.value.has(id)
}

function toggleFold(id: number) {
  const next = new Set(collapsed.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  collapsed.value = next
}

// 当前层级缩进（px）
function indentPx(): number {
  return (props.level ?? 0) * INDENT
}

// 直接子节点：叶子 与 分支
function splitChildren(node: EditRolePermissionTree) {
  const leaves: EditRolePermissionTree[] = []
  const branches: EditRolePermissionTree[] = []
  for (const c of node.children ?? []) {
    if (hasChildren(c)) branches.push(c)
    else leaves.push(c)
  }
  return { leaves, branches }
}

function typeText(type: number): string {
  return { 1: '菜单', 2: '按钮', 3: '接口' }[type] ?? '未知'
}
function typeClass(type: number): string {
  return { 1: 'tag-menu', 2: 'tag-button', 3: 'tag-api' }[type] ?? ''
}
</script>

<template>
  <div class="perm-node-list">
    <template v-for="node in nodes" :key="node.permission_id">
      <!-- 节点行：内联 padding 精确缩进 -->
      <div class="perm-node-row" :style="{ paddingLeft: indentPx() + 'px' }">
        <button
          type="button"
          class="fold-btn"
          :disabled="!hasChildren(node)"
          @click="toggleFold(node.permission_id)"
        >
          <span v-if="hasChildren(node)" class="fold-icon">{{ isCollapsed(node.permission_id) ? '▸' : '▾' }}</span>
        </button>
        <el-checkbox
          :model-value="isChecked(node.permission_id)"
          :disabled="disabled"
          class="perm-check"
          @change="(v: boolean) => emit('toggle', node.permission_id, v)"
        >
          <span class="perm-type" :class="typeClass(node.permission_type)">{{ typeText(node.permission_type) }}</span>
          <span class="perm-name">{{ node.permission_name }}</span>
        </el-checkbox>
      </div>

      <!-- 子节点（展开时） -->
      <div v-if="hasChildren(node) && !isCollapsed(node.permission_id)">
        <!-- 叶子横向铺开：整块容器统一缩进到下一层，换行后每行起点一致 -->
        <div
          v-if="splitChildren(node).leaves.length"
          class="perm-leaves"
          :style="{ paddingLeft: indentPx() + INDENT + FOLD_W + 'px' }"
        >
          <template v-for="leaf in splitChildren(node).leaves" :key="leaf.permission_id">
            <el-checkbox
              :model-value="isChecked(leaf.permission_id)"
              :disabled="disabled"
              class="perm-leaf"
              @change="(v: boolean) => emit('toggle', leaf.permission_id, v)"
            >
              <span class="perm-type" :class="typeClass(leaf.permission_type)">{{ typeText(leaf.permission_type) }}</span>
              <span class="perm-name">{{ leaf.permission_name }}</span>
            </el-checkbox>
          </template>
        </div>
        <!-- 分支继续递归（level+1） -->
        <PermTree
          v-if="splitChildren(node).branches.length"
          :nodes="splitChildren(node).branches"
          :selected-ids="selectedIds"
          :disabled="disabled"
          :level="(level ?? 0) + 1"
          @toggle="(id: number, c: boolean) => emit('toggle', id, c)"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.perm-node-row {
  display: flex;
  align-items: center;
  min-height: 36px;
}
/* 折叠按钮：button 元素，点击区大 */
.fold-btn {
  flex: 0 0 28px;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-right: 2px;
  padding: 0;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #9ca3af;
  cursor: pointer;
}
.fold-btn:hover:not(:disabled) {
  background: #f0f2f5;
  color: #4b5563;
}
.fold-btn:disabled {
  cursor: default;
  opacity: 0.4;
}
.fold-icon {
  font-size: 14px;
  line-height: 1;
}
/* 叶子横铺：CSS Grid，固定列宽，保证每列上下对齐 */
.perm-leaves {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 4px 12px;
  padding: 4px 0;
}
.perm-leaf {
  margin-right: 0;
  height: auto;
}
/* 让 checkbox 的 label 变成 flex：类型固定、名字可截断 */
.perm-check :deep(.el-checkbox__label),
.perm-leaf :deep(.el-checkbox__label) {
  display: flex;
  align-items: center;
  min-width: 0;
}
.perm-name {
  display: inline-block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.perm-type {
  flex: 0 0 auto;
  margin-right: 6px;
  font-size: 10px;
  line-height: 16px;
  border-radius: 2px;
  padding: 0 5px;
  white-space: nowrap;
  border: 1px solid currentColor;
  background: transparent;
}
.tag-menu {
  color: #409eff;
}
.tag-button {
  color: #e6a23c;
}
.tag-api {
  color: #67c23a;
}
</style>

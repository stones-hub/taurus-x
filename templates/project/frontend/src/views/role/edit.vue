<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'
// 角色新增/编辑
// 权限选择：父级纵向、叶子横向铺开的折叠树，名字后跟小号类型标签。
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiAddRole, apiGetEditRoleInfo, apiUpdateRole } from '../../api/role'
import { apiGetPermissionTree } from '../../api/permission'
import type { EditRolePermissionTree } from '../../types'
import PermTree from '../../components/PermTree.vue'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => Boolean(route.query.role_id))

const submitLoading = ref(false)

const form = reactive({
  role_id: 0,
  role_name: '',
  role_code: '',
  description: '',
  sort_order: 0,
  is_system: 0,
})

// 全量权限树（原始层级结构）
const permissionTree = ref<EditRolePermissionTree[]>([])
// 已选中的权限 ID 集合
const selectedIds = ref<Set<number>>(new Set())

const isSystemLocked = computed(() => isEdit.value && form.is_system === 1)

// 节点 id → 子孙 id 映射（父级勾选联动）
const childrenMap = ref<Map<number, number[]>>(new Map())

function buildChildrenMap(tree: EditRolePermissionTree[]): Map<number, number[]> {
  const map = new Map<number, number[]>()
  const walk = (nodes: EditRolePermissionTree[]): number[] => {
    const ids: number[] = []
    for (const n of nodes) {
      const childIds = n.children?.length ? walk(n.children) : []
      ids.push(n.permission_id, ...childIds)
      map.set(n.permission_id, childIds)
    }
    return ids
  }
  walk(tree)
  return map
}

async function loadPermissionTree() {
  if (isEdit.value) {
    const id = Number(route.query.role_id)
    const res = await apiGetEditRoleInfo(id)
    form.role_id = res.role_id
    form.role_name = res.role_name
    form.role_code = res.role_code
    form.description = res.description
    form.sort_order = res.sort_order
    form.is_system = res.is_system
    permissionTree.value = res.permissions ?? []
    const checked = new Set<number>()
    const walk = (nodes: EditRolePermissionTree[]) => {
      for (const n of nodes) {
        if (n.checked) checked.add(n.permission_id)
        if (n.children?.length) walk(n.children)
      }
    }
    walk(permissionTree.value)
    selectedIds.value = checked
  } else {
    const res = await apiGetPermissionTree({ parent_id: 0, permission_type: 0 })
    permissionTree.value = (res.tree ?? []) as unknown as EditRolePermissionTree[]
    selectedIds.value = new Set()
  }
  childrenMap.value = buildChildrenMap(permissionTree.value)
}

// 勾选联动：勾选父节点 → 勾选所有子孙；取消 → 取消所有子孙
function onToggle(id: number, checked: boolean) {
  const next = new Set(selectedIds.value)
  const descendants = childrenMap.value.get(id) ?? []
  if (checked) {
    next.add(id)
    descendants.forEach((d) => next.add(d))
  } else {
    next.delete(id)
    descendants.forEach((d) => next.delete(d))
  }
  selectedIds.value = next
}

function checkForm(): boolean {
  if (!form.role_name.trim()) {
    ElMessage.warning('请输入角色名称')
    return false
  }
  if (!form.role_code.trim()) {
    ElMessage.warning('请输入角色编码')
    return false
  }
  if (!/^[a-zA-Z0-9_]{3,50}$/.test(form.role_code.trim())) {
    ElMessage.warning('角色编码只能包含字母、数字和下划线，长度3-50位')
    return false
  }
  return true
}

async function onSubmit() {
  if (!checkForm()) return
  const permissionIds = Array.from(selectedIds.value)

  submitLoading.value = true
  try {
    if (isEdit.value) {
      await apiUpdateRole({
        role_id: form.role_id,
        role_name: form.role_name.trim(),
        permission_ids: permissionIds,
      })
      ElMessage.success('更新角色成功')
    } else {
      await apiAddRole({
        role_name: form.role_name.trim(),
        role_code: form.role_code.trim(),
        description: form.description.trim(),
        sort_order: Number(form.sort_order) || 0,
        permission_ids: permissionIds,
      })
      ElMessage.success('新增角色成功')
    }
    router.push({ name: 'role' })
  } finally {
    submitLoading.value = false
  }
}

function goBack() {
  router.push({ name: 'role' })
}

// 接住加载失败（提示已由 http.ts 拦截器弹过），避免未处理的 Promise rejection
// 被 utils/frontError.ts 误报成前端异常。
onMounted(() => {
  loadPermissionTree().catch(() => {})
})
</script>

<template>
  <div class="role-edit">
    <el-card shadow="never">
      <template #header>
        <div class="edit-head">
          <el-button link type="primary" :icon="ArrowLeft" @click="goBack">返回</el-button>
          <span class="page-title">{{ isEdit ? '编辑角色' : '新增角色' }}</span>
        </div>
      </template>

      <el-alert
        v-if="isSystemLocked"
        title="系统角色不可修改名称与权限"
        type="warning"
        :closable="false"
        show-icon
        class="tip"
      />

      <el-form label-width="90px">
        <el-divider content-position="left">基本信息</el-divider>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="角色名称" required>
              <el-input v-model="form.role_name" placeholder="请输入角色名称" :disabled="isSystemLocked" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="角色编码" required>
              <el-input
                v-model="form.role_code"
                placeholder="字母、数字、下划线，3-50位"
                :disabled="isEdit || isSystemLocked"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="排序">
              <el-input-number v-model="form.sort_order" :min="0" :disabled="isSystemLocked" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="16">
            <el-form-item label="描述">
              <el-input v-model="form.description" placeholder="请输入角色描述" :disabled="isSystemLocked" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">权限分配</el-divider>
        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="权限">
              <div class="perm-tree-box">
                <PermTree
                  :nodes="permissionTree"
                  :selected-ids="selectedIds"
                  :disabled="isSystemLocked"
                  @toggle="onToggle"
                />
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <div class="form-actions">
          <el-button :loading="submitLoading" type="primary" @click="onSubmit">提交</el-button>
          <el-button @click="goBack">取消</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

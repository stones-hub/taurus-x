<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'
// 权限新增/编辑
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  apiAddPermission,
  apiGetEditPermissionInfo,
  apiGetPermissionTree,
  apiUpdatePermission,
} from '../../api/permission'
import type { PermissionTreeNode } from '../../types'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => Boolean(route.query.permission_id))
const submitLoading = ref(false)
const permissionTree = ref<PermissionTreeNode[]>([])

const form = reactive({
  permission_id: 0,
  parent_id: 0,
  permission_name: '',
  permission_code: '',
  permission_type: 1,
  path: '',
  icon: '',
  sort_order: 0,
  status: 1,
  is_system: 0,
})

const isSystemLocked = computed(() => isEdit.value && form.is_system === 1)

const typeOptions = [
  { label: '菜单', value: 1 },
  { label: '按钮', value: 2 },
  { label: '接口', value: 3 },
]
const statusOptions = [
  { label: '启用', value: 1 },
  { label: '禁用', value: 0 },
]

// 将权限树转换为下拉选项（带缩进前缀）
function flattenTree(tree: PermissionTreeNode[], prefix = ''): { value: number; label: string }[] {
  const options: { value: number; label: string }[] = []
  for (const node of tree) {
    options.push({ value: node.permission_id, label: `${prefix}${node.permission_name} (${node.permission_code})` })
    if (node.children?.length) {
      options.push(...flattenTree(node.children, prefix + '  '))
    }
  }
  return options
}
const parentOptions = computed(() => flattenTree(permissionTree.value))

// 监听权限类型变化，重新加载权限树并重置父权限
async function onTypeChange() {
  form.parent_id = 0
  await loadPermissionTree()
}

async function loadPermissionTree() {
  if (isSystemLocked.value) return
  const res = await apiGetPermissionTree({ parent_id: 0, permission_type: form.permission_type })
  permissionTree.value = res.tree ?? []
}

async function loadEditData() {
  const id = Number(route.query.permission_id)
  const res = await apiGetEditPermissionInfo(id)
  form.permission_id = res.permission_id
  form.parent_id = res.parent_id || 0
  form.permission_name = res.permission_name
  form.permission_code = res.permission_code
  form.permission_type = res.permission_type
  form.path = res.path || ''
  form.icon = res.icon || ''
  form.sort_order = res.sort_order || 0
  form.status = res.status
  form.is_system = res.is_system
  permissionTree.value = res.permission_tree ?? []
}

function checkForm(): boolean {
  if (!form.permission_name.trim()) {
    ElMessage.warning('请输入权限名称')
    return false
  }
  if (!form.permission_code.trim()) {
    ElMessage.warning('请输入权限编码')
    return false
  }
  return true
}

async function onSubmit() {
  if (!checkForm()) return
  submitLoading.value = true
  try {
    const base = {
      parent_id: form.parent_id || 0,
      permission_name: form.permission_name.trim(),
      permission_code: form.permission_code.trim(),
      permission_type: form.permission_type,
      path: form.path.trim(),
      icon: form.icon.trim(),
      sort_order: Number(form.sort_order) || 0,
      status: Number(form.status),
      is_system: form.is_system || 0,
    }
    if (isEdit.value) {
      await apiUpdatePermission({ permission_id: form.permission_id, ...base })
      ElMessage.success('更新权限成功')
    } else {
      await apiAddPermission(base)
      ElMessage.success('新增权限成功')
    }
    router.push({ name: 'permission' })
  } finally {
    submitLoading.value = false
  }
}

function goBack() {
  router.push({ name: 'permission' })
}

// 接住加载失败：浮空的 Promise 失败会变成 Uncaught (in promise)，
// 还会被 utils/frontError.ts 误报成前端异常。提示已由 http.ts 拦截器弹过。
onMounted(() => {
  const loading = isEdit.value ? loadEditData() : loadPermissionTree()
  loading.catch(() => {})
})
</script>

<template>
  <div class="permission-edit">
    <el-card shadow="never">
      <template #header>
        <div class="edit-head">
          <el-button link type="primary" :icon="ArrowLeft" @click="goBack">返回</el-button>
          <span class="page-title">{{ isEdit ? '编辑权限' : '新增权限' }}</span>
        </div>
      </template>

      <el-alert
        v-if="isSystemLocked"
        title="系统权限不可修改名称、编码、类型、父权限、路径、图标、排序、状态"
        type="warning"
        :closable="false"
        show-icon
        class="tip"
      />

      <el-form label-width="90px">
        <el-divider content-position="left">必填选项</el-divider>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="权限名称" required>
              <el-input v-model="form.permission_name" placeholder="请输入权限名称" :disabled="isSystemLocked" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="权限编码" required>
              <el-input v-model="form.permission_code" placeholder="请输入权限编码（必须唯一）" :disabled="isSystemLocked" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="权限类型" required>
              <el-select
                v-model="form.permission_type"
                placeholder="请选择权限类型"
                style="width: 100%"
                :disabled="isSystemLocked"
                @change="onTypeChange"
              >
                <el-option v-for="o in typeOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="16">
            <el-form-item label="父权限">
              <el-select
                v-model="form.parent_id"
                placeholder="请选择父权限（留空表示根权限）"
                clearable
                style="width: 100%"
                :disabled="isSystemLocked"
              >
                <el-option label="根权限（无父权限）" :value="0" />
                <el-option v-for="o in parentOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">非必填选项，请按需填写</el-divider>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="路径">
              <el-input v-model="form.path" placeholder="请输入路径" :disabled="isSystemLocked" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="图标">
              <el-input v-model="form.icon" placeholder="请输入图标名称或类名" :disabled="isSystemLocked" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="排序">
              <el-input-number v-model="form.sort_order" :min="0" :disabled="isSystemLocked" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="状态">
              <el-select v-model="form.status" style="width: 100%" :disabled="isSystemLocked">
                <el-option v-for="o in statusOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
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

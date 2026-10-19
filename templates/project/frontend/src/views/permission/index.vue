<script setup lang="ts">
// 权限管理列表
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Plus } from '@element-plus/icons-vue'
import {
  apiDeletePermission,
  apiGetPermissionList,
  apiUpdatePermissionIsSystem,
  apiUpdatePermissionStatus,
} from '../../api/permission'
import { useListPage } from '../../composables/useListPage'
import type { PermissionListRequest, PermissionListResponseItem } from '../../types'
import { QUERY_ALL, PermissionType } from '../../types'

const router = useRouter()

// 按钮权限编码（变量形式，供 v-permission 引用）
const PERM = {
  query: 'system.account.permission.query',
  add: 'system.account.permission.add',
  refresh: 'system.account.permission.query',
  edit: 'system.account.permission.edit',
  delete: 'system.account.permission.delete',
} as const

interface PermissionFilter {
  permission_name: string
  status: number | null
  permission_type: number | null
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<PermissionListResponseItem, PermissionFilter, Omit<PermissionListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetPermissionList(p),
    buildParams: (f) => ({
      permission_name: f.permission_name,
      status: f.status ?? QUERY_ALL,
      permission_type: f.permission_type ?? QUERY_ALL,
    }),
  })

initForm({ permission_name: '', status: null, permission_type: null })

const typeMap: Record<number, string> = {
  [PermissionType.Menu]: '菜单',
  [PermissionType.Button]: '按钮',
  [PermissionType.API]: '接口',
}

function goAdd() {
  router.push({ name: 'permission-add' })
}

function goEdit(row: PermissionListResponseItem) {
  router.push({ name: 'permission-add', query: { permission_id: row.permission_id } })
}

async function handleStatusChange(row: PermissionListResponseItem, value: number) {
  try {
    await apiUpdatePermissionStatus(row.permission_id, value)
    ElMessage.success(value === 1 ? '权限已启用' : '权限已禁用')
    await loadTableData()
  } catch (e) {
    row.status = value === 1 ? 0 : 1
    ElMessage.error('状态更新失败')
  }
}

async function handleIsSystemChange(row: PermissionListResponseItem, value: number) {
  try {
    await apiUpdatePermissionIsSystem(row.permission_id, value)
    ElMessage.success('系统权限标记已更新')
    await loadTableData()
  } catch (e) {
    row.is_system = value === 1 ? 0 : 1
    ElMessage.error('更新失败')
  }
}

async function goDelete(row: PermissionListResponseItem) {
  try {
    await ElMessageBox.confirm('删除权限, 确定删除该条数据？', '提示', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await apiDeletePermission(row.permission_id)
    ElMessage.success('删除成功')
    await loadTableData()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}
</script>

<template>
  <div class="list-page">
    <el-card shadow="never">
      <!-- 筛选栏 -->
      <div class="filter-bar">
        <el-form inline @submit.prevent="search">
          <el-form-item label="权限名称">
            <el-input v-model="form.permission_name" placeholder="权限名称" clearable style="width: 180px" />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="form.status" placeholder="状态" clearable style="width: 120px">
              <el-option label="启用" :value="1" />
              <el-option label="禁用" :value="0" />
            </el-select>
          </el-form-item>
          <el-form-item label="权限类型">
            <el-select v-model="form.permission_type" placeholder="权限类型" clearable style="width: 120px">
              <el-option label="菜单" :value="PermissionType.Menu" />
              <el-option label="按钮" :value="PermissionType.Button" />
              <el-option label="接口" :value="PermissionType.API" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button v-permission="PERM.query" type="primary" :icon="Search" @click="search">查询</el-button>
            <el-button @click="reset">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 操作栏 -->
      <div class="toolbar">
        <el-button v-permission="PERM.add" type="primary" :icon="Plus" @click="goAdd">新增权限</el-button>
        <el-button v-permission="PERM.refresh" :icon="Refresh" @click="loadTableData">刷新列表</el-button>
      </div>

      <!-- 表格 -->
      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="permission_id" label="权限ID" width="90" align="center" />
        <el-table-column prop="permission_name" label="权限名称" min-width="140" />
        <el-table-column prop="permission_code" label="权限编码" min-width="200" />
        <el-table-column label="权限类型" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.permission_type === 1 ? 'primary' : row.permission_type === 2 ? 'success' : 'warning'">
              {{ typeMap[row.permission_type] ?? row.permission_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="parent_id" label="父权限ID" width="90" align="center" />
        <el-table-column prop="path" label="路径" min-width="180" show-overflow-tooltip />
        <el-table-column prop="icon" label="图标" width="100" />
        <el-table-column prop="sort_order" label="排序" width="70" align="center" />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.status"
              :active-value="1"
              :inactive-value="0"
              @change="(v: number | string | boolean) => handleStatusChange(row, v as number)"
            />
          </template>
        </el-table-column>
        <el-table-column label="系统权限" width="90" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.is_system"
              :active-value="1"
              :inactive-value="0"
              @change="(v: number | string | boolean) => handleIsSystemChange(row, v as number)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="add_time" label="添加时间" width="160" />
        <el-table-column prop="update_time" label="最后更新时间" width="160" />
        <el-table-column label="操作" width="140" fixed="right" align="center">
          <template #default="{ row }">
            <el-button v-permission="PERM.edit" link type="primary" @click="goEdit(row)">编辑</el-button>
            <el-button v-permission="PERM.delete" link type="danger" @click="goDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="table-pagination">
        <el-pagination
          :current-page="pagination.page_no"
          :page-size="pagination.page_size"
          :total="pagination.result_total"
          :page-sizes="[15, 30, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>
  </div>
</template>

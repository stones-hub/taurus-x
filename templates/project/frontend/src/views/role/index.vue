<script setup lang="ts">
// 角色管理列表
import { Search, Refresh, Plus } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { apiDeleteRole, apiGetRoleList, apiUpdateRoleStatus } from '../../api/role'
import type { RoleListRequest, RoleListResponseItem } from '../../types'
import { QUERY_ALL } from '../../types'
import { useListPage } from '../../composables/useListPage'

const router = useRouter()

// 按钮权限编码（变量形式，供 v-permission 引用）
const PERM = {
  query: 'system.account.role.query',
  add: 'system.account.role.add',
  refresh: 'system.account.role.query',
  edit: 'system.account.role.edit',
  delete: 'system.account.role.delete',
} as const

interface RoleFilter {
  role_name: string
  status: number | null
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<RoleListResponseItem, RoleFilter, Omit<RoleListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetRoleList(p),
    buildParams: (f) => ({
      role_name: f.role_name,
      status: f.status ?? QUERY_ALL,
    }),
  })

initForm({ role_name: '', status: null })

function goAdd() {
  router.push({ name: 'role-add' })
}

function goEdit(row: RoleListResponseItem) {
  router.push({ name: 'role-add', query: { role_id: row.role_id } })
}

async function handleStatusChange(row: RoleListResponseItem, value: number) {
  try {
    await apiUpdateRoleStatus(row.role_id, value)
    ElMessage.success(value === 1 ? '角色已启用' : '角色已禁用')
    await loadTableData()
  } catch (e) {
    row.status = value === 1 ? 0 : 1
    ElMessage.error('状态更新失败')
  }
}

async function goDelete(row: RoleListResponseItem) {
  try {
    await ElMessageBox.confirm('删除角色, 确定删除该条数据？', '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await apiDeleteRole(row.role_id)
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
          <el-form-item label="角色名称">
            <el-input v-model="form.role_name" placeholder="角色名称" clearable style="width: 180px" />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="form.status" placeholder="状态" clearable style="width: 120px">
              <el-option label="启用" :value="1" />
              <el-option label="禁用" :value="0" />
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
        <el-button v-permission="PERM.add" type="primary" :icon="Plus" @click="goAdd">新增角色</el-button>
        <el-button v-permission="PERM.refresh" :icon="Refresh" @click="loadTableData">刷新列表</el-button>
      </div>

      <!-- 表格 -->
      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="role_id" label="角色ID" width="90" align="center" />
        <el-table-column prop="role_name" label="角色名称" min-width="130" />
        <el-table-column prop="role_code" label="角色编码" min-width="150" />
        <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip />
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
        <el-table-column label="系统角色" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_system === 1 ? 'danger' : 'info'">{{ row.is_system === 1 ? '是' : '否' }}</el-tag>
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


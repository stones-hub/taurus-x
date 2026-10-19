<script setup lang="ts">
// 部门管理列表
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Plus } from '@element-plus/icons-vue'
import { apiDeleteDept, apiGetDeptList, apiUpdateDeptStatus } from '../../api/dept'
import { useListPage } from '../../composables/useListPage'
import type { DeptListRequest, DeptListResponseItem } from '../../types'
import { QUERY_ALL } from '../../types'

const router = useRouter()

const PERM = {
  query: 'system.department.dept.query',
  add: 'system.department.dept.add',
  refresh: 'system.department.dept.query',
  edit: 'system.department.dept.edit',
  delete: 'system.department.dept.delete',
  userAdd: 'system.department.dept.user.add',
} as const

interface DeptFilter {
  dept_name: string
  status: number | null
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<DeptListResponseItem, DeptFilter, Omit<DeptListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetDeptList(p),
    buildParams: (f) => ({
      dept_name: f.dept_name,
      status: f.status ?? QUERY_ALL,
    }),
  })

initForm({ dept_name: '', status: null })

function goAdd() {
  router.push({ name: 'dept-add' })
}

function goEdit(row: DeptListResponseItem) {
  router.push({ name: 'dept-add', query: { dept_id: row.dept_id } })
}

function manageUsers(row: DeptListResponseItem) {
  router.push({ name: 'dept-user-manage', query: { dept_id: row.dept_id } })
}

async function handleStatusChange(row: DeptListResponseItem, value: number) {
  try {
    await apiUpdateDeptStatus(row.dept_id, value)
    ElMessage.success(value === 1 ? '部门已启用' : '部门已禁用')
    await loadTableData()
  } catch (e) {
    row.status = value === 1 ? 0 : 1
    ElMessage.error('状态更新失败')
  }
}

async function goDelete(row: DeptListResponseItem) {
  try {
    await ElMessageBox.confirm('删除部门, 确定删除该条数据？', '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await apiDeleteDept(row.dept_id)
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
          <el-form-item label="部门名称">
            <el-input v-model="form.dept_name" placeholder="部门名称" clearable style="width: 200px" />
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
        <el-button v-permission="PERM.add" type="primary" :icon="Plus" @click="goAdd">新增部门</el-button>
        <el-button v-permission="PERM.refresh" :icon="Refresh" @click="loadTableData">刷新列表</el-button>
      </div>

      <!-- 表格 -->
      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="dept_id" label="部门ID" width="80" align="center" />
        <el-table-column prop="dept_name" label="部门名称" min-width="140" />
        <el-table-column label="父部门" min-width="120">
          <template #default="{ row }">{{ row.parent_dept_name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="dept_code" label="部门编码" min-width="120" />
        <el-table-column prop="description" label="部门描述" min-width="160" show-overflow-tooltip />
        <el-table-column prop="user_count" label="员工数量" width="90" align="center" />
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
        <el-table-column prop="add_time" label="添加时间" width="160" />
        <el-table-column prop="update_time" label="最后更新时间" width="160" />
        <el-table-column label="操作" width="180" fixed="right" align="center">
          <template #default="{ row }">
            <el-button v-permission="PERM.userAdd" link type="primary" @click="manageUsers(row)">员工</el-button>
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

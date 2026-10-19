<script setup lang="ts">
// 用户管理列表
import { Search, Refresh, Plus } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { apiDeleteUser, apiGetUserList, apiUpdateUserStatus } from '../../api/user-manage'
import type { UserInfo, UserListRequest } from '../../types'
import { QUERY_ALL } from '../../types'
import { useListPage } from '../../composables/useListPage'

const router = useRouter()

// 按钮权限编码
const PERM = {
  query: 'system.account.user.query',
  add: 'system.account.user.add',
  refresh: 'system.account.user.query',
  edit: 'system.account.user.edit',
  delete: 'system.account.user.delete',
} as const

interface UserFilter {
  username_mobile_email: string
  status: number | null
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<UserInfo, UserFilter, Omit<UserListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetUserList(p),
    buildParams: (f) => ({
      username_mobile_email: f.username_mobile_email,
      status: f.status ?? QUERY_ALL,
    }),
  })

initForm({ username_mobile_email: '', status: null })

function goAdd() {
  router.push({ name: 'user-add' })
}

function goEdit(row: UserInfo) {
  router.push({ name: 'user-add', query: { user_id: row.user_id } })
}

async function handleStatusChange(row: UserInfo, value: number) {
  try {
    await apiUpdateUserStatus(row.user_id, value)
    ElMessage.success(value === 1 ? '用户已启用' : '用户已禁用')
    await loadTableData()
  } catch (e) {
    row.status = value === 1 ? 0 : 1
    ElMessage.error('状态更新失败')
  }
}

async function goDelete(row: UserInfo) {
  try {
    await ElMessageBox.confirm('删除用户, 确定删除该条数据？', '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await apiDeleteUser(row.user_id)
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
        <el-form-item label="账号/手机/邮箱">
          <el-input v-model="form.username_mobile_email" placeholder="用户名/手机号/邮箱" clearable style="width: 220px" />
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
        <el-button v-permission="PERM.add" type="primary" :icon="Plus" @click="goAdd">新增用户</el-button>
        <el-button v-permission="PERM.refresh" :icon="Refresh" @click="loadTableData">刷新列表</el-button>
      </div>

      <!-- 表格 -->
      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="user_id" label="用户ID" width="80" align="center" />
        <el-table-column prop="realname" label="真实姓名" min-width="110" />
        <el-table-column prop="username" label="账号" min-width="120" />
        <el-table-column label="手机号" min-width="130">
          <template #default="{ row }">{{ row.mobile ?? '-' }}</template>
        </el-table-column>
        <el-table-column label="邮箱" min-width="150">
          <template #default="{ row }">{{ row.email ?? '-' }}</template>
        </el-table-column>
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
        <el-table-column prop="updated_at" label="最后更新时间" width="160" />
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


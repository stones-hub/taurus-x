<script setup lang="ts">
// 登录日志列表（只读）
import { apiGetLoginLogList } from '../../api/login-log'
import type { LoginLogListRequest, LoginLogResponse } from '../../types'
import { QUERY_ALL } from '../../types'
import { useListPage } from '../../composables/useListPage'

const PERM = {
  query: 'system.log.login.query',
} as const

interface LoginLogFilter {
  login_status: number | null
}

const { list, loading, pagination, form, initForm, search, handlePageChange, handleSizeChange } =
  useListPage<LoginLogResponse, LoginLogFilter, Omit<LoginLogListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetLoginLogList(p),
    buildParams: (f) => ({
      login_status: f.login_status ?? QUERY_ALL,
    }),
  })

initForm({ login_status: null })
</script>

<template>
  <div class="list-page">
    <el-card shadow="never">
      <!-- 筛选栏 -->
      <el-form inline @submit.prevent="search">
        <el-form-item label="登录状态">
          <el-select v-model="form.login_status" placeholder="状态" clearable style="width: 120px">
            <el-option label="成功" :value="1" />
            <el-option label="失败" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button v-permission="PERM.query" type="primary" @click="search">查询</el-button>
        </el-form-item>
      </el-form>

      <!-- 表格 -->
      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="log_id" label="日志ID" width="80" align="center" />
        <el-table-column prop="user_id" label="用户ID" width="80" align="center" />
        <el-table-column prop="username" label="用户名" min-width="110" />
        <el-table-column prop="login_type" label="登录方式" width="110" />
        <el-table-column prop="login_ip" label="登录IP" width="140" />
        <el-table-column prop="user_agent" label="Agent" min-width="180" show-overflow-tooltip />
        <el-table-column label="登录状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.login_status === 1 ? 'success' : 'danger'" size="small">
              {{ row.login_status === 1 ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="failure_reason" label="失败描述" min-width="140" show-overflow-tooltip />
        <el-table-column prop="login_time" label="登录时间" width="160" />
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

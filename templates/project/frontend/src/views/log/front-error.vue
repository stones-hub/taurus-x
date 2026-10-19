<script setup lang="ts">
// 前端错误日志列表（只读）
import { apiGetFrontErrorLogList } from '../../api/log'
import type { FrontErrorLogListRequest, FrontErrorLogResponse } from '../../types'
import { useListPage } from '../../composables/useListPage'
import { Search, Refresh } from '@element-plus/icons-vue'

const PERM = {
  query: 'system.log.frontError.query',
  refresh: 'system.log.frontError.query',
} as const

interface FrontErrorFilter {
  kind: string
  url: string
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<FrontErrorLogResponse, FrontErrorFilter, Omit<FrontErrorLogListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetFrontErrorLogList(p),
    buildParams: (f) => ({
      kind: f.kind || '',
      url: f.url || '',
    }),
  })

initForm({ kind: '', url: '' })
</script>

<template>
  <div class="list-page">
    <el-card shadow="never">
      <div class="filter-bar">
        <el-form inline @submit.prevent="search">
          <el-form-item label="错误类型">
            <el-select v-model="form.kind" placeholder="全部" clearable style="width: 150px">
              <el-option label="JS 错误" value="error" />
              <el-option label="Promise 异常" value="unhandledrejection" />
            </el-select>
          </el-form-item>
          <el-form-item label="页面">
            <el-input v-model="form.url" placeholder="页面 URL" clearable style="width: 220px" />
          </el-form-item>
          <el-form-item>
            <el-button v-permission="PERM.query" type="primary" :icon="Search" @click="search">查询</el-button>
            <el-button @click="reset">重置</el-button>
            <el-button v-permission="PERM.refresh" :icon="Refresh" @click="loadTableData">刷新</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="list" v-loading="loading" border>
        <el-table-column prop="log_id" label="日志ID" width="80" align="center" />
        <el-table-column prop="username" label="用户" width="100" />
        <el-table-column label="类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="row.kind === 'unhandledrejection' ? 'warning' : 'danger'" size="small">
              {{ row.kind === 'unhandledrejection' ? 'Promise异常' : 'JS错误' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="message" label="错误信息" min-width="220" show-overflow-tooltip />
        <el-table-column prop="url" label="页面" min-width="200" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="created_at" label="上报时间" width="160" />
      </el-table>

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

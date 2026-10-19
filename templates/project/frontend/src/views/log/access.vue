<script setup lang="ts">
// 访问日志列表（只读）
import { ref } from 'vue'
import { apiGetAccessLogList } from '../../api/log'
import type { AccessLogListRequest, AccessLogResponse } from '../../types'
import { useListPage } from '../../composables/useListPage'
import DetailCard from '../../components/DetailCard.vue'
import { Search, Refresh } from '@element-plus/icons-vue'

const PERM = {
  query: 'system.log.access.query',
  refresh: 'system.log.access.query',
} as const

interface AccessLogFilter {
  method: string
  path: string
  ip: string
  status_code: number | null
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<AccessLogResponse, AccessLogFilter, Omit<AccessLogListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetAccessLogList(p),
    buildParams: (f) => ({
      method: f.method || '',
      path: f.path || '',
      ip: f.ip || '',
      status_code: f.status_code ?? 0,
    }),
  })

initForm({ method: '', path: '', ip: '', status_code: null })

// 详情抽屉
const detailVisible = ref(false)
const detail = ref<AccessLogResponse | null>(null)

function openDetail(row: AccessLogResponse) {
  detail.value = row
  detailVisible.value = true
}
</script>

<template>
  <div class="list-page">
    <el-card shadow="never">
      <div class="filter-bar">
        <el-form inline @submit.prevent="search">
          <el-form-item label="方法">
            <el-select v-model="form.method" placeholder="全部" clearable style="width: 110px">
              <el-option label="GET" value="GET" />
              <el-option label="POST" value="POST" />
              <el-option label="PUT" value="PUT" />
              <el-option label="DELETE" value="DELETE" />
            </el-select>
          </el-form-item>
          <el-form-item label="路径">
            <el-input v-model="form.path" placeholder="路径" clearable style="width: 180px" />
          </el-form-item>
          <el-form-item label="IP">
            <el-input v-model="form.ip" placeholder="客户端IP" clearable style="width: 150px" />
          </el-form-item>
          <el-form-item label="状态码">
            <el-select v-model="form.status_code" placeholder="全部" clearable style="width: 100px">
              <el-option label="200" :value="200" />
              <el-option label="4xx" :value="400" />
              <el-option label="401" :value="401" />
              <el-option label="403" :value="403" />
              <el-option label="404" :value="404" />
              <el-option label="500" :value="500" />
            </el-select>
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
        <el-table-column prop="username" label="用户" width="110" />
        <el-table-column prop="method" label="方法" width="80" align="center" />
        <el-table-column label="路径" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <el-link type="primary" underline="never" @click="openDetail(row)">{{ row.path }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="状态码" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status_code >= 400 ? 'danger' : 'success'" size="small">{{ row.status_code }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column prop="duration_ms" label="耗时(ms)" width="90" align="right" />
        <el-table-column prop="created_at" label="访问时间" width="160" />
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

    <!-- 详情抽屉 -->
    <el-drawer v-model="detailVisible" title="访问日志详情" size="720px">
      <template v-if="detail">
        <DetailCard
          :fields="[
            { label: '日志ID', value: detail.log_id },
            { label: '用户', value: detail.username },
            { label: '方法', value: detail.method },
            { label: '状态码', value: detail.status_code, tagType: detail.status_code >= 400 ? 'danger' : 'success' },
            { label: '请求路径', value: detail.path, span: 2 },
            { label: 'IP', value: detail.ip },
            { label: '耗时', value: detail.duration_ms + 'ms' },
            { label: 'UserAgent', value: detail.user_agent, span: 2 },
            { label: '查询串', value: detail.query, span: 2 },
            { label: '访问时间', value: detail.created_at, span: 2 },
          ]"
        />

        <el-divider content-position="left">请求体</el-divider>
        <pre class="log-body">{{ detail.request_body || '(空)' }}</pre>

        <el-divider content-position="left">响应体</el-divider>
        <pre class="log-body">{{ detail.response_body || '(空)' }}</pre>
      </template>
    </el-drawer>
  </div>
</template>

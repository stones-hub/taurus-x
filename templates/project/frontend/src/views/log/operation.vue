<script setup lang="ts">
// 操作日志列表（只读）
import { ref } from 'vue'
import { apiGetOperationLogList } from '../../api/log'
import type { OperationLogListRequest, OperationLogResponse } from '../../types'
import { useListPage } from '../../composables/useListPage'
import DetailCard from '../../components/DetailCard.vue'
import { Search, Refresh } from '@element-plus/icons-vue'

const PERM = {
  query: 'system.log.operation.query',
  refresh: 'system.log.operation.query',
} as const

interface OperationLogFilter {
  method: string
  path: string
  action: string
  resource: string
}

const { list, loading, pagination, form, initForm, loadTableData, search, reset, handlePageChange, handleSizeChange } =
  useListPage<OperationLogResponse, OperationLogFilter, Omit<OperationLogListRequest, 'page_no' | 'page_size'>>({
    fetcher: (p) => apiGetOperationLogList(p),
    buildParams: (f) => ({
      method: f.method || '',
      path: f.path || '',
      action: f.action || '',
      resource: f.resource || '',
    }),
  })

initForm({ method: '', path: '', action: '', resource: '' })

// 详情抽屉
const detailVisible = ref(false)
const detail = ref<OperationLogResponse | null>(null)

function openDetail(row: OperationLogResponse) {
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
              <el-option label="POST" value="POST" />
              <el-option label="PUT" value="PUT" />
              <el-option label="DELETE" value="DELETE" />
            </el-select>
          </el-form-item>
          <el-form-item label="动作">
            <el-select v-model="form.action" placeholder="全部" clearable style="width: 100px">
              <el-option label="新增" value="add" />
              <el-option label="修改" value="update" />
              <el-option label="删除" value="delete" />
            </el-select>
          </el-form-item>
          <el-form-item label="资源模块">
            <el-input v-model="form.resource" placeholder="如 order" clearable style="width: 140px" />
          </el-form-item>
          <el-form-item label="路径">
            <el-input v-model="form.path" placeholder="接口路径" clearable style="width: 180px" />
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
        <el-table-column prop="resource" label="模块" width="100" />
        <el-table-column prop="action" label="动作" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.action === 'delete' ? 'danger' : row.action === 'update' ? 'warning' : 'primary'" size="small">
              {{ row.action === 'add' ? '新增' : row.action === 'update' ? '修改' : row.action === 'delete' ? '删除' : row.action }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="接口路径" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <el-link type="primary" underline="never" @click="openDetail(row)">{{ row.path }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="结果" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status_code >= 400 ? 'danger' : 'success'" size="small">{{ row.result || (row.status_code >= 400 ? '失败' : '成功') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="created_at" label="操作时间" width="160" />
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
    <el-drawer v-model="detailVisible" title="操作日志详情" size="720px">
      <template v-if="detail">
        <DetailCard
          :fields="[
            { label: '日志ID', value: detail.log_id },
            { label: '用户', value: detail.username },
            { label: '方法', value: detail.method },
            { label: '动作', value: detail.action, tagType: detail.action === 'delete' ? 'danger' : detail.action === 'update' ? 'warning' : 'primary' },
            { label: '接口路径', value: detail.path, span: 2 },
            { label: '资源模块', value: detail.resource },
            { label: 'IP', value: detail.ip },
            { label: '状态码', value: detail.status_code },
            { label: '结果', value: detail.result },
            { label: '操作时间', value: detail.created_at, span: 2 },
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

<script setup lang="ts">
// 部门员工管理（穿梭框批量分配）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiBatchUpdateDeptUser, apiGetDeptUserOptions } from '../../api/dept'
import { apiGetUserList } from '../../api/user-manage'
import { QUERY_ALL } from '../../types'
import { collectPages } from '../../utils/pagination'

const route = useRoute()
const router = useRouter()

const deptId = computed(() => Number(route.query.dept_id) || 0)

const loading = ref(false)
const submitLoading = ref(false)
const dataReady = ref(false)

// 所有用户（穿梭框左侧）
const allUsers = ref<{ label: string; key: number }[]>([])
// 当前已选（部门已有员工）
const selectedKeys = ref<number[]>([])

async function loadData() {
  loading.value = true
  dataReady.value = false
  try {
    // 左：所有用户
    const users = await collectPages((page_no, page_size) => apiGetUserList({
      page_no,
      page_size,
      username_mobile_email: '',
      status: QUERY_ALL,
    }), (user) => user.user_id)
    allUsers.value = users.map((u) => ({
      label: `${u.realname || u.username} (${u.username})`,
      key: u.user_id,
    }))
    // 右：部门已有员工
    const deptUsers = await collectPages((page, size) => apiGetDeptUserOptions(deptId.value, page, size), (user) => user.user_id)
    selectedKeys.value = deptUsers.map((member) => member.user_id)
    dataReady.value = true
  } finally {
    loading.value = false
  }
}

async function onSave() {
  if (!dataReady.value) {
    ElMessage.error('员工列表尚未完整加载，请重新加载后保存')
    return
  }
  submitLoading.value = true
  try {
    await apiBatchUpdateDeptUser({ dept_id: deptId.value, user_ids: selectedKeys.value })
    ElMessage.success('员工分配成功')
    router.push({ name: 'dept' })
  } finally {
    submitLoading.value = false
  }
}

function goBack() {
  router.push({ name: 'dept' })
}

// 接住加载失败（提示已由 http.ts 拦截器弹过），避免未处理的 Promise rejection
// 被 utils/frontError.ts 误报成前端异常；loading 由 loadData 自己的 finally 复位。
onMounted(() => {
  loadData().catch(() => {})
})
</script>

<template>
  <div class="dept-user-manage">
    <el-card shadow="never">
      <template #header>
        <span class="title">部门员工管理</span>
      </template>

      <el-transfer
        v-model="selectedKeys"
        :data="allUsers"
        filterable
        filter-placeholder="搜索用户"
        :titles="['所有用户', '部门员工']"
        :button-texts="['移除', '添加']"
        v-loading="loading"
      />

      <div class="form-actions">
        <el-button :loading="submitLoading" type="primary" @click="onSave">提交</el-button>
        <el-button @click="goBack">取消</el-button>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.title {
  font-weight: 600;
  color: #1f2937;
  font-size: 15px;
}
.form-actions {
  margin-top: 20px;
  text-align: right;
}
.dept-user-manage :deep(.el-transfer) {
  display: flex;
  justify-content: center;
}
</style>

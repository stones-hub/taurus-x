<script setup lang="ts">
// 部门员工管理（穿梭框批量分配）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiBatchUpdateDeptUser, apiGetDeptUserList } from '../../api/dept'
import { apiGetUserList } from '../../api/user-manage'
import { QUERY_ALL } from '../../types'

const route = useRoute()
const router = useRouter()

const deptId = computed(() => Number(route.query.dept_id) || 0)

const loading = ref(false)
const submitLoading = ref(false)

// 所有用户（穿梭框左侧）
const allUsers = ref<{ label: string; key: number }[]>([])
// 当前已选（部门已有员工）
const selectedKeys = ref<number[]>([])

async function loadData() {
  loading.value = true
  try {
    // 左：所有用户
    const users = await apiGetUserList({
      page_no: 1,
      page_size: 1000,
      username_mobile_email: '',
      status: QUERY_ALL,
    })
    allUsers.value = (users.list ?? []).map((u) => ({
      label: `${u.realname || u.username} (${u.username})`,
      key: u.user_id,
    }))
    // 右：部门已有员工
    const deptUsers = await apiGetDeptUserList(deptId.value, 1, 1000)
    selectedKeys.value = (deptUsers.list ?? []).map((d) => d.user_id)
  } finally {
    loading.value = false
  }
}

async function onSave() {
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

onMounted(loadData)
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

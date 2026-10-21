<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'
// 部门新增/编辑
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiAddDept, apiGetAllDepts, apiGetEditDeptInfo, apiUpdateDept } from '../../api/dept'
import type { DeptOptionItem } from '../../types'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => Boolean(route.query.dept_id))

const submitLoading = ref(false)
const deptOptions = ref<DeptOptionItem[]>([])

const form = reactive({
  dept_id: 0,
  dept_name: '',
  parent_id: 0,
  dept_code: '',
  description: '',
  sort_order: 0,
})

async function loadOptions() {
  // 编辑时排除自身，避免选自己为父部门
  const exclude = isEdit.value ? Number(route.query.dept_id) : 0
  const res = await apiGetAllDepts(exclude)
  deptOptions.value = res ?? []
}

async function loadEditData() {
  const id = Number(route.query.dept_id)
  const res = await apiGetEditDeptInfo(id)
  form.dept_id = res.dept_id
  form.dept_name = res.dept_name
  form.parent_id = res.parent_id
  form.dept_code = res.dept_code
  form.description = res.description
  form.sort_order = res.sort_order
}

function checkForm(): boolean {
  if (!form.dept_name.trim()) {
    ElMessage.warning('请输入部门名称')
    return false
  }
  return true
}

async function onSubmit() {
  if (!checkForm()) return
  submitLoading.value = true
  try {
    if (isEdit.value) {
      await apiUpdateDept({
        dept_id: form.dept_id,
        dept_name: form.dept_name.trim(),
        parent_id: form.parent_id,
        description: form.description.trim(),
        sort_order: Number(form.sort_order) || 0,
      })
      ElMessage.success('更新部门成功')
    } else {
      await apiAddDept({
        dept_name: form.dept_name.trim(),
        parent_id: form.parent_id,
        dept_code: form.dept_code.trim(),
        description: form.description.trim(),
        sort_order: Number(form.sort_order) || 0,
        dept_source: 1,
      })
      ElMessage.success('新增部门成功')
    }
    router.push({ name: 'dept' })
  } finally {
    submitLoading.value = false
  }
}

function goBack() {
  router.push({ name: 'dept' })
}

onMounted(async () => {
  // 接住加载失败（提示已由 http.ts 拦截器弹过），避免未处理的 Promise rejection
  // 被 utils/frontError.ts 误报成前端异常。
  try {
    await loadOptions()
    if (isEdit.value) {
      await loadEditData()
    }
  } catch {
    // 已提示，无需额外处理
  }
})
</script>

<template>
  <div class="dept-edit">
    <el-card shadow="never">
      <template #header>
        <div class="edit-head">
          <el-button link type="primary" :icon="ArrowLeft" @click="goBack">返回</el-button>
          <span class="page-title">{{ isEdit ? '编辑部门' : '新增部门' }}</span>
        </div>
      </template>

      <el-form label-width="90px">
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="部门名称" required>
              <el-input v-model="form.dept_name" placeholder="请输入部门名称" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="父部门">
              <el-select v-model="form.parent_id" placeholder="请选择父部门" clearable style="width: 100%">
                <el-option label="顶级部门（无父部门）" :value="0" />
                <el-option v-for="d in deptOptions" :key="d.dept_id" :label="d.dept_name" :value="d.dept_id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="部门编码">
              <el-input v-model="form.dept_code" placeholder="请输入部门编码" :disabled="isEdit" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="排序">
              <el-input-number v-model="form.sort_order" :min="0" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="16">
            <el-form-item label="描述">
              <el-input v-model="form.description" placeholder="请输入部门描述" />
            </el-form-item>
          </el-col>
        </el-row>

        <div class="form-actions">
          <el-button :loading="submitLoading" type="primary" @click="onSubmit">提交</el-button>
          <el-button @click="goBack">取消</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

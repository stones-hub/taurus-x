<script setup lang="ts">
// 设置/重置密码
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiSetPassword } from '../../api/user'

const router = useRouter()

const passwordForm = reactive({
  new_password: '',
  confirm_password: '',
})

const submitLoading = ref(false)

// 密码强度校验（helper.ValidatePasswordStrength：6-20位，含字母和数字）
function validatePassword(password: string): string | null {
  if (password.length < 6 || password.length > 20) {
    return '密码长度需为6-20位'
  }
  if (!/[a-zA-Z]/.test(password) || !/\d/.test(password)) {
    return '密码需同时包含字母和数字'
  }
  return null
}

async function onSetPassword() {
  if (!passwordForm.new_password) {
    ElMessage.warning('请输入新密码')
    return
  }
  const err = validatePassword(passwordForm.new_password)
  if (err) {
    ElMessage.warning(err)
    return
  }
  if (passwordForm.new_password !== passwordForm.confirm_password) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  submitLoading.value = true
  try {
    await apiSetPassword({ new_password: passwordForm.new_password })
    ElMessage.success('密码设置成功，请重新登录')
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
    // 设置使旧 token 失效，跳转登录页
    router.push({ name: 'login' })
  } finally {
    submitLoading.value = false
  }
}

function goBack() {
  // 返回个人信息页面
  router.push({ name: 'profile' })
}
</script>

<template>
  <div class="set-password">
    <el-card>
      <el-alert
        title="此页面用于设置或重置密码。设置成功后，您可以使用用户名和密码登录系统。"
        type="info"
        :closable="false"
        show-icon
        class="tip"
      />
      <el-form label-width="100px" label-position="left">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="新密码">
              <el-input
                v-model="passwordForm.new_password"
                type="password"
                show-password
                placeholder="6-20位，含字母和数字"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="确认新密码">
              <el-input
                v-model="passwordForm.confirm_password"
                type="password"
                show-password
                placeholder="请再次输入新密码"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <div class="form-actions">
          <el-button @click="goBack">取消</el-button>
          <el-button type="primary" :loading="submitLoading" @click="onSetPassword">设置密码</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>


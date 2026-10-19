<script setup lang="ts">
// 个人信息管理（三区块：基本信息/修改密码/手机号管理）
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  apiBindMobile,
  apiChangePassword,
  apiCurrentInfo,
  apiSendCode,
  apiUnbindMobile,
  apiUpdateProfile,
} from '../../api/user'
import type { UserInfo } from '../../types'

const userInfo = ref<UserInfo | null>(null)

// 基本信息表单
const profileForm = reactive({
  realname: '',
  nickname: '',
  email: '',
  gender: 0,
  birthday: '',
})

// 修改密码表单
const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

// 手机号表单
const mobileForm = reactive({
  mobile: '',
  code: '',
})

const passwordLoading = ref(false)
const profileLoading = ref(false)
const mobileLoading = ref(false)
const codeLoading = ref(false)

async function loadUserInfo() {
  userInfo.value = await apiCurrentInfo()
  profileForm.realname = userInfo.value.realname ?? ''
  profileForm.nickname = userInfo.value.nickname ?? ''
  profileForm.email = userInfo.value.email ?? ''
  profileForm.gender = userInfo.value.gender ?? 0
  profileForm.birthday = userInfo.value.birthday ?? ''
  mobileForm.mobile = ''
  mobileForm.code = ''
}

async function onUpdateProfile() {
  profileLoading.value = true
  try {
    await apiUpdateProfile({
      realname: profileForm.realname.trim(),
      nickname: profileForm.nickname.trim(),
      email: profileForm.email.trim() || 'default@example.com',
      gender: profileForm.gender,
      birthday: profileForm.birthday.trim() || '1900-01-01',
    })
    ElMessage.success('个人信息更新成功')
    await loadUserInfo()
  } finally {
    profileLoading.value = false
  }
}

async function onChangePassword() {
  if (!passwordForm.old_password) {
    ElMessage.warning('请输入原密码')
    return
  }
  if (passwordForm.new_password !== passwordForm.confirm_password) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  passwordLoading.value = true
  try {
    await apiChangePassword({
      old_password: passwordForm.old_password,
      new_password: passwordForm.new_password,
    })
    ElMessage.success('密码修改成功')
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
  } finally {
    passwordLoading.value = false
  }
}

async function onSendCode() {
  const mobile = userInfo.value?.mobile || mobileForm.mobile.trim()
  if (!mobile) {
    ElMessage.warning('请输入手机号')
    return
  }
  codeLoading.value = true
  try {
    await apiSendCode({ login_type: 'mobile', login_value: mobile })
    ElMessage.success('验证码发送成功')
  } finally {
    codeLoading.value = false
  }
}

async function onBindMobile() {
  if (!mobileForm.mobile.trim()) {
    ElMessage.warning('请输入手机号')
    return
  }
  if (!mobileForm.code.trim()) {
    ElMessage.warning('请输入验证码')
    return
  }
  mobileLoading.value = true
  try {
    await apiBindMobile({ mobile: mobileForm.mobile.trim(), code: mobileForm.code.trim() })
    ElMessage.success('手机号绑定成功')
    await loadUserInfo()
  } finally {
    mobileLoading.value = false
  }
}

async function onUnbindMobile() {
  if (!userInfo.value?.mobile) {
    ElMessage.warning('当前未绑定手机号')
    return
  }
  if (!mobileForm.code.trim()) {
    ElMessage.warning('请输入验证码')
    return
  }
  mobileLoading.value = true
  try {
    await apiUnbindMobile({ mobile: userInfo.value.mobile, code: mobileForm.code.trim() })
    ElMessage.success('手机号解绑成功')
    await loadUserInfo()
  } finally {
    mobileLoading.value = false
  }
}

onMounted(loadUserInfo)
</script>

<template>
  <div class="profile">
    <el-card>
      <template #header>
        <span class="section-title">基本信息</span>
      </template>
      <el-form label-width="100px" label-position="left">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="用户ID">
              <el-input :model-value="userInfo?.user_id" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="用户名">
              <el-input :model-value="userInfo?.username" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="真实姓名">
              <el-input v-model="profileForm.realname" placeholder="请输入真实姓名" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="昵称">
              <el-input v-model="profileForm.nickname" placeholder="请输入昵称" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="邮箱">
              <el-input v-model="profileForm.email" placeholder="请输入邮箱" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="性别">
              <el-select v-model="profileForm.gender" placeholder="请选择性别">
                <el-option label="未知" :value="0" />
                <el-option label="男" :value="1" />
                <el-option label="女" :value="2" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="生日">
              <el-date-picker
                v-model="profileForm.birthday"
                type="date"
                value-format="YYYY-MM-DD"
                placeholder="请选择生日"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="职位">
              <el-input :model-value="userInfo?.position" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="最后登录时间">
              <el-input :model-value="userInfo?.last_login_time" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="最后登录IP">
              <el-input :model-value="userInfo?.last_login_ip" disabled />
            </el-form-item>
          </el-col>
        </el-row>
        <div class="form-actions">
          <el-button type="primary" :loading="profileLoading" @click="onUpdateProfile">保存</el-button>
        </div>
      </el-form>
    </el-card>

    <el-card>
      <template #header>
        <span class="section-title">修改密码</span>
      </template>
      <el-form label-width="100px" label-position="left">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="原密码">
              <el-input v-model="passwordForm.old_password" type="password" show-password placeholder="请输入原密码" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="新密码">
              <el-input v-model="passwordForm.new_password" type="password" show-password placeholder="6-20位，含字母和数字" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="确认新密码">
              <el-input v-model="passwordForm.confirm_password" type="password" show-password placeholder="请再次输入新密码" />
            </el-form-item>
          </el-col>
        </el-row>
        <div class="form-actions">
          <el-button type="primary" :loading="passwordLoading" @click="onChangePassword">修改密码</el-button>
        </div>
      </el-form>
    </el-card>

    <el-card>
      <template #header>
        <span class="section-title">手机号管理</span>
      </template>
      <el-form label-width="100px" label-position="left">
        <el-row :gutter="20">
          <el-col v-if="userInfo?.mobile" :span="12">
            <el-form-item label="当前手机号">
              <el-input :model-value="userInfo.mobile" disabled />
            </el-form-item>
          </el-col>
          <el-col v-else :span="12">
            <el-form-item label="手机号">
              <el-input v-model="mobileForm.mobile" placeholder="请输入手机号" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="验证码">
              <div class="code-row">
                <el-input v-model="mobileForm.code" placeholder="请输入验证码" />
                <el-button :loading="codeLoading" @click="onSendCode">发送验证码</el-button>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
        <div class="form-actions">
          <el-button v-if="!userInfo?.mobile" type="primary" :loading="mobileLoading" @click="onBindMobile">
            绑定手机号
          </el-button>
          <el-button v-else type="danger" :loading="mobileLoading" @click="onUnbindMobile">
            解绑手机号
          </el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.profile {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.code-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
</style>

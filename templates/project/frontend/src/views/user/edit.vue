<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'
// 用户新增/编辑
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiAddUser, apiGetUserInfo, apiUpdateUser } from '../../api/user-manage'
import { apiGetRoleList } from '../../api/role'
import type { RoleListResponseItem, UserInfo } from '../../types'
import { QUERY_ALL } from '../../types'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => Boolean(route.query.user_id))

const submitLoading = ref(false)
const roles = ref<RoleListResponseItem[]>([])
const checkedRoleIds = ref<number[]>([])
// 进入编辑页时目标账号真实持有的角色（后端给的，不是当前勾选状态）。
// 用来区分「已经持有的禁用角色」和「想新增的禁用角色」：前者允许保留和取消，
// 后者前端直接不可勾选，服务端也会拒（CheckRoleStatusAssignable）。
const heldRoleIds = ref<number[]>([])

const form = reactive({
  user_id: 0,
  username: '',
  password: '',
  mobile: '',
  realname: '',
  nickname: '',
  avatar: '',
  email: '',
  gender: 0,
  birthday: '',
})

// 加载角色列表（含已禁用：目标账号可能持有某个后来被禁用的角色，
// 不列出来就会在保存时把这条关联静默删掉）
async function loadRoles() {
  const res = await apiGetRoleList({ page_no: 1, page_size: 200, role_name: '', status: QUERY_ALL })
  roles.value = res.list ?? []
}

// 禁用角色只有「已经持有」时才可勾选（即只能保留或取消，不能新增）
function roleSelectable(role: RoleListResponseItem): boolean {
  return role.status === 1 || heldRoleIds.value.includes(role.role_id)
}

// 加载编辑数据
async function loadEditData() {
  const id = Number(route.query.user_id)
  const res = await apiGetUserInfo(id)
  const u: UserInfo = res.user
  form.user_id = u.user_id
  form.username = u.username
  form.mobile = u.mobile ?? ''
  form.realname = u.realname
  form.nickname = u.nickname
  form.avatar = u.avatar
  form.email = u.email ?? ''
  form.gender = u.gender
  form.birthday = u.birthday
  // 回显已选角色（permissions.roles 里 checked 的；后端会把持有中的禁用角色一并带上）
  const checked: number[] = []
  const known = new Map<number, RoleListResponseItem>()
  for (const r of roles.value) known.set(r.role_id, r)
  for (const r of res.permissions?.roles ?? []) {
    if (!r.checked) continue
    checked.push(r.role.role_id)
    // 角色列表接口分页取 200 条，极端情况下可能没覆盖到这个角色，补进来保证可见
    if (!known.has(r.role.role_id)) {
      roles.value = [
        ...roles.value,
        {
          role_id: r.role.role_id,
          role_name: r.role.role_name,
          role_code: r.role.role_code,
          description: r.role.description,
          sort_order: r.role.sort_order,
          status: r.role.status,
          is_system: r.role.is_system,
          update_time: r.role.updated_at,
          add_time: r.role.created_at,
        },
      ]
    }
  }
  heldRoleIds.value = checked
  checkedRoleIds.value = [...checked]
}

function onRoleToggle(roleId: number, checked: boolean) {
  const next = new Set(checkedRoleIds.value)
  if (checked) next.add(roleId)
  else next.delete(roleId)
  checkedRoleIds.value = Array.from(next)
}

function checkForm(): boolean {
  if (!form.username.trim()) {
    ElMessage.warning('请输入用户名')
    return false
  }
  if (!isEdit.value && !form.password) {
    ElMessage.warning('请输入密码')
    return false
  }
  if (!isEdit.value && !form.mobile.trim()) {
    ElMessage.warning('请输入手机号')
    return false
  }
  return true
}

async function onSubmit() {
  if (!checkForm()) return
  submitLoading.value = true
  try {
    if (isEdit.value) {
      await apiUpdateUser({
        user_id: form.user_id,
        realname: form.realname.trim(),
        nickname: form.nickname.trim(),
        password: form.password, // 空则不改密码
        gender: form.gender,
        birthday: form.birthday.trim(),
        email: form.email.trim(),
        role_ids: checkedRoleIds.value,
      })
      ElMessage.success('更新用户成功')
    } else {
      await apiAddUser({
        username: form.username.trim(),
        password: form.password,
        mobile: form.mobile.trim(),
        realname: form.realname.trim(),
        nickname: form.nickname.trim(),
        avatar: form.avatar.trim(),
        email: form.email.trim(),
        gender: form.gender,
        birthday: form.birthday.trim(),
        user_source: 3, // 管理员创建
        role_ids: checkedRoleIds.value,
      })
      ElMessage.success('新增用户成功')
    }
    router.push({ name: 'user' })
  } finally {
    submitLoading.value = false
  }
}

function goBack() {
  router.push({ name: 'user' })
}

onMounted(async () => {
  await loadRoles()
  if (isEdit.value) {
    await loadEditData()
  }
})
</script>

<template>
  <div class="user-edit">
    <el-card shadow="never">
      <template #header>
        <div class="edit-head">
          <el-button link type="primary" :icon="ArrowLeft" @click="goBack">返回</el-button>
          <span class="page-title">{{ isEdit ? '编辑用户' : '新增用户' }}</span>
        </div>
      </template>

      <el-form label-width="90px">
        <el-divider content-position="left">登录信息</el-divider>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="用户名" required>
              <el-input v-model="form.username" placeholder="5-20位" :disabled="isEdit" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="手机号" required>
              <el-input v-model="form.mobile" placeholder="请输入手机号" :disabled="isEdit" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item :label="isEdit ? '密码' : '密码'" required>
              <el-input
                v-model="form.password"
                type="password"
                show-password
                :placeholder="isEdit ? '留空则不修改密码' : '请输入密码'"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">基本信息</el-divider>
        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="真实姓名">
              <el-input v-model="form.realname" placeholder="请输入真实姓名" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="昵称">
              <el-input v-model="form.nickname" placeholder="请输入昵称" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="邮箱">
              <el-input v-model="form.email" placeholder="请输入邮箱" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="性别">
              <el-select v-model="form.gender" style="width: 100%">
                <el-option label="未知" :value="0" />
                <el-option label="男" :value="1" />
                <el-option label="女" :value="2" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="生日">
              <el-date-picker
                v-model="form.birthday"
                type="date"
                value-format="YYYY-MM-DD"
                placeholder="请选择生日"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">角色分配</el-divider>
        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="角色">
              <div class="role-list">
                <el-checkbox
                  v-for="role in roles"
                  :key="role.role_id"
                  :model-value="checkedRoleIds.includes(role.role_id)"
                  :disabled="!roleSelectable(role)"
                  class="role-item"
                  @change="(v: boolean) => onRoleToggle(role.role_id, v)"
                >
                  {{ role.role_name }}
                  <el-tag v-if="role.status !== 1" size="small" type="warning">已禁用</el-tag>
                </el-checkbox>
              </div>
              <div v-if="roles.length === 0" class="empty-tip">暂无可用角色</div>
              <div class="tip">
                已禁用的角色只保留目标账号现有的关联，可以取消，但不能新增。
              </div>
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

<style scoped>
.role-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 20px;
}
.role-item {
  margin-right: 0;
  height: auto;
}
</style>

<script setup lang="ts">
import { useAuthStore } from '../../store/auth'

const auth = useAuthStore()
</script>

<template>
  <el-card>
    <template #header>
      <span>控制台</span>
    </template>
    <p style="color: #6b7280">
      欢迎，{{ auth.user?.realname || auth.user?.username || 'Admin' }}！请从左侧菜单选择功能。
    </p>
  </el-card>
</template>

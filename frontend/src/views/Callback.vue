<template>
    <div class="callback-container"><el-loading text="登录中，请稍候..." /></div>
</template>
<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const router = useRouter()
onMounted(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')
    const userInfoStr = params.get('user_info')
    if (token) {
        localStorage.setItem('access_token', token)
        if (userInfoStr) localStorage.setItem('user_info', userInfoStr)
        ElMessage.success('登录成功')
        router.replace('/dashboard')
    } else {
        ElMessage.error('登录失败，请重试')
        window.location.href = import.meta.env.VITE_FEISHU_LOGIN_URL
    }
})
</script>
<style scoped>
.callback-container {
    height: 100vh;
    display: flex;
    justify-content: center;
    align-items: center;
}
</style>
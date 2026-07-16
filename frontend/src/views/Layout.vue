<template>
  <el-container class="layout-container">
    <el-aside width="220px" class="aside">
      <div class="logo"><span>日志分析平台</span></div>
      <el-menu :default-active="$route.path" router class="menu" background-color="#001529" text-color="#fff"
        active-text-color="#409EFF">
        <!-- 第一个菜单项：实时日志查看 -->
        <el-menu-item index="/realtime">
          <el-icon>
            <Monitor />
          </el-icon>
          <span>实时日志查看</span>
        </el-menu-item>

        <el-menu-item index="/upload">
          <el-icon>
            <Upload />
          </el-icon>
          <span>日志上传</span>
        </el-menu-item>
        <el-menu-item index="/log-records">
          <el-icon>
            <FolderOpened />
          </el-icon>
          <span>日志记录</span>
        </el-menu-item>
        <el-menu-item index="/dashboard">
          <el-icon>
            <DataBoard />
          </el-icon>
          <span>仪表板</span>
        </el-menu-item>
        <el-menu-item index="/test-scenarios">
          <el-icon>
            <Operation />
          </el-icon>
          <span>测试场景</span>
        </el-menu-item>
        <el-menu-item index="/tasks">
          <el-icon>
            <List />
          </el-icon>
          <span>任务列表</span>
        </el-menu-item>
        <el-menu-item index="/rules">
          <el-icon>
            <Setting />
          </el-icon>
          <span>规则配置</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="header-left"><span class="breadcrumb">{{ $route.meta.title || '首页' }}</span></div>
        <div class="header-right">
          <span class="user-info">{{ userInfo.name || '未登录' }}</span>
          <el-button type="text" @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main"><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Monitor, DataBoard, FolderOpened, Upload, List, Operation, Setting } from '@element-plus/icons-vue'
import { getCurrentUser } from '@/api/auth'

const router = useRouter()
const userInfo = ref({ name: '加载中...' })

onMounted(async () => {
  try {
    const data = await getCurrentUser()
    userInfo.value = data
    localStorage.setItem('user_info', JSON.stringify(data))
  } catch {
    const stored = localStorage.getItem('user_info')
    if (stored) try { userInfo.value = JSON.parse(stored) } catch { }
  }
})

const handleLogout = () => {
  ElMessageBox.confirm('确定退出吗？', '提示').then(() => {
    localStorage.removeItem('access_token')
    localStorage.removeItem('user_info')
    window.location.href = import.meta.env.VITE_FEISHU_LOGIN_URL
  }).catch(() => { })
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.aside {
  background-color: #001529;
  color: #fff;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: bold;
  background: #002140;
}

.menu {
  border-right: none;
  height: calc(100vh - 60px);
}

.header {
  background: #fff;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}

.header-left {
  font-size: 16px;
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 15px;
}

.main {
  background: #f0f2f5;
  padding: 20px;
  height: calc(100vh - 80px);
  /* 确保有高度 */
  overflow: hidden;
}
</style>

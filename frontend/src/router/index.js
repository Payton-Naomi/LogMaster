import { createRouter, createWebHistory } from 'vue-router'

// 飞书登录地址（生产环境用）
const FEISHU_LOGIN_URL = import.meta.env.VITE_FEISHU_LOGIN_URL

// 判断是否为开发模式
const isDev = import.meta.env.MODE === 'development'

const routes = [
  // 回调页面
  {
    path: '/callback',
    name: 'Callback',
    component: () => import('@/views/Callback.vue'),
    meta: { requiresAuth: false }
  },
  // 主布局
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '仪表板' } },
      { path: 'upload', name: 'Upload', component: () => import('@/views/Upload.vue'), meta: { title: '日志上传' } },
      { path: 'serial-config', name: 'SerialConfig', component: () => import('@/views/SerialConfig.vue'), meta: { title: '串口配置' } },
      { path: 'tasks', name: 'TaskList', component: () => import('@/views/TaskList.vue'), meta: { title: '任务列表' } },
      { path: 'task/:taskId', name: 'TaskDetail', component: () => import('@/views/TaskDetail.vue'), meta: { title: '任务详情' } },
      { path: 'realtime/:taskId', name: 'RealtimeLog', component: () => import('@/views/RealtimeLog.vue'), meta: { title: '实时日志' } },
      { path: 'analysis/:taskId', name: 'AnalysisResult', component: () => import('@/views/AnalysisResult.vue'), meta: { title: '解析结果' } },
      { path: 'rules', name: 'RuleConfig', component: () => import('@/views/RuleConfig.vue'), meta: { title: '规则配置' } }
    ]
  },
  // 404
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// ---------- 路由守卫：核心改动 ----------
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('access_token')

  // ----- 开发模式：自动注入模拟 Token -----
  if (isDev && !token) {
    // 设置模拟用户信息（仅供开发调试）
    localStorage.setItem('access_token', 'dev_mock_token_' + Date.now())
    localStorage.setItem('user_info', JSON.stringify({ 
      name: '本地开发', 
      user_id: 'dev_user' 
    }))
    console.log('🔧 [开发模式] 已自动登录，跳过飞书认证')
    next()
    return
  }

  // ----- 生产模式：正常权限校验 -----
  if (to.meta.requiresAuth) {
    if (!token) {
      // 未登录，跳转到后端飞书登录地址
      window.location.href = FEISHU_LOGIN_URL
    } else {
      next()
    }
  } else {
    next()
  }
})

export default router
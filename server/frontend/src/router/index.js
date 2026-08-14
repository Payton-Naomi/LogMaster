import { createRouter, createWebHistory } from 'vue-router'

const FEISHU_LOGIN_URL = import.meta.env.VITE_FEISHU_LOGIN_URL || '/api/auth/feishu-login'
let sessionVerified = false

const routes = [
  // 主布局
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      // 默认进入日志上传
      { path: '', redirect: '/upload' },
      
      // 仪表板
      { 
        path: 'dashboard', 
        name: 'Dashboard', 
        component: () => import('@/views/Dashboard.vue'), 
        meta: { title: '仪表板' } 
      },

      // 测试场景
      {
        path: 'test-scenarios',
        name: 'TestScenarios',
        component: () => import('@/views/TestScenarioManager.vue'),
        meta: { title: '测试场景' }
      },
      
      // 日志上传
      { 
        path: 'upload', 
        name: 'Upload', 
        component: () => import('@/views/Upload.vue'), 
        meta: { title: '日志上传' } 
      },

      {
        path: 'query',
        name: 'QueryPortal',
        component: () => import('@/views/QueryPortal.vue'),
        meta: { title: '采集日志查询' }
      },

      // 日志记录
      {
        path: 'log-records',
        name: 'LogRecords',
        component: () => import('@/views/LogRecordsSpectacle.vue'),
        meta: { title: '日志记录' }
      },
      
      // 任务列表
      { 
        path: 'tasks', 
        name: 'TaskList', 
        component: () => import('@/views/TaskList.vue'), 
        meta: { title: '任务列表' } 
      },
      
      // 任务详情
      { 
        path: 'task/:taskId', 
        name: 'TaskDetail', 
        component: () => import('@/views/TaskDetail.vue'), 
        meta: { title: '任务详情' } 
      },
      
      // 规则配置
      { 
        path: 'rules', 
        name: 'RuleConfig', 
        component: () => import('@/views/RuleConfig.vue'), 
        meta: { title: '规则配置' } 
      },
      
      // 解析结果
      { 
        path: 'analysis/:taskId', 
        name: 'AnalysisResult', 
        component: () => import('@/views/AnalysisResult.vue'), 
        meta: { title: '解析结果' } 
      },

      {
        path: 'admin',
        name: 'Admin',
        component: () => import('@/views/Admin.vue'),
        meta: { title: '管理控制台' }
      }
    ]
  },
  // 404
  { path: '/:pathMatch(.*)*', redirect: '/upload' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth) {
    return true
  }

  if (sessionVerified) {
    return true
  }

  try {
    const response = await fetch('/api/user/info', { credentials: 'same-origin' })
    if (!response.ok) throw new Error('unauthorized')
    const result = await response.json()
    if (result.code !== 0) throw new Error(result.message)
    sessionVerified = true
    localStorage.setItem('user_info', JSON.stringify(result.data))
    return true
  } catch {
    window.location.replace(FEISHU_LOGIN_URL)
    return false
  }
})

export default router

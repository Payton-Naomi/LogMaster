<template>
  <el-container class="layout-container">
    <el-aside :width="isCollapsed ? '72px' : '232px'" class="desktop-aside">
      <div class="brand" :class="{ collapsed: isCollapsed }">
        <div class="brand-mark"><el-icon><DataAnalysis /></el-icon></div>
        <div v-show="!isCollapsed" class="brand-copy"><strong>LogMaster</strong><span>日志分析平台</span></div>
      </div>
      <nav class="navigation" aria-label="主导航">
        <el-menu :default-active="activeMenu" :collapse="isCollapsed" :collapse-transition="false" router class="menu">
          <template v-for="group in navGroups" :key="group.label">
            <div v-show="!isCollapsed" class="menu-group-label">{{ group.label }}</div>
            <el-menu-item v-for="item in group.items" :key="item.path" :index="item.path">
              <el-icon><component :is="item.icon" /></el-icon><template #title>{{ item.label }}</template>
            </el-menu-item>
          </template>
        </el-menu>
      </nav>
    </el-aside>

    <el-drawer v-model="mobileMenuOpen" direction="ltr" size="280px" :with-header="false" class="mobile-drawer">
      <div class="brand mobile-brand">
        <div class="brand-mark"><el-icon><DataAnalysis /></el-icon></div>
        <div class="brand-copy"><strong>LogMaster</strong><span>日志分析平台</span></div>
      </div>
      <nav class="navigation" aria-label="移动端主导航">
        <el-menu :default-active="activeMenu" router class="menu" @select="mobileMenuOpen = false">
          <template v-for="group in navGroups" :key="group.label">
            <div class="menu-group-label">{{ group.label }}</div>
            <el-menu-item v-for="item in group.items" :key="item.path" :index="item.path">
              <el-icon><component :is="item.icon" /></el-icon><template #title>{{ item.label }}</template>
            </el-menu-item>
          </template>
        </el-menu>
      </nav>
    </el-drawer>

    <el-container class="workspace">
      <el-header class="header">
        <div class="header-left">
          <el-tooltip :content="isMobile ? '打开导航' : isCollapsed ? '展开导航' : '收起导航'" placement="bottom">
            <button class="icon-button" type="button" @click="toggleNavigation">
              <el-icon><Menu v-if="isMobile" /><Expand v-else-if="isCollapsed" /><Fold v-else /></el-icon>
            </button>
          </el-tooltip>
          <div class="breadcrumb"><span>工作台</span><el-icon><ArrowRight /></el-icon><strong>{{ $route.meta.title || '首页' }}</strong></div>
        </div>
        <div class="header-right">
          <div class="service-status" :class="serviceStatus"><span class="status-dot" /><span>{{ serviceStatusText }}</span></div>
          <el-popover placement="bottom-end" :width="340" trigger="click" @show="loadNotifications"><template #reference><el-badge :value="unreadCount" :hidden="!unreadCount" class="notification-badge"><button class="icon-button" type="button" aria-label="通知"><el-icon><Bell /></el-icon></button></el-badge></template><div class="notification-popover"><div class="notification-head"><strong>通知</strong><div><el-button link @click="openNotificationSettings">通知设置</el-button><el-button link :disabled="!unreadCount" @click="readAll">全部已读</el-button></div></div><button v-for="item in notifications" :key="item.id" type="button" class="notification-item" :class="{ unread: !item.is_read }" @click="openNotification(item)"><strong>{{ item.title || '系统通知' }}</strong><span v-if="item.task_id || item.task_name || item.original_name" class="notification-task">解析任务：{{ notificationTaskName(item) }}</span><span>{{ item.message || item.content }}</span></button><el-empty v-if="!notifications.length" description="暂无通知" :image-size="45" /></div></el-popover><span class="header-divider" />
          <el-dropdown trigger="click" @command="handleUserCommand">
            <button class="user-menu" type="button">
              <span class="avatar">{{ userInitial }}</span><span class="user-name">{{ userInfo.name || '未登录' }}</span><el-icon class="chevron"><ArrowDown /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>{{ userInfo.email || '当前登录账号' }}</el-dropdown-item>
                <el-dropdown-item disabled>{{ roleLabel }}</el-dropdown-item>
                <el-dropdown-item divided command="logout"><el-icon><SwitchButton /></el-icon>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="main">
        <router-view v-slot="{ Component }">
          <component :is="Component" :class="{ 'shared-particle-surface': route.path !== '/log-records' }" />
        </router-view>
      </el-main>
    </el-container>
    <el-tooltip :content="themeMode === 'dark' ? '切换到白天模式' : '切换到黑夜模式'" placement="right">
      <button class="global-theme-toggle" type="button" :aria-label="themeMode === 'dark' ? '切换到白天模式' : '切换到黑夜模式'" :aria-pressed="themeMode === 'light'" @click="toggleTheme">
        <el-icon><Sunny v-if="themeMode === 'dark'" /><Moon v-else /></el-icon>
      </button>
    </el-tooltip>
    <el-dialog v-model="notificationSettingsOpen" title="通知设置" width="420px" append-to-body>
      <div class="notification-settings" v-loading="notificationSettingsLoading"><el-switch v-for="item in notificationSettingItems" :key="item.key" v-model="notificationSettings[item.key]" :active-text="item.label" /></div>
      <template #footer><el-button @click="notificationSettingsOpen = false">取消</el-button><el-button type="primary" :loading="notificationSettingsSaving" @click="saveNotificationSettings">保存</el-button></template>
    </el-dialog>
  </el-container>
</template>

<script setup>
import { computed, markRaw, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { ArrowDown, ArrowRight, Bell, DataAnalysis, DataBoard, Expand, FolderOpened, Fold, Key, List, Menu, Moon, Operation, Search, Setting, Sunny, SwitchButton, Upload } from '@element-plus/icons-vue'
import { getNotificationSettings, getNotifications, markAllNotificationsRead, markNotificationRead, updateNotificationSettings } from '@/api/notification'
import { getCurrentUser, logout } from '@/api/auth'
import { useTheme } from '@/utils/theme'

const route = useRoute()
const router = useRouter()
const isCollapsed = ref(false)
const isMobile = ref(false)
const mobileMenuOpen = ref(false)
const { themeMode, toggleTheme } = useTheme()
const serviceStatus = ref('checking')
const userInfo = ref({ name: '加载中...' })
const notifications = ref([])
const unreadCount = ref(0)
const notificationSettingsOpen = ref(false)
const notificationSettingsLoading = ref(false)
const notificationSettingsSaving = ref(false)
const notificationSettings = ref({ task_completed: true, task_failed: true, task_cancelled: true, ai_completed: true, ai_failed: true, result_assigned: true, result_commented: true })
const notificationSettingItems = [{ key: 'task_completed', label: '任务完成' }, { key: 'task_failed', label: '任务失败' }, { key: 'task_cancelled', label: '任务取消' }, { key: 'ai_completed', label: 'AI 分析完成' }, { key: 'ai_failed', label: 'AI 分析失败' }, { key: 'result_assigned', label: '异常结果被分配' }, { key: 'result_commented', label: '异常结果新增备注' }]
let notificationStream = null
const loginURL = import.meta.env.VITE_FEISHU_LOGIN_URL || '/api/auth/feishu-login'
const baseNavGroups = [
  { label: '日志', items: [{ path: '/upload', label: '日志上传', icon: markRaw(Upload) }, { path: '/query', label: '采集日志查询', icon: markRaw(Search) }] },
  { label: '分析', items: [{ path: '/log-records', label: '日志记录', icon: markRaw(FolderOpened) }, { path: '/tasks', label: '分析任务', icon: markRaw(List) }, { path: '/dashboard', label: '数据概览', icon: markRaw(DataBoard) }] },
  { label: '配置', items: [{ path: '/test-scenarios', label: '测试场景', icon: markRaw(Operation) }, { path: '/rules', label: '解析规则', icon: markRaw(Setting) }] },
  { label: '管理', items: [{ path: '/admin', label: '管理控制台', icon: markRaw(Key), roleAware: true }] }
]

const activeMenu = computed(() => route.path.startsWith('/task/') || route.path.startsWith('/analysis/') ? '/tasks' : route.path)
const navGroups = computed(() => baseNavGroups.map(group => ({
  ...group,
  items: group.items.map(item => item.roleAware ? { ...item, label: userInfo.value.role === 'user' ? '申请与审批' : '管理控制台' } : item)
})).filter(group => group.items.length))
const serviceStatusText = computed(() => ({ checking: '状态检测中', online: '服务正常', offline: '服务异常' }[serviceStatus.value]))
const userInitial = computed(() => (userInfo.value.name || '用户').trim().charAt(0).toUpperCase())
const roleLabel = computed(() => ({ user: '普通用户', developer: '开发', admin: '普通管理员', super_admin: '超级管理员' })[userInfo.value.role] || '普通用户')
const notificationTaskName = (item) => item.task_name || item.original_name || item.task_original_name || item.name || item.task_id || '未命名任务'

function updateViewport() {
  isMobile.value = window.innerWidth < 768
  if (!isMobile.value && window.innerWidth < 1440) isCollapsed.value = true
  if (isMobile.value) isCollapsed.value = false
  else mobileMenuOpen.value = false
}

function toggleNavigation() {
  if (isMobile.value) mobileMenuOpen.value = true
  else isCollapsed.value = !isCollapsed.value
}

async function loadUser() {
  try { userInfo.value = await getCurrentUser() }
  catch { userInfo.value = { name: '未登录' } }
}

async function checkService() {
  try {
    const response = await fetch('/api/health', { credentials: 'same-origin' })
    serviceStatus.value = response.ok ? 'online' : 'offline'
  } catch { serviceStatus.value = 'offline' }
}
async function loadNotifications() { try { const data = await getNotifications({ page: 1, page_size: 10 }); notifications.value = data.list || []; unreadCount.value = Number(data.unread || 0) } catch { /* Notification center is optional during backend rollout. */ } }
async function readAll() { try { await markAllNotificationsRead(); notifications.value = notifications.value.map(item => ({ ...item, is_read: true, read_at: item.read_at || new Date().toISOString() })); unreadCount.value = 0 } catch { ElMessage.error('通知标记失败') } }
async function openNotification(item) { try { if (!item.is_read) { await markNotificationRead(item.id); item.is_read = true; item.read_at = item.read_at || new Date().toISOString(); unreadCount.value = Math.max(0, unreadCount.value - 1) } if (item.task_id) await router.push({ name: 'TaskDetail', params: { taskId: item.task_id } }) } catch { ElMessage.error('通知处理失败') } }
async function openNotificationSettings() { notificationSettingsOpen.value = true; notificationSettingsLoading.value = true; try { notificationSettings.value = { ...notificationSettings.value, ...await getNotificationSettings() } } catch { ElMessage.error('通知设置加载失败') } finally { notificationSettingsLoading.value = false } }
async function saveNotificationSettings() { notificationSettingsSaving.value = true; try { await updateNotificationSettings(notificationSettings.value); notificationSettingsOpen.value = false; ElMessage.success('通知设置已保存') } catch { ElMessage.error('通知设置保存失败') } finally { notificationSettingsSaving.value = false } }
function startNotificationStream() { if (!window.EventSource) return; notificationStream = new EventSource('/api/notifications/stream', { withCredentials: true }); notificationStream.addEventListener('notification', event => { try { const item = JSON.parse(event.data); notifications.value = [item, ...notifications.value.filter(value => value.id !== item.id)].slice(0, 10); if (!item.is_read) unreadCount.value += 1; ElNotification({ title: item.title || '系统通知', message: item.message || '你有一条新通知', type: 'info', duration: 5000 }) } catch { /* Ignore malformed SSE payload. */ } }) }

async function handleUserCommand(command) {
  if (command !== 'logout') return
  try {
    await ElMessageBox.confirm('退出后需要重新通过飞书登录。', '确认退出', { confirmButtonText: '退出登录', cancelButtonText: '取消', type: 'warning' })
    await logout()
    window.location.href = loginURL
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('退出失败，请稍后重试')
  }
}

onMounted(() => {
  updateViewport()
  window.addEventListener('resize', updateViewport)
  loadUser()
  checkService()
  loadNotifications()
  startNotificationStream()
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewport)
  notificationStream?.close()
})
</script>

<style scoped>
.layout-container{height:100dvh;min-width:0;background:var(--lm-surface-page)}
.desktop-aside{z-index:2;display:flex;overflow:hidden;flex-direction:column;border-right:1px solid #252c36;background:#171c24;color:#fff;transition:width 180ms ease}
.brand{display:flex;height:72px;flex:0 0 72px;align-items:center;gap:12px;padding:0 18px;border-bottom:1px solid #2a313c}.brand.collapsed{justify-content:center;padding:0}
.brand-mark{display:grid;width:36px;height:36px;flex:0 0 36px;place-items:center;border-radius:6px;background:#2877e8;color:#fff;font-size:20px}
.brand-copy{display:flex;min-width:0;flex-direction:column;gap:3px;white-space:nowrap}.brand-copy strong{font-size:16px}.brand-copy span{color:#929cab;font-size:11px}
.navigation{flex:1;overflow-y:auto;padding:12px 10px 20px}.menu{border-right:0;background:transparent}.menu-group-label{padding:13px 12px 7px;color:#747f8e;font-size:11px;font-weight:600}
.menu :deep(.el-menu-item){height:42px;margin:3px 0;border-radius:5px;color:#aeb7c4}.menu :deep(.el-menu-item:hover){background:#222a35;color:#fff}.menu :deep(.el-menu-item.is-active){background:#243a57;color:#73aefc}
.menu :deep(.el-menu-item.is-active::before){position:absolute;left:0;width:3px;height:20px;border-radius:0 3px 3px 0;background:#4c94f5;content:''}.menu.el-menu--collapse{width:52px}.menu.el-menu--collapse :deep(.el-menu-item){justify-content:center;padding:0!important}
.workspace{min-width:0}.header{display:flex;height:64px;flex:0 0 64px;align-items:center;justify-content:space-between;padding:0 22px;border-bottom:1px solid var(--lm-border);background:#fff}
.header-left,.header-right,.breadcrumb,.user-menu,.service-status{display:flex;align-items:center}.header-left{min-width:0;gap:13px}.header-right{gap:16px}
.icon-button,.user-menu{border:0;background:transparent;cursor:pointer}.icon-button{display:grid;width:36px;height:36px;flex:0 0 36px;place-items:center;border-radius:5px;color:#556171;font-size:18px}.icon-button:hover{background:#f0f3f6;color:#1e2937}
.breadcrumb{min-width:0;gap:7px;color:#8a94a3;font-size:13px;white-space:nowrap}.breadcrumb .el-icon{font-size:11px}.breadcrumb strong{overflow:hidden;color:#26313e;font-weight:600;text-overflow:ellipsis}
.service-status{gap:7px;color:#657180;font-size:12px}.status-dot{width:7px;height:7px;border-radius:50%;background:#a9b1bc}.online .status-dot{background:#27936c;box-shadow:0 0 0 3px #e3f3ed}.offline .status-dot{background:#cf4f4f;box-shadow:0 0 0 3px #fbe8e8}.header-divider{width:1px;height:22px;background:#e7eaee}
.user-menu{max-width:210px;gap:9px;padding:4px 5px;border-radius:6px;color:#344152}.user-menu:hover{background:#f5f7f9}.avatar{display:grid;width:30px;height:30px;flex:0 0 30px;place-items:center;border-radius:50%;background:#e7effb;color:#286dc8;font-size:12px;font-weight:700}.user-name{overflow:hidden;font-size:13px;font-weight:500;text-overflow:ellipsis;white-space:nowrap}.chevron{flex:0 0 auto;color:#8a94a3;font-size:11px}
.notification-badge{display:flex}.notification-popover{display:grid;max-height:420px;gap:4px;overflow:auto}.notification-head{display:flex;align-items:center;justify-content:space-between;padding-bottom:8px;border-bottom:1px solid rgba(255,255,255,.12)}.notification-head>div{display:flex;gap:6px}.notification-item{display:flex;width:100%;flex-direction:column;gap:4px;padding:10px 8px;border:0;border-radius:5px;background:transparent;color:inherit;text-align:left;cursor:pointer}.notification-item:hover,.notification-item.unread{background:rgba(6,182,212,.12)}.notification-item strong{font-size:12px}.notification-item span{overflow:hidden;color:#8b9099;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.notification-item .notification-task{color:#67e8f9;font-size:11px}.notification-settings{display:grid;gap:16px}.notification-settings :deep(.el-switch){justify-content:space-between}
.main{height:calc(100dvh - 64px);min-width:0;overflow:hidden;padding:20px 22px;background:var(--lm-surface-page)}.main>*,.main :deep(>*){min-width:0}.mobile-brand{color:#fff}
:global(.mobile-drawer.el-drawer){background:#171c24}:global(.mobile-drawer .el-drawer__body){display:flex;overflow:hidden;flex-direction:column;padding:0}
@media(max-width:1439px) and (min-width:768px){.main{padding:14px 16px}.header{padding:0 16px}.service-status span:last-child,.header-divider{display:none}.header-right{gap:10px}}
@media(max-width:767px){.desktop-aside{display:none}.header{padding:0 14px}.breadcrumb>span,.breadcrumb>.el-icon,.service-status span:last-child,.header-divider,.user-name,.chevron{display:none}.header-right{gap:12px}.service-status{padding:0 5px}.main{padding:14px}}
</style>

<style scoped>
.layout-container,.workspace,.main{background:radial-gradient(circle at 70% -20%,rgba(6,182,212,.08),transparent 34%),#0b0d10!important}.desktop-aside{background:rgba(17,20,24,.82)!important;border-right-color:rgba(255,255,255,.07)!important}.brand{border-bottom-color:rgba(255,255,255,.07)!important}.brand-mark{background:linear-gradient(135deg,#0891b2,#06b6d4);box-shadow:0 8px 20px rgba(6,182,212,.2)}.menu-group-label{color:#5f6874}.menu :deep(.el-menu-item){color:#8b9099;transition:background .18s ease,transform .18s ease}.menu :deep(.el-menu-item:hover){background:rgba(255,255,255,.05);color:#e4e6ea;transform:translateX(2px)}.menu :deep(.el-menu-item.is-active){background:rgba(6,182,212,.14);color:#67e8f9}.menu :deep(.el-menu-item.is-active::before){background:#06b6d4;box-shadow:0 0 10px rgba(6,182,212,.8)}.header{background:rgba(11,13,16,.72)!important;border-bottom-color:rgba(255,255,255,.07)!important}.icon-button,.user-menu{color:#aab0b8}.icon-button:hover,.user-menu:hover{background:rgba(255,255,255,.05);color:#e4e6ea}.avatar{background:rgba(6,182,212,.16);color:#67e8f9}.breadcrumb{color:#717780}.breadcrumb strong{color:#e4e6ea}.service-status{color:#8b9099}.online .status-dot{background:#34d399;box-shadow:0 0 0 3px rgba(52,211,153,.12),0 0 10px rgba(52,211,153,.5)}.offline .status-dot{background:#f43f5e;box-shadow:0 0 0 3px rgba(244,63,94,.12)}
</style>
<style scoped>
.layout-container,.workspace,.main{background:#020617}.header{background:#0b1220;border-bottom-color:#1e293b}.breadcrumb{color:#64748b}.breadcrumb strong{color:#e2e8f0}.icon-button{color:#94a3b8}.icon-button:hover{background:#111c2f;color:#f8fafc}.user-menu{color:#cbd5e1}.user-menu:hover{background:#111c2f}.avatar{background:#172554;color:#93c5fd}.service-status{color:#94a3b8}
.main{position:relative;isolation:isolate}
:global(.shared-particle-surface){
  position:relative;
  z-index:1;
  background:
    radial-gradient(circle at 50% 8%,rgba(255,255,255,.1),transparent 34%),
    radial-gradient(circle at 50% 72%,rgba(56,189,248,.07),transparent 48%),
    linear-gradient(145deg,rgba(20,23,27,.82),rgba(31,36,42,.76) 48%,rgba(17,20,24,.84))!important;
  box-shadow:inset 0 1px 0 rgba(255,255,255,.1),inset 0 0 0 1px rgba(255,255,255,.035);
}
:global(.shared-particle-surface.tasks-page),
:global(.shared-particle-surface.dashboard-page),
:global(.shared-particle-surface.upload-page),
:global(.shared-particle-surface.page) {
  box-sizing: border-box;
  padding: 28px clamp(18px, 2.4vw, 34px) 42px !important;
  scrollbar-color: rgba(121, 213, 230, .42) transparent;
  scrollbar-width: thin;
}
:global(.shared-particle-surface::-webkit-scrollbar) { width: 8px; }
:global(.shared-particle-surface::-webkit-scrollbar-track) { background: transparent; }
:global(.shared-particle-surface::-webkit-scrollbar-thumb) {
  border: 2px solid transparent;
  border-radius: 999px;
  background: rgba(121, 213, 230, .34);
  background-clip: padding-box;
}
:global(.shared-particle-surface::-webkit-scrollbar-thumb:hover) {
  background: rgba(121, 213, 230, .56);
  background-clip: padding-box;
}

:global(.shared-particle-surface > .page-heading),
:global(.shared-particle-surface > .page-header),
:global(.shared-particle-surface.page > header),
:global(.shared-particle-surface.scenario-page > .page-header) {
  border: 1px solid rgba(255,255,255,.16) !important;
  border-radius: 14px !important;
  background: linear-gradient(120deg, rgba(255,255,255,.095), rgba(255,255,255,.045)) !important;
  
  
  box-shadow: inset 0 1px 0 rgba(255,255,255,.13), 0 18px 52px rgba(0,0,0,.28) !important;
}
:global(.shared-particle-surface.scenario-page > .page-header),
:global(.shared-particle-surface.rule-config > header),
:global(.shared-particle-surface.upload-page > .page-heading),
:global(.shared-particle-surface.admin-page > .page-heading) {
  min-height: 112px;
  box-sizing: border-box;
  align-items: center !important;
  gap: 20px;
  padding: 22px 24px !important;
}
:global(.shared-particle-surface > .page-heading h1),
:global(.shared-particle-surface > .page-header h1),
:global(.shared-particle-surface.page > header h1),
:global(.shared-particle-surface.scenario-page > .page-header h1) {
  margin: 0 0 5px !important;
  color: #f5f7fa !important;
  font-size: 25px !important;
  font-weight: 700 !important;
  letter-spacing: .01em !important;
}
:global(.shared-particle-surface > .page-heading p),
:global(.shared-particle-surface > .page-header p),
:global(.shared-particle-surface.page > header p),
:global(.shared-particle-surface.scenario-page > .page-header p) {
  margin: 0 !important;
  color: #b7bec8 !important;
  font-size: 12px !important;
}
:global(.shared-particle-surface .eyebrow) { display: none !important; }
:global(.shared-particle-surface > .page-heading .heading-actions),
:global(.shared-particle-surface > .page-heading .hero-actions),
:global(.shared-particle-surface > .page-header .heading-actions) { z-index: 2; }

:global(.shared-particle-surface .summary-grid > .summary-item),
:global(.shared-particle-surface.rule-config > .summary > div) {
  position: relative;
  min-height: 94px;
  padding: 16px 18px !important;
  border: 1px solid rgba(255,255,255,.16) !important;
  border-radius: 14px !important;
  background: linear-gradient(145deg, rgba(255,255,255,.095), rgba(255,255,255,.05)) !important;
  
  
  box-shadow: inset 0 1px 0 rgba(255,255,255,.13), 0 16px 44px rgba(0,0,0,.24) !important;
  transition: border-color .24s cubic-bezier(.4,0,.2,1), background .24s cubic-bezier(.4,0,.2,1), box-shadow .24s cubic-bezier(.4,0,.2,1);
}
:global(.shared-particle-surface .summary-grid > .summary-item:hover),
:global(.shared-particle-surface.rule-config > .summary > div:hover) {
  border-color: rgba(103,232,249,.34) !important;
  background: linear-gradient(145deg, rgba(255,255,255,.13), rgba(103,232,249,.065)) !important;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.17), 0 20px 54px rgba(0,0,0,.3), 0 0 24px rgba(6,182,212,.1) !important;
}
:global(.shared-particle-surface.rule-config > .summary) {
  display: grid !important;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  border: 0 !important;
  background: transparent !important;
}
:global(.shared-particle-surface.rule-config > .summary > div) { min-width: 0 !important; border-right: 1px solid rgba(255,255,255,.16) !important; }
.global-theme-toggle {
  position: fixed;
  z-index: 100;
  bottom: 20px;
  left: 20px;
  display: grid;
  width: 48px;
  height: 48px;
  place-items: center;
  border: 1px solid rgba(103,232,249,.55);
  border-radius: 14px;
  background: linear-gradient(145deg, rgba(20,35,44,.92), rgba(6,182,212,.32));
  box-shadow: inset 0 1px rgba(255,255,255,.18), 0 10px 28px rgba(0,0,0,.42), 0 0 24px rgba(6,182,212,.22);
  color: #a8f3fb;
  cursor: pointer;
  font-size: 21px;
  transition: transform .24s cubic-bezier(.4,0,.2,1), background .24s cubic-bezier(.4,0,.2,1), box-shadow .24s cubic-bezier(.4,0,.2,1);
}
.global-theme-toggle:hover { transform: translateY(-3px); background: linear-gradient(145deg, rgba(20,48,59,.96), rgba(6,182,212,.5)); box-shadow: inset 0 1px rgba(255,255,255,.24), 0 14px 32px rgba(0,0,0,.5), 0 0 30px rgba(6,182,212,.38); }
.global-theme-toggle:active { transform: scale(.9); }
:global(html[data-log-theme="light"]) body { background: #c8d9df !important; color: #17303b !important; }
:global(html[data-log-theme="light"]) {
  color-scheme: light;
  --lm-surface-page: #d7e5e9;
  --lm-surface-panel: rgba(255,255,255,.72);
  --lm-border: rgba(67,98,112,.22);
  --lm-text-primary: #17303b;
  --lm-text-secondary: #55727d;
}
:global(html[data-log-theme="light"] .layout-container),
:global(html[data-log-theme="light"] .workspace),
:global(html[data-log-theme="light"] .main) {
  background: radial-gradient(circle at 70% -20%, rgba(255,255,255,.72), transparent 34%), linear-gradient(145deg, #c2d5dc, #e0ebee 48%, #b9ced6) !important;
  color: #17303b !important;
}
:global(html[data-log-theme="light"] .desktop-aside) { background: rgba(224,237,241,.82) !important; border-right-color: rgba(47,92,110,.2) !important; color: #1c3b47 !important; }
:global(html[data-log-theme="light"] .brand),
:global(html[data-log-theme="light"] .header) { background: rgba(232,243,246,.72) !important; border-color: rgba(47,92,110,.16) !important; }
:global(html[data-log-theme="light"] .brand-copy strong),
:global(html[data-log-theme="light"] .breadcrumb strong),
:global(html[data-log-theme="light"] .user-name) { color: #17303b !important; }
:global(html[data-log-theme="light"] .brand-copy span),
:global(html[data-log-theme="light"] .breadcrumb),
:global(html[data-log-theme="light"] .service-status),
:global(html[data-log-theme="light"] .menu-group-label) { color: #55727d !important; }
:global(html[data-log-theme="light"] .menu .el-menu-item) { color: #486773 !important; }
:global(html[data-log-theme="light"] .menu .el-menu-item:hover) { background: rgba(6,150,180,.1) !important; color: #174b5c !important; }
:global(html[data-log-theme="light"] .menu .el-menu-item.is-active) { background: rgba(6,150,180,.18) !important; color: #09647a !important; }
:global(html[data-log-theme="light"] .shared-particle-surface) {
  background: radial-gradient(circle at 50% 8%, rgba(255,255,255,.64), transparent 34%), linear-gradient(145deg, rgba(215,231,235,.82), rgba(239,246,247,.72) 48%, rgba(198,217,223,.84)) !important;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.75), inset 0 0 0 1px rgba(47,92,110,.1);
}
:global(html[data-log-theme="light"] .shared-particle-surface .page-heading),
:global(html[data-log-theme="light"] .shared-particle-surface .page-header),
:global(html[data-log-theme="light"] .shared-particle-surface.page > header),
:global(html[data-log-theme="light"] .shared-particle-surface .panel),
:global(html[data-log-theme="light"] .shared-particle-surface .tasks-panel),
:global(html[data-log-theme="light"] .shared-particle-surface .summary-item),
:global(html[data-log-theme="light"] .shared-particle-surface .project-workspace),
:global(html[data-log-theme="light"] .shared-particle-surface .result-panel) {
  border-color: rgba(67,98,112,.24) !important;
  background: linear-gradient(145deg, rgba(255,255,255,.5), rgba(224,240,244,.28)) !important;
  box-shadow: inset 0 1px rgba(255,255,255,.82), 0 18px 48px rgba(35,67,83,.16) !important;
}
:global(html[data-log-theme="light"] .shared-particle-surface h1),
:global(html[data-log-theme="light"] .shared-particle-surface h2),
:global(html[data-log-theme="light"] .shared-particle-surface h3),
:global(html[data-log-theme="light"] .shared-particle-surface strong),
:global(html[data-log-theme="light"] .shared-particle-surface td) { color: #17303b !important; }
:global(html[data-log-theme="light"] .shared-particle-surface p),
:global(html[data-log-theme="light"] .shared-particle-surface small),
:global(html[data-log-theme="light"] .shared-particle-surface th) { color: #55727d !important; }
:global(html[data-log-theme="light"] .shared-particle-surface .el-input__wrapper),
:global(html[data-log-theme="light"] .shared-particle-surface .el-select__wrapper),
:global(html[data-log-theme="light"] .shared-particle-surface textarea) { background: rgba(255,255,255,.5) !important; box-shadow: 0 0 0 1px rgba(67,98,112,.24) inset !important; color: #17303b !important; }
:global(html[data-log-theme="light"] .global-theme-toggle) { border-color: rgba(47,92,110,.28); background: linear-gradient(145deg, rgba(255,255,255,.74), rgba(103,201,219,.42)); color: #09647a; box-shadow: inset 0 1px rgba(255,255,255,.9), 0 10px 24px rgba(35,67,83,.22), 0 0 22px rgba(6,150,180,.2); }
:global(html[data-log-theme="light"] .el-popper),
:global(html[data-log-theme="light"] .el-dialog),
:global(html[data-log-theme="light"] .el-message-box),
:global(html[data-log-theme="light"] .el-drawer) { background: rgba(244,250,251,.96) !important; border-color: rgba(67,98,112,.24) !important; color: #17303b !important; }
@media(max-width: 760px) {
  .global-theme-toggle { right: 16px; bottom: 16px; left: auto; width: 44px; height: 44px; }
  :global(.shared-particle-surface.tasks-page),
  :global(.shared-particle-surface.dashboard-page),
  :global(.shared-particle-surface.upload-page),
  :global(.shared-particle-surface.page) { padding: 18px 14px 32px !important; }
  :global(.shared-particle-surface.scenario-page > .page-header),
  :global(.shared-particle-surface.rule-config > header),
  :global(.shared-particle-surface.upload-page > .page-heading),
  :global(.shared-particle-surface.admin-page > .page-heading) {
    min-height: 0;
    align-items: flex-start !important;
    flex-direction: column;
    padding: 18px !important;
  }
  :global(.shared-particle-surface.rule-config > .summary) { grid-template-columns: repeat(2, minmax(0,1fr)); }
}
@media(max-width: 1180px) and (min-width: 761px) {
  :global(.shared-particle-surface.tasks-page),
  :global(.shared-particle-surface.dashboard-page),
  :global(.shared-particle-surface.upload-page),
  :global(.shared-particle-surface.page) { padding: 18px 16px 32px !important; }
  :global(.shared-particle-surface > .page-heading),
  :global(.shared-particle-surface > .page-header),
  :global(.shared-particle-surface.page > header) { gap: 14px; }
}
</style>

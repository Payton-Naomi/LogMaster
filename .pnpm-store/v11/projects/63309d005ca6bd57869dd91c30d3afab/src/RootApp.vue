<script setup>
import { computed, h, onBeforeUnmount, ref } from 'vue'
import { Avatar, ConfigProvider, Menu, Tag } from 'ant-design-vue'
import {
  CloudUploadOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
  SettingOutlined,
  SlidersOutlined,
} from '@ant-design/icons-vue'
import CollectionView from './views/CollectionView.vue'
import HistoryView from './views/HistoryView.vue'
import UploadQueueView from './views/UploadQueueView.vue'
import CatalogView from './views/CatalogView.vue'
import SettingsView from './views/SettingsView.vue'
import { useDesktopService } from './composables/useDesktopService'

const desktop = useDesktopService()
const activeView = ref(['collection'])
const activeKey = computed(() => activeView.value[0])
const now = ref(new Date())
const timer = window.setInterval(() => { now.value = new Date() }, 1000)
onBeforeUnmount(() => window.clearInterval(timer))

const views = [
  { key: 'collection', label: '采集工作台', icon: () => h(DatabaseOutlined) },
  { key: 'history', label: '历史文件', icon: () => h(FolderOpenOutlined) },
  { key: 'uploads', label: '上传队列', icon: () => h(CloudUploadOutlined) },
  { key: 'catalog', label: '配置管理', icon: () => h(SlidersOutlined) },
  { key: 'settings', label: '设置', icon: () => h(SettingOutlined) },
]
const theme = {
  token: {
    colorPrimary: '#16705f',
    colorInfo: '#16705f',
    colorSuccess: '#24865f',
    colorWarning: '#b7791f',
    colorError: '#b74848',
    borderRadius: 6,
    controlHeight: 34,
    fontSize: 14,
    fontFamily: 'LogMasterUI, Arial, "Source Han Sans SC", "Source Han Sans CN", "思源黑体", sans-serif',
  },
  components: {
    Menu: { itemHeight: 52, horizontalItemBorderRadius: 0 },
    Switch: { trackHeight: 24, trackMinWidth: 42, handleSize: 20 },
    Table: { cellPaddingBlockSM: 9, cellPaddingInlineSM: 10 },
  },
}
const selectedStatus = computed(() => desktop.selectedDevice.value?.noLogAlert ? '长时间无日志' : desktop.selectedDevice.value?.enabled ? '采集中' : '已关闭')
const clock = computed(() => {
  const value = now.value
  const pad = (number, size = 2) => String(number).padStart(size, '0')
  return `[${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())} ${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}]`
})
</script>

<template>
  <ConfigProvider :theme="theme">
  <div class="app-shell">
    <header class="topbar">
      <div class="brand"><Avatar class="brand-mark" shape="square" src="/app-icon.png" /><div><strong>LogMaster</strong><small>桌面采集端</small></div></div>
      <Menu v-model:selected-keys="activeView" class="topnav" mode="horizontal" :items="views" />
      <Tag class="service-state" :color="desktop.serviceReady.value ? 'success' : 'error'">{{ desktop.serviceReady.value ? '本地服务正常' : '服务未连接' }}</Tag>
    </header>
    <div class="content-shell">
      <CollectionView v-show="activeKey === 'collection'" :desktop="desktop" />
      <HistoryView v-show="activeKey === 'history'" :devices="desktop.devices.value" :invoke="desktop.invoke" />
      <UploadQueueView v-show="activeKey === 'uploads'" :active="activeKey === 'uploads'" :devices="desktop.devices.value" :queue-status="desktop.queueStatus.value" :invoke="desktop.invoke" />
      <CatalogView v-show="activeKey === 'catalog'" :catalog="desktop.catalog.value" :invoke="desktop.invoke" />
      <SettingsView v-show="activeKey === 'settings'" :settings="desktop.settings.value" :invoke="desktop.invoke" @saved="desktop.refreshAll" />
    </div>
    <footer class="statusbar"><span>当前通道：{{ desktop.selectedDevice.value?.portName || '未选择' }} · {{ selectedStatus }}</span><span v-if="desktop.selectedDevice.value?.configStatus !== 'saved'" class="footer-warning">通道配置未保存</span><time>{{ clock }}</time><span class="status-right">待上传 {{ desktop.currentPendingText.value }} · 待核对 {{ desktop.queueStatus.value.uncertain || 0 }}</span></footer>
  </div>
  </ConfigProvider>
</template>

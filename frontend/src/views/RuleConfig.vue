<template>
  <div class="rule-page">
    <div class="page-heading">
      <div>
        <h1>规则配置</h1>
        <p>管理日志关键字段、适用范围和问题级别</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">新增规则</el-button>
    </div>

    <div class="summary-strip">
      <div><span>规则总数</span><strong>{{ rules.length }}</strong></div>
      <div><span>已启用</span><strong class="success">{{ enabledRules }}</strong></div>
      <div><span>严重规则</span><strong class="danger">{{ criticalRules }}</strong></div>
      <div><span>适用项目</span><strong>联咏自研</strong></div>
      <div class="source-note"><el-icon><DocumentChecked /></el-icon><span>已根据《记录仪自研项目日志关键字段汇总》整理</span></div>
    </div>

    <div class="rule-layout">
      <aside class="category-panel">
        <div class="category-title">规则分类</div>
        <button
          v-for="category in categories"
          :key="category.id"
          type="button"
          class="category-item"
          :class="{ active: activeCategory === category.id }"
          @click="activeCategory = category.id"
        >
          <el-icon><component :is="category.icon" /></el-icon>
          <span>{{ category.name }}</span>
          <b>{{ categoryCount(category.id) }}</b>
        </button>
      </aside>

      <main class="rule-panel">
        <div class="filters">
          <el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="搜索规则名称、关键字或说明" class="search-input" />
          <el-select v-model="scopeFilter" clearable placeholder="全部适用范围" class="scope-filter">
            <el-option label="自研通用" value="自研通用" />
            <el-option label="老项目" value="老项目" />
            <el-option label="新项目" value="新项目" />
            <el-option label="DR4800/5800" value="DR4800/5800" />
            <el-option label="自研电容项目" value="自研电容项目" />
          </el-select>
          <el-select v-model="levelFilter" clearable placeholder="全部级别" class="level-filter">
            <el-option label="严重" value="critical" />
            <el-option label="警告" value="warning" />
            <el-option label="信息" value="info" />
          </el-select>
          <el-button :icon="Refresh" title="重置筛选" @click="resetFilters" />
        </div>

        <el-table :data="pagedRules" class="rule-table">
          <el-table-column label="规则名称" min-width="170">
            <template #default="scope">
              <div class="rule-name">
                <span class="level-dot" :class="scope.row.level" />
                <div><strong>{{ scope.row.name }}</strong><small>{{ categoryName(scope.row.category) }}</small></div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="关键字 / 表达式" min-width="310">
            <template #default="scope"><code class="keyword-code" :title="scope.row.keyword">{{ scope.row.keyword }}</code></template>
          </el-table-column>
          <el-table-column prop="scope" label="适用范围" min-width="125" />
          <el-table-column label="级别" width="90">
            <template #default="scope"><el-tag :type="levelMeta[scope.row.level].type" effect="plain">{{ levelMeta[scope.row.level].label }}</el-tag></template>
          </el-table-column>
          <el-table-column label="状态" width="85">
            <template #default="scope"><el-switch v-model="scope.row.enabled" @change="markChanged" /></template>
          </el-table-column>
          <el-table-column label="操作" width="110" fixed="right">
            <template #default="scope">
              <el-button text circle :icon="Edit" title="编辑规则" @click="openEditDialog(scope.row)" />
              <el-button text circle :icon="Delete" title="删除规则" @click="removeRule(scope.row)" />
            </template>
          </el-table-column>
        </el-table>

        <div class="table-footer">
          <span>显示 {{ pagedRules.length }} 条，共 {{ filteredRules.length }} 条规则</span>
          <div>
            <span class="save-state"><el-icon><CircleCheck /></el-icon>{{ saveState }}</span>
            <el-pagination v-model:current-page="page" :page-size="pageSize" :total="filteredRules.length" layout="prev, pager, next" />
          </div>
        </div>
      </main>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑规则' : '新增规则'" width="560px" destroy-on-close>
      <el-form label-position="top">
        <div class="dialog-grid">
          <el-form-item label="规则名称" required><el-input v-model="ruleForm.name" placeholder="例如：视频丢帧" /></el-form-item>
          <el-form-item label="规则分类" required>
            <el-select v-model="ruleForm.category" style="width: 100%">
              <el-option v-for="item in categories.slice(1)" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="关键字 / 正则表达式" required>
          <el-input v-model="ruleForm.keyword" type="textarea" :rows="3" placeholder="多个关键字可使用 | 分隔" />
        </el-form-item>
        <div class="dialog-grid">
          <el-form-item label="适用范围">
            <el-input v-model="ruleForm.scope" placeholder="自研通用" />
          </el-form-item>
          <el-form-item label="问题级别">
            <el-select v-model="ruleForm.level" style="width: 100%">
              <el-option label="严重" value="critical" />
              <el-option label="警告" value="warning" />
              <el-option label="信息" value="info" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="规则说明"><el-input v-model="ruleForm.description" placeholder="说明命中后代表的日志事件" /></el-form-item>
        <div class="enabled-row"><div><strong>启用规则</strong><span>启用后参与新任务的日志解析</span></div><el-switch v-model="ruleForm.enabled" /></div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRule">保存规则</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, markRaw, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  CircleCheck,
  Connection,
  Delete,
  DocumentChecked,
  Edit,
  Film,
  FolderOpened,
  Monitor,
  Operation,
  Plus,
  Refresh,
  Search,
  SwitchButton,
  Tools
} from '@element-plus/icons-vue'

const categories = [
  { id: 'all', name: '全部规则', icon: markRaw(Operation) },
  { id: 'power', name: '开关机与电源', icon: markRaw(SwitchButton) },
  { id: 'storage', name: 'SD 卡与存储', icon: markRaw(FolderOpened) },
  { id: 'recording', name: '录像与视频', icon: markRaw(Film) },
  { id: 'system', name: '系统与稳定性', icon: markRaw(Monitor) },
  { id: 'connectivity', name: '连接与通信', icon: markRaw(Connection) },
  { id: 'feature', name: '设备功能', icon: markRaw(Operation) },
  { id: 'tool', name: '辅助工具', icon: markRaw(Tools) }
]

const initialRules = [
  { id: 1, name: '老项目开关机', category: 'power', keyword: 'starting | need_power_off', scope: '老项目', level: 'info', enabled: true, description: '识别老项目开关机过程' },
  { id: 2, name: '新项目开关机', category: 'power', keyword: 's|shutdown:', scope: '新项目', level: 'info', enabled: true, description: '适用于 DR2820 等新项目' },
  { id: 3, name: 'ACC 状态变化', category: 'power', keyword: 'AccOff|AccON|trans to|acc', scope: '自研通用', level: 'info', enabled: true, description: '识别 ACC 通断电与状态切换' },
  { id: 4, name: '进入缩时模式', category: 'power', keyword: 'ready to enter lapse mode', scope: '自研通用', level: 'info', enabled: true, description: '识别停车监控缩时模式' },
  { id: 5, name: '系统模式', category: 'power', keyword: 'FL_BOOT_SYSMODE|XA_NORMAL_SYSMODE|XA_GUIDER_SYSMODE|XA_FAC_SYSMODE|XA_FAC_SMT_SYSMODE', scope: '自研通用', level: 'info', enabled: true, description: '解析正常、新手引导、厂测等系统模式' },
  { id: 6, name: '开机类型', category: 'power', keyword: 'POWER_ID_PSW1|POWER_ID_PSW2|POWER_ID_PSW3|POWER_ID_PSW4|POWER_ID_HWRT|POWER_ID_SWRT|POWER_ID_4G', scope: '自研通用', level: 'warning', enabled: true, description: '区分按键、ACC、USB、GSensor、硬件或软件看门狗等唤醒源' },
  { id: 7, name: '关机类型', category: 'power', keyword: 'SYSTEMMNG_SHUTDOWN_HIGH_TEMP_E|SYSTEMMNG_SHUTDOWN_POWER_PLUGOUT_E|SYSTEMMNG_SHUTDOWN_ACC_OFF_E|SYSTEMMNG_SHUTDOWN_VOL_LOW_E|SYSTEMMNG_SHUTDOWN_REBOOT_E', scope: '自研通用', level: 'warning', enabled: true, description: '解析高温、断电、ACC OFF、低电及重启等关机原因' },
  { id: 8, name: '通断电文件系统异常', category: 'power', keyword: 'ubi0 e|ubi1 e|ubi2 e|uib3 e|ubifs e', scope: '自研电容项目', level: 'critical', enabled: true, description: '开关机通断电判断是否存在 UBI 异常' },
  { id: 9, name: 'SD 卡状态', category: 'storage', keyword: 'sd state|STGMNG_SD_NOEXSIT_STATE|STGMNG_SD_INSERT_STATE|STGMNG_SD_ERROR_STATE|STGMNG_SD_FSOK_STATE|STGMNG_SD_PLUGOUT_STATE', scope: '自研通用', level: 'warning', enabled: true, description: '解析 SD 卡插入、异常、可用、拔出等状态' },
  { id: 10, name: '提示格式化存储卡', category: 'storage', keyword: 's_FacParam.sd_state   = 2', scope: '自研通用', level: 'warning', enabled: true, description: '卡状态为需要格式化' },
  { id: 11, name: '卡内存在非记录仪文件', category: 'storage', keyword: 's_FacParam.sd_state   = 11', scope: '自研通用', level: 'warning', enabled: true, description: '检测到无法识别的文件' },
  { id: 12, name: '存储卡性能不足', category: 'storage', keyword: 'speed monitor state cb, state', scope: '自研通用', level: 'warning', enabled: true, description: '提示更换性能更高的存储卡' },
  { id: 13, name: '文件系统异常', category: 'storage', keyword: 'FAT-fs', scope: '自研通用', level: 'critical', enabled: true, description: '大量打印时判断文件系统异常；带电拔卡场景可忽略' },
  { id: 14, name: '视频录制起止', category: 'recording', keyword: 'File Start|File End', scope: '自研通用', level: 'info', enabled: true, description: '匹配视频文件开始和结束' },
  { id: 15, name: '视频丢帧', category: 'recording', keyword: 'queue is full!!! drop frame', scope: '自研通用', level: 'critical', enabled: true, description: '编码队列已满导致视频帧丢失' },
  { id: 16, name: '累计丢帧超限', category: 'recording', keyword: 'SD write detected frame loss for', scope: '自研通用', level: 'critical', enabled: true, description: '30 秒内累计丢帧时间达到 15000ms' },
  { id: 17, name: 'MP4 写文件失败', category: 'recording', keyword: 'XA_MP4_Write failed', scope: '自研通用', level: 'critical', enabled: true, description: 'MP4 文件写入失败' },
  { id: 18, name: '紧急视频录制卡住', category: 'recording', keyword: 'Failed to create falloc directory: /mnt/sd/.tmp', scope: '自研通用', level: 'critical', enabled: true, description: '无法创建预分配目录导致紧急录像卡住' },
  { id: 19, name: '后录相关', category: 'recording', keyword: 'AHD', scope: '自研通用', level: 'info', enabled: true, description: '筛选 AHD 后录模块日志' },
  { id: 20, name: '系统崩溃', category: 'system', keyword: 'backtrace', scope: '自研通用', level: 'critical', enabled: true, description: '检测系统调用栈与崩溃信息' },
  { id: 21, name: '应用程序崩溃', category: 'system', keyword: 'Log_Signal_Data', scope: '自研通用', level: 'critical', enabled: true, description: '检测应用信号异常和崩溃' },
  { id: 22, name: '看门狗重启', category: 'system', keyword: '2f0050080 :|00000001 00000000 00000000 0000000', scope: 'DR4800/5800', level: 'critical', enabled: true, description: '根据寄存器反馈判断看门狗重启' },
  { id: 23, name: 'CPU 温度', category: 'system', keyword: 'cpu_temp', scope: '自研通用', level: 'warning', enabled: true, description: '提取 CPU 温度用于高温分析' },
  { id: 24, name: 'Wi-Fi 信息', category: 'connectivity', keyword: 'Wifi--------', scope: '自研通用', level: 'info', enabled: true, description: '提取 Wi-Fi 名称及连接信息' },
  { id: 25, name: '蓝牙相关', category: 'connectivity', keyword: 'blemng', scope: '自研通用', level: 'info', enabled: true, description: '筛选蓝牙管理模块日志' },
  { id: 26, name: 'GPS 定位', category: 'connectivity', keyword: 'RMC:', scope: '自研通用', level: 'info', enabled: true, description: '提取 GPS RMC 定位语句' },
  { id: 27, name: 'ADAS 相关', category: 'feature', keyword: 'zdd Adas', scope: '自研通用', level: 'info', enabled: true, description: '筛选 ADAS 模块日志' },
  { id: 28, name: '声音播放', category: 'feature', keyword: 'voice play', scope: '自研通用', level: 'info', enabled: true, description: '检查提示音播放状态' },
  { id: 29, name: 'OTA 升级', category: 'feature', keyword: 'OTA start', scope: '自研通用', level: 'warning', enabled: true, description: '识别 OTA 升级开始事件' },
  { id: 30, name: '疲劳驾驶提醒', category: 'feature', keyword: 'adas_driver_take_a_rest.aac|text=开了很久了，休息一下再赶路吧', scope: '自研通用', level: 'info', enabled: true, description: '识别疲劳驾驶语音提醒' },
  { id: 31, name: '联咏出流信息命令', category: 'tool', keyword: 'cat /proc/hdal/comm/info|cat /proc/hdal/venc/info|cat /proc/hdal/vprc/info|cat /proc/hdal/vcap/info', scope: '自研通用', level: 'info', enabled: false, description: '用于抓取联咏平台各模块出流信息' },
  { id: 32, name: '串口导出日志命令', category: 'tool', keyword: 'cp -r /mnt/other/log/ /mnt/sd | sync', scope: '自研通用', level: 'info', enabled: false, description: '通过串口将设备日志复制到 SD 卡' }
]

const rules = ref(initialRules)
const activeCategory = ref('all')
const keyword = ref('')
const scopeFilter = ref('')
const levelFilter = ref('')
const page = ref(1)
const pageSize = 12
const dialogVisible = ref(false)
const editingId = ref(null)
const saveState = ref('所有更改已保存')
const ruleForm = reactive({ name: '', category: 'system', keyword: '', scope: '自研通用', level: 'warning', description: '', enabled: true })
const levelMeta = { critical: { label: '严重', type: 'danger' }, warning: { label: '警告', type: 'warning' }, info: { label: '信息', type: 'info' } }

const enabledRules = computed(() => rules.value.filter((item) => item.enabled).length)
const criticalRules = computed(() => rules.value.filter((item) => item.level === 'critical').length)
const filteredRules = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  return rules.value.filter((rule) => {
    const matchesCategory = activeCategory.value === 'all' || rule.category === activeCategory.value
    const matchesSearch = !search || [rule.name, rule.keyword, rule.description].some((value) => value.toLowerCase().includes(search))
    const matchesScope = !scopeFilter.value || rule.scope === scopeFilter.value
    const matchesLevel = !levelFilter.value || rule.level === levelFilter.value
    return matchesCategory && matchesSearch && matchesScope && matchesLevel
  })
})
const pagedRules = computed(() => filteredRules.value.slice((page.value - 1) * pageSize, page.value * pageSize))

watch([activeCategory, keyword, scopeFilter, levelFilter], () => { page.value = 1 })
const categoryCount = (id) => id === 'all' ? rules.value.length : rules.value.filter((item) => item.category === id).length
const categoryName = (id) => categories.find((item) => item.id === id)?.name || '其他'

const resetForm = () => Object.assign(ruleForm, { name: '', category: activeCategory.value === 'all' ? 'system' : activeCategory.value, keyword: '', scope: '自研通用', level: 'warning', description: '', enabled: true })
const openCreateDialog = () => { editingId.value = null; resetForm(); dialogVisible.value = true }
const openEditDialog = (rule) => { editingId.value = rule.id; Object.assign(ruleForm, rule); dialogVisible.value = true }

const saveRule = () => {
  if (!ruleForm.name.trim() || !ruleForm.keyword.trim()) {
    ElMessage.warning('请填写规则名称和关键字')
    return
  }
  if (editingId.value) {
    const index = rules.value.findIndex((item) => item.id === editingId.value)
    rules.value.splice(index, 1, { ...rules.value[index], ...ruleForm })
  } else {
    rules.value.unshift({ id: Date.now(), ...ruleForm })
  }
  dialogVisible.value = false
  saveState.value = '所有更改已保存'
  ElMessage.success(editingId.value ? '规则已更新' : '规则已创建')
}

const removeRule = async (rule) => {
  await ElMessageBox.confirm(`确定删除规则“${rule.name}”吗？`, '删除规则', { type: 'warning' })
  rules.value = rules.value.filter((item) => item.id !== rule.id)
  ElMessage.success('规则已删除')
}

const markChanged = () => {
  saveState.value = `已自动保存于 ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`
}
const resetFilters = () => { keyword.value = ''; scopeFilter.value = ''; levelFilter.value = ''; activeCategory.value = 'all' }
</script>

<style scoped>
.rule-page { height: 100%; overflow: auto; color: #1f2937; box-sizing: border-box; }
.page-heading { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: 18px; }
.page-heading h1 { margin: 0; font-size: 22px; line-height: 1.4; letter-spacing: 0; }
.page-heading p { margin: 5px 0 0; color: #7a8493; font-size: 14px; }

.summary-strip { display: flex; align-items: stretch; margin-bottom: 16px; padding: 14px 18px; border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }
.summary-strip > div:not(.source-note) { display: flex; min-width: 120px; flex-direction: column; gap: 5px; padding-right: 24px; margin-right: 24px; border-right: 1px solid #edf0f3; }
.summary-strip span { color: #7a8493; font-size: 12px; }
.summary-strip strong { color: #253044; font-size: 19px; }
.summary-strip strong.success { color: #2f9275; }
.summary-strip strong.danger { color: #d95858; }
.source-note { display: flex; align-items: center; gap: 7px; margin-left: auto; color: #657184; }
.source-note .el-icon { color: #3478dc; font-size: 18px; }

.rule-layout { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 16px; min-height: calc(100% - 146px); }
.category-panel, .rule-panel { border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; box-sizing: border-box; }
.category-panel { padding: 14px 10px; }
.category-title { padding: 2px 10px 12px; border-bottom: 1px solid #edf0f3; color: #485467; font-size: 13px; font-weight: 600; }
.category-item { display: grid; width: 100%; grid-template-columns: 22px minmax(0, 1fr) auto; align-items: center; gap: 8px; margin-top: 5px; padding: 10px; border: 0; border-radius: 4px; background: transparent; color: #596273; text-align: left; cursor: pointer; }
.category-item:hover { background: #f7f9fc; }
.category-item.active { background: #edf4ff; color: #2868ca; font-weight: 600; }
.category-item .el-icon { font-size: 17px; }
.category-item span { font-size: 13px; }
.category-item b { color: #8b95a3; font-size: 11px; font-weight: 500; }

.rule-panel { min-width: 0; padding: 16px; }
.filters { display: flex; align-items: center; gap: 9px; margin-bottom: 14px; }
.search-input { width: min(360px, 45%); }
.scope-filter { width: 150px; }
.level-filter { width: 120px; }
.rule-panel :deep(.el-table__header th) { background: #f8fafc; color: #667085; font-weight: 600; }
.rule-name { display: flex; align-items: center; gap: 9px; }
.level-dot { width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; }
.level-dot.critical { background: #d95858; }
.level-dot.warning { background: #d99a32; }
.level-dot.info { background: #4a82d4; }
.rule-name > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.rule-name strong { overflow: hidden; color: #253044; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.rule-name small { color: #9099a6; font-size: 11px; }
.keyword-code { display: block; overflow: hidden; max-width: 100%; color: #38475e; font: 12px Consolas, monospace; text-overflow: ellipsis; white-space: nowrap; }
.table-footer { display: flex; align-items: center; justify-content: space-between; padding-top: 15px; color: #7a8493; font-size: 12px; }
.table-footer > div, .save-state { display: flex; align-items: center; }
.table-footer > div { gap: 18px; }
.save-state { gap: 5px; color: #6f7b8b; }
.save-state .el-icon { color: #2f9275; }

.dialog-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.enabled-row { display: flex; align-items: center; justify-content: space-between; padding: 12px 0 2px; }
.enabled-row > div { display: flex; flex-direction: column; gap: 4px; }
.enabled-row strong { font-size: 14px; }
.enabled-row span { color: #8a94a3; font-size: 12px; }

@media (max-width: 1050px) {
  .summary-strip { overflow-x: auto; }
  .source-note { display: none; }
  .rule-layout { grid-template-columns: 1fr; }
  .category-panel { display: flex; overflow-x: auto; gap: 5px; }
  .category-title { display: none; }
  .category-item { width: auto; min-width: max-content; margin-top: 0; }
}

@media (max-width: 680px) {
  .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }
  .filters { flex-wrap: wrap; }
  .search-input, .scope-filter, .level-filter { width: 100%; }
  .summary-strip > div:not(.source-note) { min-width: 96px; }
  .dialog-grid { grid-template-columns: 1fr; }
  .table-footer { align-items: flex-start; flex-direction: column; gap: 10px; }
  .save-state { display: none; }
}
</style>

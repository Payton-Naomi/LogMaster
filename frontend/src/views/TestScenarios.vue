<template>
  <div class="scenario-page">
    <div class="page-heading">
      <div>
        <h1>测试场景</h1>
        <p>为不同测试类型配置需要关注的日志事件和判定规则</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="showCreateTip">新增场景</el-button>
    </div>

    <div class="scenario-layout">
      <aside class="scenario-list-panel">
        <div class="panel-title">
          <h2>场景列表</h2>
          <span>{{ scenarios.length }}</span>
        </div>
        <button
          v-for="scene in scenarios"
          :key="scene.id"
          class="scenario-item"
          :class="{ active: selectedId === scene.id }"
          type="button"
          @click="selectedId = scene.id"
        >
          <span class="scene-icon" :class="scene.color">
            <el-icon><component :is="scene.icon" /></el-icon>
          </span>
          <span class="scene-copy">
            <strong>{{ scene.name }}</strong>
            <small>{{ scene.description }}</small>
          </span>
          <span class="rule-count">{{ enabledCount(scene) }}/{{ scene.checks.length }}</span>
        </button>
      </aside>

      <main class="scenario-editor">
        <div class="editor-header">
          <div class="editor-identity">
            <span class="scene-icon large" :class="activeScenario.color">
              <el-icon><component :is="activeScenario.icon" /></el-icon>
            </span>
            <div>
              <div class="title-line">
                <h2>{{ activeScenario.name }}</h2>
                <el-tag type="success" effect="plain">已启用</el-tag>
              </div>
              <p>{{ activeScenario.description }}</p>
            </div>
          </div>
          <el-button :icon="CopyDocument" @click="copyScenario">复制场景</el-button>
        </div>

        <div class="scene-settings">
          <el-form label-position="top">
            <div class="settings-grid">
              <el-form-item label="场景名称">
                <el-input v-model="activeScenario.name" />
              </el-form-item>
              <el-form-item label="结果判定">
                <el-select v-model="activeScenario.judgement" style="width: 100%">
                  <el-option label="任一严重异常即失败" value="any-error" />
                  <el-option label="全部规则通过才成功" value="all-pass" />
                  <el-option label="仅生成报告，不判定" value="report-only" />
                </el-select>
              </el-form-item>
            </div>
          </el-form>
        </div>

        <div class="checks-heading">
          <div>
            <h3>解析关注项</h3>
            <p>启用的关注项会在日志解析时自动检测</p>
          </div>
          <span>已启用 {{ enabledCount(activeScenario) }} 项</span>
        </div>

        <div class="check-list">
          <div v-for="check in activeScenario.checks" :key="check.id" class="check-row" :class="{ disabled: !check.enabled }">
            <div class="check-toggle"><el-switch v-model="check.enabled" /></div>
            <div class="check-main">
              <div class="check-title">
                <strong>{{ check.name }}</strong>
                <el-tag :type="severityType(check.severity)" size="small" effect="plain">{{ severityLabel(check.severity) }}</el-tag>
              </div>
              <p>{{ check.description }}</p>
              <div class="keyword-list">
                <span v-for="keyword in check.keywords" :key="keyword">{{ keyword }}</span>
              </div>
            </div>
            <el-select v-model="check.severity" class="severity-select" :disabled="!check.enabled">
              <el-option label="严重" value="critical" />
              <el-option label="警告" value="warning" />
              <el-option label="提示" value="info" />
            </el-select>
            <el-button text circle :icon="Edit" :disabled="!check.enabled" title="编辑规则" @click="editCheck(check)" />
          </div>
        </div>

        <div class="editor-footer">
          <div class="save-status">
            <el-icon><CircleCheck /></el-icon>
            <span>{{ saveStatus }}</span>
          </div>
          <div>
            <el-button @click="restoreDefaults">恢复默认</el-button>
            <el-button type="primary" @click="saveScenario">保存配置</el-button>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed, markRaw, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  CircleCheck,
  CopyDocument,
  Edit,
  Film,
  Plus,
  SwitchButton
} from '@element-plus/icons-vue'

const createScenarios = () => [
  {
    id: 'power-cycle',
    name: '开关机测试',
    description: '检查设备启动、关机和循环重启过程中的关键异常',
    icon: markRaw(SwitchButton),
    color: 'blue',
    judgement: 'any-error',
    checks: [
      {
        id: 'unexpected-reboot',
        name: '异常重启',
        description: '识别看门狗复位、系统崩溃、内核异常等非预期重启行为',
        severity: 'critical',
        enabled: true,
        keywords: ['watchdog reset', 'kernel panic', 'unexpected reboot']
      },
      {
        id: 'factory-reset',
        name: '恢复出厂设置',
        description: '检测测试过程中是否发生恢复出厂设置或用户数据被清除',
        severity: 'critical',
        enabled: true,
        keywords: ['factory reset', 'wipe_data', 'restore default']
      },
      {
        id: 'slow-boot',
        name: '启动耗时异常',
        description: '统计启动阶段耗时，超过 60 秒时生成警告',
        severity: 'warning',
        enabled: true,
        keywords: ['boot_completed', 'system ready']
      },
      {
        id: 'shutdown-incomplete',
        name: '关机流程不完整',
        description: '检查关机关键服务是否按顺序退出并完成数据落盘',
        severity: 'warning',
        enabled: false,
        keywords: ['shutdown timeout', 'sync failed']
      }
    ]
  },
  {
    id: 'sd-card-aging',
    name: 'SD 卡挂测',
    description: '分析长时间录像过程中的存储、视频连续性和写入稳定性',
    icon: markRaw(Film),
    color: 'green',
    judgement: 'any-error',
    checks: [
      {
        id: 'frame-drop',
        name: '视频丢帧',
        description: '检测编码帧序号不连续、写帧失败或帧率低于预期',
        severity: 'critical',
        enabled: true,
        keywords: ['frame dropped', 'frame sequence gap', 'write frame failed']
      },
      {
        id: 'recording-interrupted',
        name: '录像异常中断',
        description: '识别录像未正常封装、文件损坏或录制任务意外退出',
        severity: 'critical',
        enabled: true,
        keywords: ['record stopped unexpectedly', 'muxer error', 'video corrupted']
      },
      {
        id: 'sd-io-error',
        name: 'SD 卡读写异常',
        description: '检测 I/O 错误、SD 卡掉卡、重新挂载和文件系统错误',
        severity: 'critical',
        enabled: true,
        keywords: ['I/O error', 'sdcard unmounted', 'filesystem error']
      },
      {
        id: 'write-latency',
        name: '写入延迟过高',
        description: '统计视频块写入耗时，持续高延迟时生成警告',
        severity: 'warning',
        enabled: true,
        keywords: ['write latency', 'slow storage']
      }
    ]
  }
]

const scenarios = ref(createScenarios())
const selectedId = ref('power-cycle')
const saveStatus = ref('配置尚未修改')
const activeScenario = computed(() => scenarios.value.find((item) => item.id === selectedId.value) || scenarios.value[0])

const enabledCount = (scene) => scene.checks.filter((check) => check.enabled).length
const severityType = (severity) => ({ critical: 'danger', warning: 'warning', info: 'info' })[severity]
const severityLabel = (severity) => ({ critical: '严重', warning: '警告', info: '提示' })[severity]

const saveScenario = () => {
  localStorage.setItem('logmaster-test-scenarios', JSON.stringify(scenarios.value))
  saveStatus.value = `已保存于 ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`
  ElMessage.success('测试场景配置已保存')
}

const restoreDefaults = () => {
  const defaults = createScenarios()
  const defaultScene = defaults.find((item) => item.id === selectedId.value)
  const index = scenarios.value.findIndex((item) => item.id === selectedId.value)
  scenarios.value.splice(index, 1, defaultScene)
  saveStatus.value = '已恢复默认配置，保存后生效'
}

const editCheck = (check) => ElMessage.info(`“${check.name}”的关键词编辑器将在后端规则接口对接时启用`)
const copyScenario = () => ElMessage.info('场景复制功能已预留')
const showCreateTip = () => ElMessage.info('后续可在这里添加更多测试类型')
</script>

<style scoped>
.scenario-page {
  height: 100%;
  overflow: auto;
  color: #1f2937;
  box-sizing: border-box;
}

.page-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 18px;
}

.page-heading h1 { margin: 0; font-size: 22px; line-height: 1.4; letter-spacing: 0; }
.page-heading p { margin: 5px 0 0; color: #7a8493; font-size: 14px; }

.scenario-layout {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 18px;
  min-height: calc(100% - 76px);
}

.scenario-list-panel,
.scenario-editor {
  background: #fff;
  border: 1px solid #dfe3e8;
  border-radius: 6px;
  box-sizing: border-box;
}

.scenario-list-panel { padding: 18px 12px; }

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px 13px;
  border-bottom: 1px solid #edf0f3;
}

.panel-title h2 { margin: 0; font-size: 15px; letter-spacing: 0; }
.panel-title span { color: #8b95a3; font-size: 12px; }

.scenario-item {
  display: grid;
  width: 100%;
  grid-template-columns: 40px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  padding: 13px 10px;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.scenario-item:hover { background: #f7f9fc; }
.scenario-item.active { border-color: #b9d2f7; background: #f1f6fd; }

.scene-icon {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 5px;
  font-size: 20px;
}

.scene-icon.blue { color: #3478dc; background: #eaf2ff; }
.scene-icon.green { color: #2e9275; background: #e9f7f2; }
.scene-icon.large { width: 46px; height: 46px; font-size: 23px; }

.scene-copy { display: flex; min-width: 0; flex-direction: column; gap: 5px; }
.scene-copy strong { font-size: 14px; }
.scene-copy small { overflow: hidden; color: #8a94a3; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.rule-count { color: #7b8695; font-size: 11px; }

.scenario-editor { min-width: 0; padding: 22px; }

.editor-header,
.editor-identity,
.title-line,
.checks-heading,
.editor-footer,
.save-status {
  display: flex;
  align-items: center;
}

.editor-header { justify-content: space-between; padding-bottom: 20px; border-bottom: 1px solid #edf0f3; }
.editor-identity { min-width: 0; gap: 12px; }
.title-line { gap: 10px; }
.title-line h2 { margin: 0; font-size: 19px; letter-spacing: 0; }
.editor-identity p { margin: 5px 0 0; color: #7b8695; font-size: 13px; }

.scene-settings { padding: 18px 0 4px; border-bottom: 1px solid #edf0f3; }
.settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.scene-settings :deep(.el-form-item) { margin-bottom: 14px; }
.scene-settings :deep(.el-form-item__label) { padding-bottom: 6px; color: #596273; line-height: 20px; }
.scene-settings :deep(.el-input__wrapper),
.scene-settings :deep(.el-select__wrapper) { min-height: 40px; }

.checks-heading { justify-content: space-between; margin: 20px 0 10px; }
.checks-heading h3 { margin: 0; font-size: 15px; letter-spacing: 0; }
.checks-heading p { margin: 4px 0 0; color: #8a94a3; font-size: 12px; }
.checks-heading > span { color: #667085; font-size: 12px; }

.check-list { border-top: 1px solid #edf0f3; }

.check-row {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) 92px 34px;
  min-height: 100px;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid #edf0f3;
  transition: opacity 0.2s;
}

.check-row.disabled { opacity: 0.55; }
.check-main { min-width: 0; }
.check-title { display: flex; align-items: center; gap: 8px; }
.check-title strong { font-size: 14px; }
.check-main p { margin: 6px 0 8px; color: #7d8795; font-size: 12px; line-height: 1.5; }
.keyword-list { display: flex; overflow: hidden; gap: 6px; }
.keyword-list span { overflow: hidden; padding: 3px 7px; border-radius: 3px; background: #f2f4f7; color: #667085; font: 11px Consolas, monospace; text-overflow: ellipsis; white-space: nowrap; }
.severity-select { width: 92px; }

.editor-footer { justify-content: space-between; padding-top: 20px; }
.save-status { gap: 6px; color: #7d8795; font-size: 12px; }
.save-status .el-icon { color: #2e9275; }

@media (max-width: 980px) {
  .scenario-layout { grid-template-columns: 1fr; }
  .scenario-list-panel { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .panel-title { grid-column: 1 / -1; }
  .scenario-item { margin-top: 0; }
}

@media (max-width: 680px) {
  .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }
  .scenario-list-panel { display: block; }
  .scenario-item { margin-top: 8px; }
  .scenario-editor { padding: 16px; }
  .editor-header { align-items: flex-start; flex-direction: column; gap: 14px; }
  .settings-grid { grid-template-columns: 1fr; }
  .check-row { grid-template-columns: 38px minmax(0, 1fr) 34px; padding: 12px 0; }
  .severity-select { display: none; }
  .keyword-list { flex-wrap: wrap; }
  .editor-footer { align-items: flex-start; flex-direction: column; gap: 14px; }
}
</style>

<template>
  <div class="detail-page">
    <div class="page-heading">
      <div class="heading-left">
        <el-button text circle :icon="ArrowLeft" title="返回任务列表" @click="router.push('/tasks')" />
        <div><h1>{{ task.name }}</h1><p>{{ task.id }}</p></div>
      </div>
      <div class="heading-actions">
        <el-button :icon="RefreshRight" @click="restartTask">重新解析</el-button>
        <el-button type="primary" :icon="DataAnalysis" @click="router.push(`/analysis/${task.id}`)">查看解析结果</el-button>
      </div>
    </div>

    <div class="overview-grid">
      <section class="task-overview">
        <div class="status-line">
          <div><el-tag type="success" effect="plain">解析完成</el-tag><el-tag type="info" effect="plain">{{ task.scene }}</el-tag></div>
          <span>完成于 {{ task.finishedAt }}</span>
        </div>
        <div class="meta-grid">
          <div><span>所属项目</span><strong>{{ task.project }}</strong></div>
          <div><span>日志文件</span><strong>{{ task.fileCount }} 个 / {{ task.size }}</strong></div>
          <div><span>日志行数</span><strong>{{ task.lines.toLocaleString() }}</strong></div>
          <div><span>解析耗时</span><strong>{{ task.duration }}</strong></div>
          <div><span>字符编码</span><strong>UTF-8</strong></div>
          <div><span>解析规则</span><strong>联咏自研 · 开关机测试</strong></div>
        </div>
      </section>

      <section class="result-overview">
        <div class="result-title"><span class="result-icon"><el-icon><WarningFilled /></el-icon></span><div><span>测试结论</span><strong>发现异常，需要关注</strong></div></div>
        <div class="result-counts">
          <div><strong class="danger">6</strong><span>严重</span></div>
          <div><strong class="warning">14</strong><span>警告</span></div>
          <div><strong>21</strong><span>命中规则</span></div>
        </div>
      </section>
    </div>

    <div class="content-grid">
      <div>
        <section class="panel">
          <div class="panel-heading"><div><h2>解析流程</h2><p>任务各处理阶段及耗时</p></div></div>
          <div class="process-list">
            <div v-for="step in processSteps" :key="step.name" class="process-step">
              <span class="step-check"><el-icon><Check /></el-icon></span>
              <div><strong>{{ step.name }}</strong><span>{{ step.description }}</span></div>
              <time>{{ step.duration }}</time>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-heading"><div><h2>日志文件</h2><p>本次任务包含的原始日志</p></div><span>{{ files.length }} 项</span></div>
          <div class="file-list">
            <div v-for="file in files" :key="file.name" class="file-row">
              <span class="file-icon"><el-icon><Document /></el-icon></span>
              <div><strong>{{ file.name }}</strong><span>{{ file.path }}</span></div>
              <span>{{ file.size }}</span>
              <el-tag type="success" size="small" effect="plain">已解析</el-tag>
            </div>
          </div>
        </section>
      </div>

      <div>
        <section class="panel hit-panel">
          <div class="panel-heading"><div><h2>规则命中摘要</h2><p>按关键字段聚合的命中次数</p></div><el-button type="primary" link @click="router.push(`/analysis/${task.id}`)">全部结果</el-button></div>
          <div class="hit-list">
            <div v-for="item in ruleHits" :key="item.name" class="hit-row">
              <span class="level-mark" :class="item.level" />
              <div><strong>{{ item.name }}</strong><code>{{ item.keyword }}</code></div>
              <b>{{ item.count }}</b>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-heading"><div><h2>任务日志</h2><p>解析服务运行记录</p></div></div>
          <div class="service-log">
            <div v-for="line in serviceLogs" :key="line.time"><time>{{ line.time }}</time><span :class="line.level">{{ line.level }}</span><p>{{ line.message }}</p></div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Check, DataAnalysis, Document, RefreshRight, WarningFilled } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const task = reactive({ id: route.params.taskId, name: 'DR2860_稳定性回归_0716', project: 'DR2860', scene: '开关机测试', fileCount: 4, size: '286.4 MB', lines: 324580, duration: '8.4 秒', finishedAt: '2026-07-16 14:32:18' })
const processSteps = [
  { name: '读取与解压日志', description: '识别 4 个日志文件，自动检测 UTF-8 编码', duration: '1.2 秒' },
  { name: '时间线标准化', description: '提取并排序 324,580 行有效日志', duration: '2.1 秒' },
  { name: '匹配场景规则', description: '执行开关机测试场景的 8 项关注规则', duration: '3.8 秒' },
  { name: '聚合与生成结果', description: '合并重复事件并生成问题摘要', duration: '1.3 秒' }
]
const files = [
  { name: 'system_20260716_1401.log', path: 'DR2860/system/', size: '84.6 MB' },
  { name: 'kernel_20260716_1401.log', path: 'DR2860/kernel/', size: '46.2 MB' },
  { name: 'app_20260716_1401.log', path: 'DR2860/app/', size: '112.8 MB' },
  { name: 'boot_history.txt', path: 'DR2860/', size: '42.8 MB' }
]
const ruleHits = [
  { name: '异常重启', keyword: 'POWER_ID_SWRT | 2f0050080 :', count: 3, level: 'critical' },
  { name: '系统崩溃', keyword: 'backtrace', count: 2, level: 'critical' },
  { name: '通断电文件系统异常', keyword: 'ubi0 e | ubifs e', count: 1, level: 'critical' },
  { name: 'ACC 状态变化', keyword: 'AccOff | AccON | trans to', count: 12, level: 'info' },
  { name: '关机类型', keyword: 'SYSTEMMNG_SHUTDOWN_*', count: 8, level: 'warning' }
]
const serviceLogs = [
  { time: '14:32:09', level: 'INFO', message: '日志文件校验完成，共 4 个文件' },
  { time: '14:32:12', level: 'INFO', message: '已加载开关机测试规则，共 8 项' },
  { time: '14:32:16', level: 'WARN', message: '命中 3 条异常重启记录' },
  { time: '14:32:18', level: 'INFO', message: '解析任务完成，生成 20 项问题' }
]
const restartTask = () => ElMessage.success('任务已重新加入解析队列')
</script>

<style scoped>
.detail-page { height: 100%; overflow: auto; color: #1f2937; box-sizing: border-box; }
.page-heading, .heading-left, .heading-actions, .status-line, .result-title, .panel-heading { display: flex; align-items: center; }
.page-heading { justify-content: space-between; margin-bottom: 18px; }
.heading-left { gap: 8px; }
.heading-left h1 { margin: 0; font-size: 21px; letter-spacing: 0; }
.heading-left p { margin: 4px 0 0; color: #8a94a3; font: 12px Consolas, monospace; }
.heading-actions { gap: 8px; }
.overview-grid { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(300px, .8fr); gap: 16px; margin-bottom: 16px; }
.task-overview, .result-overview, .panel { border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }
.task-overview { padding: 18px 20px; }
.status-line { justify-content: space-between; padding-bottom: 15px; border-bottom: 1px solid #edf0f3; }
.status-line > div { display: flex; gap: 7px; }
.status-line > span { color: #8a94a3; font-size: 12px; }
.meta-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 18px 24px; padding-top: 16px; }
.meta-grid > div { display: flex; min-width: 0; flex-direction: column; gap: 5px; }
.meta-grid span { color: #8a94a3; font-size: 11px; }
.meta-grid strong { overflow: hidden; color: #344054; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.result-overview { padding: 20px; }
.result-title { gap: 11px; }
.result-icon { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 5px; background: #fff0f0; color: #d95858; font-size: 22px; }
.result-title > div { display: flex; flex-direction: column; gap: 4px; }
.result-title span { color: #8a94a3; font-size: 11px; }
.result-title strong { color: #b94242; font-size: 15px; }
.result-counts { display: grid; grid-template-columns: repeat(3, 1fr); margin-top: 20px; padding-top: 16px; border-top: 1px solid #edf0f3; }
.result-counts div { display: flex; align-items: center; flex-direction: column; gap: 5px; border-right: 1px solid #edf0f3; }
.result-counts div:last-child { border-right: 0; }
.result-counts strong { font-size: 20px; }
.result-counts strong.danger { color: #d95858; }.result-counts strong.warning { color: #cf8b20; }
.result-counts span { color: #8a94a3; font-size: 11px; }
.content-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.content-grid > div { min-width: 0; }
.panel { margin-bottom: 16px; padding: 18px 20px; }
.panel-heading { min-height: 40px; justify-content: space-between; margin-bottom: 10px; }
.panel-heading h2 { margin: 0; font-size: 15px; letter-spacing: 0; }
.panel-heading p { margin: 4px 0 0; color: #8a94a3; font-size: 11px; }
.panel-heading > span { color: #8a94a3; font-size: 12px; }
.process-step { display: grid; grid-template-columns: 28px minmax(0, 1fr) auto; align-items: center; gap: 10px; min-height: 58px; border-bottom: 1px solid #edf0f3; }
.process-step:last-child { border-bottom: 0; }
.step-check { display: grid; width: 22px; height: 22px; place-items: center; border-radius: 50%; background: #eaf8f3; color: #2f9275; font-size: 12px; }
.process-step > div { display: flex; flex-direction: column; gap: 4px; }
.process-step strong { font-size: 13px; }.process-step span, .process-step time { color: #8a94a3; font-size: 11px; }
.file-row { display: grid; grid-template-columns: 34px minmax(0, 1fr) auto auto; align-items: center; gap: 10px; min-height: 58px; border-bottom: 1px solid #edf0f3; }
.file-row:last-child { border-bottom: 0; }
.file-icon { display: grid; width: 31px; height: 31px; place-items: center; border-radius: 4px; background: #edf4ff; color: #3478dc; }
.file-row > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.file-row strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.file-row span { color: #8a94a3; font-size: 11px; }
.hit-row { display: grid; grid-template-columns: 8px minmax(0, 1fr) auto; align-items: center; gap: 10px; min-height: 62px; border-bottom: 1px solid #edf0f3; }
.hit-row:last-child { border-bottom: 0; }.level-mark { width: 7px; height: 7px; border-radius: 50%; }.level-mark.critical { background: #d95858; }.level-mark.warning { background: #cf8b20; }.level-mark.info { background: #3478dc; }
.hit-row > div { display: flex; min-width: 0; flex-direction: column; gap: 5px; }.hit-row strong { font-size: 12px; }.hit-row code { overflow: hidden; color: #788394; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.hit-row b { color: #495568; font-size: 12px; }
.service-log { padding: 10px 12px; border-radius: 4px; background: #141b29; color: #dce5f2; font: 11px/1.7 Consolas, monospace; }
.service-log > div { display: grid; grid-template-columns: 58px 38px minmax(0, 1fr); gap: 7px; }.service-log time { color: #7f8ca0; }.service-log span { color: #68a0ea; }.service-log span.WARN { color: #e3a84c; }.service-log p { margin: 0; }
@media (max-width: 1000px) { .overview-grid, .content-grid { grid-template-columns: 1fr; } }
@media (max-width: 680px) { .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }.heading-actions { width: 100%; }.heading-actions .el-button { flex: 1; }.meta-grid { grid-template-columns: repeat(2, 1fr); } }
</style>

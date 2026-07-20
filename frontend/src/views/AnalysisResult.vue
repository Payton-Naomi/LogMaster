<template>
  <div class="result-page">
    <div class="page-heading">
      <div class="heading-left"><el-button text circle :icon="ArrowLeft" title="返回任务" @click="router.push(`/task/${taskId}`)" /><div><h1>解析结果</h1><p>DR2860_稳定性回归_0716 · {{ taskId }}</p></div></div>
      <div><el-button :icon="Download" @click="exportReport">导出报告</el-button><el-button type="primary" :icon="Document" @click="exportRaw">导出命中日志</el-button></div>
    </div>

    <section class="conclusion-band">
      <div class="conclusion-main"><span><el-icon><WarningFilled /></el-icon></span><div><small>测试结论</small><h2>发现严重异常，建议定位后重新测试</h2><p>共匹配 21 条规则，聚合出 20 个问题，其中 6 个严重问题。</p></div></div>
      <div class="conclusion-stats"><div><strong>324,580</strong><span>日志总行数</span></div><div><strong class="danger">6</strong><span>严重问题</span></div><div><strong class="warning">14</strong><span>警告问题</span></div><div><strong>21</strong><span>命中规则</span></div></div>
    </section>

    <div class="result-layout">
      <aside class="result-sidebar">
        <div class="side-title">问题分类</div>
        <button v-for="category in categories" :key="category.id" type="button" :class="{ active: selectedCategory === category.id }" @click="selectedCategory = category.id"><span>{{ category.name }}</span><b>{{ category.count }}</b></button>
        <div class="coverage"><div><span>规则覆盖率</span><strong>87.5%</strong></div><el-progress :percentage="88" :show-text="false" :stroke-width="7" /><p>已执行 21 / 24 条启用规则</p></div>
      </aside>

      <main class="issues-panel">
        <div class="issue-filters"><el-input v-model="search" :prefix-icon="Search" clearable placeholder="搜索问题或关键字" /><el-select v-model="level" clearable placeholder="全部级别"><el-option label="严重" value="critical" /><el-option label="警告" value="warning" /></el-select><span>共 {{ filteredIssues.length }} 类问题</span></div>
        <div class="issue-list">
          <article v-for="issue in filteredIssues" :key="issue.id" class="issue-card">
            <button type="button" class="issue-header" @click="toggleIssue(issue.id)">
              <span class="issue-icon" :class="issue.level"><el-icon><WarningFilled /></el-icon></span>
              <span class="issue-copy"><span><strong>{{ issue.title }}</strong><el-tag :type="issue.level === 'critical' ? 'danger' : 'warning'" size="small" effect="plain">{{ issue.level === 'critical' ? '严重' : '警告' }}</el-tag></span><small>{{ issue.description }}</small></span>
              <span class="issue-count"><b>{{ issue.count }}</b> 次</span>
              <el-icon class="expand-icon" :class="{ open: expanded.includes(issue.id) }"><ArrowDown /></el-icon>
            </button>
            <div v-if="expanded.includes(issue.id)" class="issue-detail">
              <div class="keyword-line"><span>命中关键字</span><code>{{ issue.keyword }}</code><span>适用范围</span><b>{{ issue.scope }}</b></div>
              <div class="evidence-title"><span>证据日志</span><el-button type="primary" link @click="copyEvidence(issue)">复制</el-button></div>
              <div class="evidence-log"><div v-for="(log, index) in issue.logs" :key="index"><span>{{ log.time }}</span><em>{{ log.level }}</em><code>{{ log.content }}</code></div></div>
              <div class="suggestion"><strong>处理建议</strong><span>{{ issue.suggestion }}</span></div>
            </div>
          </article>
          <el-empty v-if="!filteredIssues.length" description="没有匹配的问题" />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowDown, ArrowLeft, Document, Download, Search, WarningFilled } from '@element-plus/icons-vue'

const route = useRoute(); const router = useRouter(); const taskId = route.params.taskId
const search = ref(''); const level = ref(''); const selectedCategory = ref('all'); const expanded = ref([1])
const categories = [{ id: 'all', name: '全部问题', count: 20 }, { id: 'power', name: '开关机与电源', count: 8 }, { id: 'storage', name: 'SD 卡与存储', count: 5 }, { id: 'recording', name: '录像与视频', count: 4 }, { id: 'system', name: '系统稳定性', count: 3 }]
const issues = [
  { id: 1, category: 'power', title: '检测到看门狗异常重启', description: '测试过程中出现软件看门狗唤醒源，属于非预期开机', keyword: 'POWER_ID_SWRT | 2f0050080 :', scope: 'DR4800/5800', level: 'critical', count: 3, suggestion: '结合重启前 30 秒的 backtrace 和 CPU 温度日志定位卡死模块。', logs: [{ time: '14:18:31.208', level: 'ERROR', content: 'power source: POWER_ID_SWRT, 2f0050080 : 00000001 00000000' }, { time: '14:18:31.214', level: 'INFO', content: 'system boot starting, wakeup source = watchdog' }] },
  { id: 2, category: 'system', title: '系统崩溃调用栈', description: '发现 backtrace 崩溃信息，关联录像服务异常退出', keyword: 'backtrace', scope: '自研通用', level: 'critical', count: 2, suggestion: '保留完整调用栈并交由录像模块负责人分析空指针位置。', logs: [{ time: '14:18:29.771', level: 'FATAL', content: 'backtrace: #00 pc 00034f20 recorder_service' }] },
  { id: 3, category: 'power', title: 'UBIFS 文件系统异常', description: '通断电测试期间出现 UBI/UBIFS 错误', keyword: 'ubi0 e|ubi1 e|ubi2 e|uib3 e|ubifs e', scope: '自研电容项目', level: 'critical', count: 1, suggestion: '检查断电时序和数据同步流程，确认关机前 sync 已完成。', logs: [{ time: '14:18:28.102', level: 'ERROR', content: 'ubifs error: failed to write node, error -5' }] },
  { id: 4, category: 'storage', title: '存储卡文件系统异常', description: 'FAT-fs 相关错误持续打印，非带电拔卡场景', keyword: 'FAT-fs', scope: '自研通用', level: 'critical', count: 4, suggestion: '使用工具检测 SD 卡文件系统，确认是否可稳定复现。', logs: [{ time: '14:06:12.402', level: 'ERROR', content: 'FAT-fs (mmcblk0p1): error, invalid access to FAT' }] },
  { id: 5, category: 'recording', title: '视频编码队列丢帧', description: '编码队列满导致录像帧被丢弃', keyword: 'queue is full!!! drop frame', scope: '自研通用', level: 'critical', count: 17, suggestion: '检查编码器处理能力、码率配置与 SD 卡写入性能。', logs: [{ time: '14:22:44.912', level: 'ERROR', content: 'venc queue is full!!! drop frame channel=0 seq=98231' }] },
  { id: 6, category: 'recording', title: '累计丢帧时间超限', description: '30 秒内累计丢帧时间超过 15000ms', keyword: 'SD write detected frame loss for', scope: '自研通用', level: 'critical', count: 2, suggestion: '更换高性能 SD 卡复测，并采集存储写入延迟。', logs: [{ time: '14:22:45.103', level: 'ERROR', content: 'SD write detected frame loss for 15842ms' }] },
  { id: 7, category: 'storage', title: '存储卡性能不足', description: '存储速度监控返回性能不足状态', keyword: 'speed monitor state cb, state', scope: '自研通用', level: 'warning', count: 9, suggestion: '记录 SD 卡品牌、容量和速度等级，使用推荐卡复测。', logs: [{ time: '14:21:03.018', level: 'WARN', content: 'speed monitor state cb, state = low_speed' }] },
  { id: 8, category: 'power', title: '关机原因异常', description: '循环开关机中出现低电关机，与测试预期不符', keyword: 'SYSTEMMNG_SHUTDOWN_VOL_LOW_E', scope: '自研通用', level: 'warning', count: 2, suggestion: '核对供电电压和电源管理采样值。', logs: [{ time: '13:58:20.552', level: 'WARN', content: 'shutdown type: SYSTEMMNG_SHUTDOWN_VOL_LOW_E' }] }
]
const filteredIssues = computed(() => issues.filter((item) => (selectedCategory.value === 'all' || item.category === selectedCategory.value) && (!level.value || item.level === level.value) && (!search.value || `${item.title}${item.keyword}${item.description}`.toLowerCase().includes(search.value.toLowerCase()))))
const toggleIssue = (id) => { expanded.value = expanded.value.includes(id) ? expanded.value.filter((item) => item !== id) : [...expanded.value, id] }
const copyEvidence = async (issue) => { await navigator.clipboard?.writeText(issue.logs.map((item) => `${item.time} ${item.level} ${item.content}`).join('\n')); ElMessage.success('证据日志已复制') }
const exportReport = () => ElMessage.success('分析报告已加入下载队列'); const exportRaw = () => ElMessage.success('命中日志已加入下载队列')
</script>

<style scoped>
.result-page { height: 100%; overflow: auto; color: #1f2937; box-sizing: border-box; }
.page-heading, .heading-left, .conclusion-band, .conclusion-main, .conclusion-stats, .issue-filters, .keyword-line, .evidence-title { display: flex; align-items: center; }
.page-heading { justify-content: space-between; margin-bottom: 18px; }.heading-left { gap: 8px; }.heading-left h1 { margin: 0; font-size: 21px; letter-spacing: 0; }.heading-left p { margin: 4px 0 0; color: #8a94a3; font-size: 12px; }.page-heading > div:last-child { display: flex; gap: 8px; }
.conclusion-band { justify-content: space-between; margin-bottom: 16px; padding: 18px 22px; border: 1px solid #e8c8c8; border-radius: 6px; background: #fffafa; }.conclusion-main { gap: 13px; }.conclusion-main > span { display: grid; width: 46px; height: 46px; place-items: center; border-radius: 5px; background: #ffe8e8; color: #d95858; font-size: 24px; }.conclusion-main small { color: #9b6d6d; }.conclusion-main h2 { margin: 3px 0; color: #a53f3f; font-size: 17px; letter-spacing: 0; }.conclusion-main p { margin: 0; color: #8b6d6d; font-size: 12px; }.conclusion-stats { min-width: 420px; }.conclusion-stats > div { display: flex; min-width: 95px; align-items: center; flex-direction: column; gap: 5px; border-right: 1px solid #efdada; }.conclusion-stats > div:last-child { border: 0; }.conclusion-stats strong { font-size: 20px; }.conclusion-stats strong.danger { color: #d95858; }.conclusion-stats strong.warning { color: #cf8b20; }.conclusion-stats span { color: #8b7373; font-size: 11px; }
.result-layout { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 16px; }.result-sidebar, .issues-panel { border: 1px solid #dfe3e8; border-radius: 6px; background: #fff; }.result-sidebar { height: max-content; padding: 14px 10px; }.side-title { padding: 2px 10px 12px; border-bottom: 1px solid #edf0f3; color: #596273; font-size: 13px; font-weight: 600; }.result-sidebar > button { display: flex; width: 100%; align-items: center; justify-content: space-between; margin-top: 5px; padding: 10px; border: 0; border-radius: 4px; background: transparent; color: #596273; cursor: pointer; }.result-sidebar > button:hover { background: #f7f9fc; }.result-sidebar > button.active { background: #edf4ff; color: #2868ca; font-weight: 600; }.result-sidebar button span { font-size: 12px; }.result-sidebar button b { font-size: 11px; }.coverage { margin-top: 16px; padding: 15px 10px 4px; border-top: 1px solid #edf0f3; }.coverage > div { display: flex; justify-content: space-between; margin-bottom: 9px; font-size: 12px; }.coverage p { margin: 7px 0 0; color: #8a94a3; font-size: 10px; }
.issues-panel { min-width: 0; padding: 16px; }.issue-filters { gap: 9px; margin-bottom: 14px; }.issue-filters .el-input { width: 320px; }.issue-filters .el-select { width: 120px; }.issue-filters > span { margin-left: auto; color: #8a94a3; font-size: 11px; }.issue-card { margin-bottom: 9px; border: 1px solid #e1e5ea; border-radius: 5px; overflow: hidden; }.issue-header { display: grid; width: 100%; grid-template-columns: 36px minmax(0, 1fr) 58px 20px; align-items: center; gap: 10px; padding: 13px 15px; border: 0; background: #fff; color: inherit; text-align: left; cursor: pointer; }.issue-header:hover { background: #fafbfd; }.issue-icon { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 4px; }.issue-icon.critical { color: #d95858; background: #fff0f0; }.issue-icon.warning { color: #cf8b20; background: #fff6e7; }.issue-copy { display: flex; min-width: 0; flex-direction: column; gap: 5px; }.issue-copy > span { display: flex; align-items: center; gap: 7px; }.issue-copy strong { font-size: 13px; }.issue-copy small { overflow: hidden; color: #7f8997; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.issue-count { color: #7f8997; font-size: 11px; }.issue-count b { color: #3f4b5e; font-size: 14px; }.expand-icon { color: #8a94a3; transition: transform .2s; }.expand-icon.open { transform: rotate(180deg); }
.issue-detail { padding: 14px 16px 16px 61px; border-top: 1px solid #edf0f3; background: #fafbfd; }.keyword-line { gap: 9px; color: #7d8795; font-size: 11px; }.keyword-line code { overflow: hidden; max-width: 45%; padding: 4px 7px; border-radius: 3px; background: #eef1f5; color: #39475c; text-overflow: ellipsis; white-space: nowrap; }.keyword-line b { color: #596273; }.evidence-title { justify-content: space-between; margin-top: 14px; color: #596273; font-size: 12px; font-weight: 600; }.evidence-log { padding: 9px 12px; border-radius: 4px; background: #141b29; color: #dbe5f2; font: 11px/1.8 Consolas, monospace; }.evidence-log > div { display: grid; grid-template-columns: 88px 42px minmax(0, 1fr); gap: 7px; }.evidence-log span { color: #8390a3; }.evidence-log em { color: #ed7272; font-style: normal; }.evidence-log code { overflow-wrap: anywhere; }.suggestion { display: flex; gap: 10px; margin-top: 12px; padding: 9px 11px; border-left: 3px solid #4b82d4; background: #edf4ff; font-size: 11px; }.suggestion strong { flex: 0 0 auto; color: #316bc4; }.suggestion span { color: #596b83; }
@media (max-width: 1050px) { .conclusion-band { align-items: flex-start; flex-direction: column; gap: 18px; }.conclusion-stats { width: 100%; min-width: 0; }.result-layout { grid-template-columns: 1fr; }.result-sidebar { display: flex; overflow-x: auto; }.side-title, .coverage { display: none; }.result-sidebar > button { min-width: max-content; } }
@media (max-width: 680px) { .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }.conclusion-stats { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }.conclusion-stats > div { border: 0; }.issue-filters { flex-wrap: wrap; }.issue-filters .el-input, .issue-filters .el-select { width: 100%; }.issue-detail { padding-left: 14px; }.keyword-line { align-items: flex-start; flex-direction: column; }.keyword-line code { max-width: 100%; }.evidence-log > div { grid-template-columns: 1fr; margin-bottom: 7px; } }
</style>

<template>
  <div class="dashboard-page">
    <div class="page-heading">
      <div>
        <h1>日志数据概览</h1>
        <p>汇总日志采集、解析和异常分布情况</p>
      </div>
      <el-segmented v-model="range" :options="rangeOptions" @change="refreshCharts" />
    </div>

    <div class="stat-grid">
      <div v-for="item in stats" :key="item.label" class="stat-card">
        <div class="stat-topline">
          <span>{{ item.label }}</span>
          <el-icon :class="item.tone"><component :is="item.icon" /></el-icon>
        </div>
        <strong>{{ item.value }}</strong>
        <div class="stat-trend" :class="item.trendTone">
          <el-icon><Top v-if="item.direction === 'up'" /><Bottom v-else /></el-icon>
          <span>{{ item.trend }}</span>
          <em>较上一周期</em>
        </div>
      </div>
    </div>

    <section class="chart-panel trend-panel">
      <div class="panel-heading">
        <div>
          <h2>日志与异常趋势</h2>
          <p>每日解析日志量及异常日志变化</p>
        </div>
        <div class="legend-note">
          <span><i class="log-dot" />日志总量</span>
          <span><i class="error-dot" />异常日志</span>
        </div>
      </div>
      <div ref="trendChartRef" class="trend-chart" />
    </section>

    <div class="chart-grid">
      <section class="chart-panel">
        <div class="panel-heading">
          <div>
            <h2>日志级别分布</h2>
            <p>当前周期内各级别日志占比</p>
          </div>
          <span class="panel-total">共 184.3 万行</span>
        </div>
        <div ref="levelChartRef" class="small-chart" />
      </section>

      <section class="chart-panel">
        <div class="panel-heading">
          <div>
            <h2>关键异常排行</h2>
            <p>根据记录仪日志关键字段聚合</p>
          </div>
        </div>
        <div ref="moduleChartRef" class="small-chart" />
      </section>
    </div>

    <section class="chart-panel recent-panel">
      <div class="panel-heading">
        <div>
          <h2>最近解析任务</h2>
          <p>最新完成的日志分析记录</p>
        </div>
        <el-button type="primary" link @click="router.push('/tasks')">查看全部</el-button>
      </div>
      <el-table :data="recentTasks" class="task-table">
        <el-table-column prop="name" label="任务名称" min-width="180" />
        <el-table-column prop="scene" label="测试场景" min-width="130" />
        <el-table-column prop="logs" label="日志行数" min-width="110" />
        <el-table-column prop="issues" label="发现问题" min-width="110">
          <template #default="scope">
            <span :class="{ 'issue-count': scope.row.issues > 0 }">{{ scope.row.issues }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="time" label="完成时间" min-width="160" />
        <el-table-column label="状态" width="100">
          <template #default><el-tag type="success" effect="plain">已完成</el-tag></template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>

<script setup>
import { markRaw, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { Bottom, CircleCheck, DataLine, Files, Top, Warning } from '@element-plus/icons-vue'

const router = useRouter()
const range = ref('近 7 天')
const rangeOptions = ['近 7 天', '近 30 天']
const trendChartRef = ref(null)
const levelChartRef = ref(null)
const moduleChartRef = ref(null)
const chartInstances = []
let resizeObserver = null

const stats = [
  { label: '解析日志总量', value: '1,843,260', trend: '12.6%', direction: 'up', trendTone: 'positive', tone: 'blue', icon: markRaw(Files) },
  { label: '异常日志', value: '1,286', trend: '8.2%', direction: 'down', trendTone: 'positive', tone: 'red', icon: markRaw(Warning) },
  { label: '解析任务', value: '48', trend: '6.4%', direction: 'up', trendTone: 'positive', tone: 'green', icon: markRaw(DataLine) },
  { label: '解析成功率', value: '98.7%', trend: '0.5%', direction: 'up', trendTone: 'positive', tone: 'gold', icon: markRaw(CircleCheck) }
]

const recentTasks = [
  { name: 'DR2860_稳定性回归_0716', scene: '开关机测试', logs: '324,580', issues: 6, time: '2026-07-16 14:32' },
  { name: 'SD64G_24H挂测', scene: 'SD 卡挂测', logs: '518,204', issues: 12, time: '2026-07-16 11:08' },
  { name: '基础功能冒烟测试', scene: '通用解析', logs: '86,912', issues: 0, time: '2026-07-15 18:46' },
  { name: 'DR2860_循环开关机', scene: '开关机测试', logs: '207,345', issues: 3, time: '2026-07-15 16:21' }
]

const trendData = {
  '近 7 天': {
    labels: ['07-10', '07-11', '07-12', '07-13', '07-14', '07-15', '07-16'],
    logs: [182, 246, 218, 305, 274, 289, 329],
    errors: [96, 164, 121, 238, 192, 221, 254]
  },
  '近 30 天': {
    labels: ['06-17', '06-21', '06-25', '06-29', '07-03', '07-07', '07-11', '07-16'],
    logs: [604, 718, 692, 844, 791, 936, 1024, 1187],
    errors: [318, 402, 371, 566, 489, 624, 702, 786]
  }
}

const baseTooltip = {
  trigger: 'axis',
  backgroundColor: '#fff',
  borderColor: '#dfe3e8',
  borderWidth: 1,
  textStyle: { color: '#303846', fontSize: 12 }
}

const initCharts = () => {
  const trendChart = echarts.init(trendChartRef.value)
  const levelChart = echarts.init(levelChartRef.value)
  const moduleChart = echarts.init(moduleChartRef.value)
  chartInstances.push(trendChart, levelChart, moduleChart)

  const current = trendData[range.value]
  trendChart.setOption({
    animationDuration: 500,
    tooltip: baseTooltip,
    grid: { left: 16, right: 18, top: 24, bottom: 8, containLabel: true },
    xAxis: { type: 'category', boundaryGap: false, data: current.labels, axisLine: { lineStyle: { color: '#dfe3e8' } }, axisTick: { show: false }, axisLabel: { color: '#7a8493' } },
    yAxis: [
      { type: 'value', name: '日志量（千行）', nameTextStyle: { color: '#8993a1', padding: [0, 0, 8, 0] }, splitLine: { lineStyle: { color: '#edf0f3' } }, axisLabel: { color: '#7a8493' } },
      { type: 'value', name: '异常数', nameTextStyle: { color: '#8993a1', padding: [0, 0, 8, 0] }, splitLine: { show: false }, axisLabel: { color: '#7a8493' } }
    ],
    series: [
      { name: '日志总量', type: 'line', smooth: true, symbol: 'circle', symbolSize: 7, data: current.logs, lineStyle: { width: 3, color: '#3478dc' }, itemStyle: { color: '#3478dc' }, areaStyle: { color: 'rgba(52, 120, 220, 0.08)' } },
      { name: '异常日志', type: 'line', yAxisIndex: 1, smooth: true, symbol: 'circle', symbolSize: 7, data: current.errors, lineStyle: { width: 2, color: '#d95858' }, itemStyle: { color: '#d95858' } }
    ]
  })

  levelChart.setOption({
    animationDuration: 500,
    tooltip: { trigger: 'item', formatter: '{b}<br/>{c} 行（{d}%）' },
    legend: { orient: 'vertical', right: 12, top: 'center', itemWidth: 10, itemHeight: 10, itemGap: 16, textStyle: { color: '#596273', fontSize: 12 } },
    series: [{
      type: 'pie',
      radius: ['50%', '72%'],
      center: ['35%', '52%'],
      avoidLabelOverlap: true,
      label: { show: false },
      emphasis: { scaleSize: 5 },
      data: [
        { value: 1124000, name: 'INFO', itemStyle: { color: '#3979d9' } },
        { value: 523000, name: 'DEBUG', itemStyle: { color: '#43a286' } },
        { value: 181000, name: 'WARN', itemStyle: { color: '#d99a32' } },
        { value: 15260, name: 'ERROR', itemStyle: { color: '#d95858' } }
      ]
    }],
    graphic: [
      { type: 'text', left: '30%', top: '44%', style: { text: '1.84M', fill: '#263244', font: '600 22px sans-serif', textAlign: 'center' } },
      { type: 'text', left: '31%', top: '56%', style: { text: '总行数', fill: '#8a94a3', font: '12px sans-serif', textAlign: 'center' } }
    ]
  })

  moduleChart.setOption({
    animationDuration: 500,
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 12, right: 26, top: 10, bottom: 8, containLabel: true },
    xAxis: { type: 'value', splitLine: { lineStyle: { color: '#edf0f3' } }, axisLabel: { color: '#8a94a3' } },
    yAxis: { type: 'category', data: ['应用崩溃', '文件系统异常', '异常重启', 'SD 卡异常', '视频丢帧'], axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: '#596273' } },
    series: [{ type: 'bar', data: [32, 47, 68, 96, 128], barWidth: 14, itemStyle: { color: '#d95858', borderRadius: [0, 3, 3, 0] }, label: { show: true, position: 'right', color: '#596273' } }]
  })
}

const refreshCharts = () => {
  if (!chartInstances.length) return
  const current = trendData[range.value]
  chartInstances[0].setOption({ xAxis: { data: current.labels }, series: [{ data: current.logs }, { data: current.errors }] })
}

onMounted(async () => {
  await nextTick()
  initCharts()
  resizeObserver = new ResizeObserver(() => chartInstances.forEach((chart) => chart.resize()))
  resizeObserver.observe(trendChartRef.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chartInstances.forEach((chart) => chart.dispose())
})
</script>

<style scoped>
.dashboard-page {
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

.page-heading h1 {
  margin: 0;
  font-size: 22px;
  line-height: 1.4;
  letter-spacing: 0;
}

.page-heading p,
.panel-heading p {
  margin: 5px 0 0;
  color: #7a8493;
  font-size: 13px;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.stat-card,
.chart-panel {
  background: #fff;
  border: 1px solid #dfe3e8;
  border-radius: 6px;
}

.stat-card {
  min-width: 0;
  padding: 18px 20px;
}

.stat-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #667085;
  font-size: 13px;
}

.stat-topline .el-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 4px;
  font-size: 18px;
}

.stat-topline .blue { color: #3478dc; background: #edf4ff; }
.stat-topline .red { color: #d95858; background: #fff0f0; }
.stat-topline .green { color: #2f9275; background: #eaf8f3; }
.stat-topline .gold { color: #c9861b; background: #fff6e7; }

.stat-card > strong {
  display: block;
  margin: 10px 0 8px;
  color: #202b3c;
  font-size: 27px;
  line-height: 1.2;
}

.stat-trend {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #2f9275;
  font-size: 12px;
}

.stat-trend .el-icon { font-size: 12px; }
.stat-trend em { margin-left: 3px; color: #98a1ad; font-style: normal; }

.chart-panel {
  min-width: 0;
  padding: 18px 20px;
  box-sizing: border-box;
}

.trend-panel { margin-bottom: 16px; }

.panel-heading {
  display: flex;
  min-height: 42px;
  align-items: flex-start;
  justify-content: space-between;
}

.panel-heading h2 {
  margin: 0;
  color: #293548;
  font-size: 16px;
  letter-spacing: 0;
}

.legend-note {
  display: flex;
  gap: 18px;
  color: #667085;
  font-size: 12px;
}

.legend-note span { display: flex; align-items: center; gap: 6px; }
.legend-note i { width: 9px; height: 9px; border-radius: 50%; }
.log-dot { background: #3478dc; }
.error-dot { background: #d95858; }

.trend-chart { height: 285px; }

.chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.small-chart { height: 270px; }
.panel-total { color: #8a94a3; font-size: 12px; }

.recent-panel { margin-bottom: 20px; }
.task-table { margin-top: 10px; }
.recent-panel :deep(.el-table__header th) { background: #f8fafc; color: #667085; font-weight: 600; }
.issue-count { color: #d95858; font-weight: 600; }

@media (max-width: 1100px) {
  .stat-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 760px) {
  .page-heading { align-items: flex-start; flex-direction: column; gap: 12px; }
  .chart-grid { grid-template-columns: 1fr; }
}

@media (max-width: 520px) {
  .stat-grid { grid-template-columns: 1fr; }
  .legend-note { display: none; }
}
</style>

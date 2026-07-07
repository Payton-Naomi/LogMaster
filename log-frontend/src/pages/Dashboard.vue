<template>
    <div class="dashboard">
        <h2>数据概览</h2>
        <!-- 统计卡片 -->
        <div class="stat-cards">
            <el-card class="stat-card">
                <div class="stat-title">今日采集日志量</div>
                <div class="stat-value">12890 条</div>
            </el-card>
            <el-card class="stat-card">
                <div class="stat-title">异常日志数</div>
                <div class="stat-value" style="color: #f56c6c;">127 条</div>
            </el-card>
            <el-card class="stat-card">
                <div class="stat-title">在线设备数</div>
                <div class="stat-value" style="color: #67c23a;">8 台</div>
            </el-card>
            <el-card class="stat-card">
                <div class="stat-title">今日告警数</div>
                <div class="stat-value" style="color: #e6a23c;">12 条</div>
            </el-card>
        </div>

        <!-- ECharts 趋势图 -->
        <el-card class="chart-card" style="margin-top: 20px;">
            <div ref="chartRef" style="height: 400px;"></div>
        </el-card>
    </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts'

const chartRef = ref(null)
let chartInstance = null

// 初始化图表
const initChart = () => {
    chartInstance = echarts.init(chartRef.value)
    const option = {
        title: { text: '近7天异常日志趋势' },
        tooltip: { trigger: 'axis' },
        legend: { data: ['Error', 'Warn', 'Fatal'] },
        xAxis: {
            type: 'category',
            data: ['7-01', '7-02', '7-03', '7-04', '7-05', '7-06', '7-07']
        },
        yAxis: { type: 'value' },
        series: [
            { name: 'Error', type: 'line', data: [120, 132, 101, 134, 90, 230, 210], smooth: true },
            { name: 'Warn', type: 'line', data: [220, 182, 191, 234, 290, 330, 310], smooth: true },
            { name: 'Fatal', type: 'line', data: [15, 23, 20, 15, 19, 33, 41], smooth: true }
        ]
    }
    chartInstance.setOption(option)
}

// 窗口 resize 时重绘图表
const handleResize = () => chartInstance?.resize()

onMounted(() => {
    initChart()
    window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
    chartInstance?.dispose()
})
</script>

<style scoped>
.dashboard {
    padding: 20px;
}

.stat-cards {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 20px;
}

.stat-card {
    text-align: center;
}

.stat-title {
    color: #666;
    font-size: 14px;
}

.stat-value {
    font-size: 28px;
    font-weight: bold;
    margin-top: 10px;
}

.chart-card {
    padding: 20px;
}
</style>
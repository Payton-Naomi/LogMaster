<template>
    <div class="device-page">
        <h2>设备列表</h2>
        <el-table :data="deviceList" border style="margin-top: 20px">
            <el-table-column prop="sn" label="设备SN" width="180" />
            <el-table-column prop="name" label="设备名称" width="180" />
            <el-table-column prop="firmware" label="固件版本" width="120" />
            <el-table-column prop="status" label="状态" width="100">
                <template #default="{ row }">
                    <el-tag :type="row.status === '在线' ? 'success' : 'danger'">
                        {{ row.status }}
                    </el-tag>
                </template>
            </el-table-column>
            <el-table-column prop="lastSeen" label="最后在线时间" />
        </el-table>
    </div>
</template>

<script setup>
// 第1周先放假数据，第1周周五换成后端接口请求
const deviceList = [
    {
        sn: "SN20260701001",
        name: "行车记录仪A",
        firmware: "V1.2.3",
        status: "在线",
        lastSeen: "2026-07-07 14:30:00",
    },
    {
        sn: "SN20260701002",
        name: "行车记录仪B",
        firmware: "V1.2.2",
        status: "离线",
        lastSeen: "2026-07-06 09:15:00",
    },
    {
        sn: "SN20260701003",
        name: "行车记录仪C",
        firmware: "V1.2.3",
        status: "在线",
        lastSeen: "2026-07-07 15:00:00",
    },
];
</script>

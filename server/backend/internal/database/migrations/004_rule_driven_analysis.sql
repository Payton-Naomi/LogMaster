ALTER TABLE logmaster_api.parse_rules
    ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'manual';

ALTER TABLE logmaster_api.parse_results
    ADD COLUMN IF NOT EXISTS rule_id BIGINT REFERENCES logmaster_api.parse_rules(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rule_name VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS category VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS event_time TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS context_start_time TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS context_end_time TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS context_lines JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS related_causes JSONB NOT NULL DEFAULT '[]'::JSONB;

CREATE INDEX IF NOT EXISTS idx_parse_rules_enabled_priority
    ON logmaster_api.parse_rules (enabled, priority, id);
CREATE INDEX IF NOT EXISTS idx_parse_results_task_event_time
    ON logmaster_api.parse_results (task_id, event_time, id);

INSERT INTO logmaster_api.parse_rules
    (name, category, keyword, scope, level, enabled, description, priority, source)
SELECT seed.name, seed.category, seed.keyword, seed.scope, seed.level, seed.enabled,
       seed.description, seed.priority, seed.source
FROM (VALUES
    ('设备启动状态', 'power', 'starting|need_power_off', '老项目', 'info', FALSE, '开关机状态记录', 300, 'keyword_document'),
    ('设备关机状态', 'power', 'shutdown:', '新项目（如 DR2820）', 'info', FALSE, '关机状态和关机原因', 300, 'keyword_document'),
    ('ACC 状态', 'power', 'AccOff|AccON|trans to|acc', '自研通用', 'info', FALSE, 'ACC 状态切换', 300, 'keyword_document'),
    ('进入缩时模式', 'power', 'ready to enter lapse mode', '自研通用', 'info', FALSE, '进入缩时录像模式', 300, 'keyword_document'),
    ('SD 卡状态', 'storage', 'sd state|sd_state', '自研通用', 'info', FALSE, 'SD 卡状态变化', 300, 'keyword_document'),
    ('系统崩溃', 'system', 'backtrace', '自研通用', 'critical', TRUE, '系统崩溃堆栈', 10, 'keyword_document'),
    ('ADAS 日志', 'feature', 'zdd Adas', '自研通用', 'info', FALSE, 'ADAS 功能日志', 300, 'keyword_document'),
    ('声音播放', 'feature', 'voice play', '自研通用', 'info', FALSE, '语音播放状态', 300, 'keyword_document'),
    ('Wi-Fi 信息', 'connectivity', 'Wifi--------', '自研通用', 'info', FALSE, 'Wi-Fi 名称和连接信息', 300, 'keyword_document'),
    ('视频录制状态', 'recording', 'File Start|File End', '自研通用', 'info', FALSE, '视频文件开始和结束', 300, 'keyword_document'),
    ('后录状态', 'recording', 'AHD', '自研通用', 'info', FALSE, '后摄相关日志', 300, 'keyword_document'),
    ('蓝牙状态', 'connectivity', 'blemng', '自研通用', 'info', FALSE, '蓝牙管理日志', 300, 'keyword_document'),
    ('OTA 状态', 'feature', 'OTA start', '自研通用', 'info', FALSE, 'OTA 开始日志', 300, 'keyword_document'),
    ('GPS 状态', 'feature', 'RMC:', '自研通用', 'info', FALSE, 'GPS RMC 数据', 300, 'keyword_document'),
    ('CPU 温度', 'system', 'cpu_temp', '自研通用', 'info', FALSE, 'CPU 温度日志', 300, 'keyword_document'),
    ('系统模式', 'system', 'FL_BOOT_SYSMODE|XA_NORMAL_SYSMODE|XA_GUIDER_SYSMODE|XA_FAC_SYSMODE|XA_FAC_SMT_SYSMODE', '自研通用', 'info', FALSE, '系统启动模式枚举', 300, 'keyword_document'),
    ('开机唤醒来源', 'power', 'POWER_ID_PSW1|POWER_ID_PSW2|POWER_ID_PSW3|POWER_ID_PSW4|POWER_ID_HWRT|POWER_ID_SWRT|POWER_ID_4G', '自研通用', 'info', FALSE, '设备开机唤醒来源', 300, 'keyword_document'),
    ('关机原因', 'power', 'SYSTEMMNG_SHUTDOWN_', '自研通用', 'info', FALSE, '设备关机原因枚举', 300, 'keyword_document'),
    ('SD 卡枚举状态', 'storage', 'STGMNG_SD_', '自研通用', 'info', FALSE, 'SD 卡管理状态枚举', 300, 'keyword_document'),
    ('联咏出流信息命令', 'tool', 'cat /proc/hdal/comm/info|cat /proc/hdal/venc/info|cat /proc/hdal/vprc/info|cat /proc/hdal/vcap/info', '自研通用', 'info', FALSE, '辅助排查命令，不默认参与解析', 500, 'keyword_document'),
    ('看门狗重启', 'power', '2f0050080|POWER_ID_SWRT', 'DR4800/5800', 'critical', TRUE, '看门狗或软件复位', 10, 'keyword_document'),
    ('视频丢帧', 'recording', 'queue is full!!! drop frame', '自研通用', 'warning', TRUE, '编码队列满导致视频丢帧', 20, 'keyword_document'),
    ('应用程序崩溃', 'system', 'Log_Signal_Data', '自研通用', 'critical', TRUE, '应用程序信号崩溃', 10, 'keyword_document'),
    ('疲劳驾驶提醒', 'feature', 'adas_driver_take_a_rest.aac|开了很久了，休息一下再赶路吧', '自研通用', 'info', FALSE, '疲劳驾驶语音提醒', 300, 'keyword_document'),
    ('蓝牙查询命令', 'tool', 'hciconfig -a', '自研通用', 'info', FALSE, '辅助查询命令，不默认参与解析', 500, 'keyword_document'),
    ('UBI 文件系统异常', 'storage', 'ubi0 e|ubi1 e|ubi2 e|ubi3 e|ubifs e', '自研电容项目', 'warning', TRUE, '通断电过程中 UBI 异常', 30, 'keyword_document'),
    ('串口导出日志命令', 'tool', 'cp -r /mnt/other/log/ /mnt/sd|sync', '自研通用', 'info', FALSE, '辅助导出命令，不默认参与解析', 500, 'keyword_document'),
    ('提示格式化存储卡', 'storage', 's_FacParam.sd_state = 2|s_FacParam.sd_state   = 2', '自研通用', 'warning', TRUE, 'SD 卡需要格式化', 20, 'keyword_document'),
    ('卡内存在非记录仪文件', 'storage', 's_FacParam.sd_state = 11|s_FacParam.sd_state   = 11', '自研通用', 'warning', TRUE, 'SD 卡存在非记录仪文件', 20, 'keyword_document'),
    ('SD 卡性能不足', 'storage', 'speed monitor state cb, state', '自研通用', 'warning', TRUE, 'SD 卡性能不足建议更换', 20, 'keyword_document'),
    ('MP4 写入失败', 'recording', 'XA_MP4_Write failed', '自研通用', 'critical', TRUE, '视频文件写入失败', 5, 'keyword_document'),
    ('紧急视频目录创建失败', 'recording', 'Failed to create falloc directory: /mnt/sd/.tmp', '自研通用', 'critical', TRUE, '紧急视频录制目录创建失败', 5, 'keyword_document'),
    ('FAT 文件系统日志', 'storage', 'FAT-fs', '自研通用', 'warning', FALSE, '普通 FAT-fs 打印可能为正常过程，默认关闭以避免误报', 400, 'keyword_document'),
    ('30 秒窗口累计丢帧', 'recording', 'SD write detected frame loss for', '自研通用', 'critical', TRUE, '累计丢帧达到 15000ms 时判为异常', 5, 'keyword_document'),
    ('块设备 I/O 异常', 'storage', 'blk_update_request: I/O error|Buffer I/O error|Input/output error', '自研通用', 'critical', TRUE, 'SD 卡块设备读写失败', 5, 'derived'),
    ('FAT 读取异常', 'storage', 'FAT read failed|unable to read inode|Dirty bit is set', '自研通用', 'critical', TRUE, '文件系统读取失败或未正常卸载', 5, 'derived'),
    ('存储空间分配失败', 'storage', 'FALLOCATE_CON_CLUSTER failed|POOL_ALLOC failed', '自研通用', 'critical', TRUE, '录像文件空间分配失败', 8, 'derived'),
    ('通用错误', 'system', 'FATAL|ERROR', '自研通用', 'critical', TRUE, '未被专用规则覆盖的错误日志', 900, 'system'),
    ('通用警告', 'system', 'WARNING|WARN', '自研通用', 'warning', FALSE, '未被专用规则覆盖的警告日志，默认关闭以避免噪声', 950, 'system')
) AS seed(name, category, keyword, scope, level, enabled, description, priority, source)
WHERE NOT EXISTS (
    SELECT 1 FROM logmaster_api.parse_rules existing
    WHERE existing.name = seed.name AND existing.keyword = seed.keyword
);

INSERT INTO logmaster_api.test_scenarios (id, name, description, color, judgement, checks)
VALUES
    ('power-cycle', '开关机测试', '检查正常开关机、看门狗和异常重启', 'blue', 'any-error',
     '[{"id":"watchdog","name":"看门狗重启","severity":"critical","enabled":true,"keywords":["2f0050080","POWER_ID_SWRT"]},{"id":"crash","name":"系统或应用崩溃","severity":"critical","enabled":true,"keywords":["backtrace","Log_Signal_Data"]}]'::jsonb),
    ('sd-aging', 'SD 卡挂测', '检查丢帧、写盘失败和文件系统异常', 'orange', 'any-error',
     '[{"id":"write-failed","name":"MP4 写入失败","severity":"critical","enabled":true,"keywords":["XA_MP4_Write failed"]},{"id":"io-error","name":"块设备 I/O 异常","severity":"critical","enabled":true,"keywords":["I/O error","FAT read failed"]},{"id":"frame-loss","name":"视频丢帧","severity":"warning","enabled":true,"keywords":["queue is full!!! drop frame","SD write detected frame loss for"]}]'::jsonb)
ON CONFLICT (id) DO NOTHING;

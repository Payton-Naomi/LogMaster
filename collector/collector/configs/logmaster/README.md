# LogMaster 内置配置维护说明

普通用户不能导入或修改这些配置。四个 YAML 文件会直接打包进 EXE，修改后必须重新构建并发布新版程序。

## 只需要维护这四个文件

- `projects.yaml`：只维护项目 ID 和项目名称。
- `tasks.yaml`：只维护测试任务 ID、名称和类型。
- `keywords.yaml`：只维护关键字方案、类别和每个独立关键字。
- `defaults.yaml`：只维护串口、日志、存储和上传默认值。

`branding/app-icon-source.png` 是图标素材，不属于运行配置。

## 增加项目

在 `projects.yaml` 的末尾增加一行：

```yaml
- { id: dr9999, name: DR9999 }
```

`id` 发布后不要修改，否则已保存通道会认为原项目已删除。

## 增加测试任务

在 `tasks.yaml` 的末尾增加一行：

```yaml
- { id: long-aging, name: 长时间挂测, type: special }
```

普通挂测的 `type` 写 `normal`，其他专项写 `special`。

## 增加关键字

在 `keywords.yaml` 找到对应类别，在 `rules` 下增加一行：

```yaml
- { id: unique-rule-id, name: 界面显示文字, match: 实际匹配内容, mode: contains, case_sensitive: false }
```

- `id`：全局稳定标识，不能与现有关键字重复。
- `name`：界面上看到的名称。
- `match`：日志中实际查找的文本。
- `mode`：普通文本写 `contains`，正则表达式写 `regex`。
- `case_sensitive`：是否严格区分大小写。

新增类别时复制一个完整的 `id/name/scope/rules` 块即可。每条关键字必须独立列出，不能把多个枚举合并成一条。

## 修改默认值

`defaults.yaml` 使用秒、分钟、MB、GB 等直观单位。例如分段大小写 `segment_size_mb: 32`，存储保护写 `limit_gb: 50`。

修改任何文件后必须执行测试和 Wails 构建。配置语法、重复 ID、空匹配内容或不支持的版本会让测试失败，不能直接发布。

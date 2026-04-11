# gtoh migrate 进度条 — 测试检查清单

## 一、单元测试

### 1.1 shouldUpdate 测试

| 测试用例 | current | total | 预期 |
|----------|---------|-------|------|
| 小批量-第一个 | 1 | 100 | true |
| 小批量-中间 | 50 | 100 | true |
| 小批量-最后 | 100 | 100 | true |
| 大批量-第一个 | 1 | 10000 | true |
| 大批量-第10个 | 10 | 10000 | true |
| 大批量-第11个 | 11 | 10000 | false |
| 大批量-第20个 | 20 | 10000 | true |
| 大批量-最后 | 10000 | 10000 | true |

### 1.2 scanFiles 测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 空目录 | 无媒体文件 | 返回空切片 |
| 单文件 | 1 个 jpg | 返回 1 个 FileEntry |
| 多格式混合 | jpg + json + mp4 | 返回 2 个（跳过 json） |
| 嵌套目录 | 子目录中有文件 | 递归收集所有文件 |
| 多个 yearFolders | 2 个年份文件夹 | 合并所有文件 |

### 1.3 processSingleFile 测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 完整流程 | 有 JSON 的图片 | 复制 + exiftool + metadata |
| 无 JSON | 无侧车文件 | 继续处理，deviceFolder 为空 |
| 无时间戳 | 无法解析时间 | 跳过，stats.SkippedNoTime++ |
| 文件冲突 | 输出目录已有同名 | 跳过，stats.SkippedExists++ |
| exiftool 失败 | 不支持的格式 | 跳过，stats.FailedExif++ |

## 二、集成测试

### 2.1 小批量（100 文件）

准备 100 个测试文件（含 JSON 侧车），运行：
```bash
gtoh migrate test_input/ test_output/
```

验证：
- [ ] 进度条从 0% 到 100% 更新 100 次
- [ ] 终端输出无闪烁
- [ ] 最终统计 scanned=100
- [ ] metadata 目录有对应数量的 JSON 文件

### 2.2 大批量（1000 文件）

验证：
- [ ] 进度条每 1% 更新一次（每 10 个文件）
- [ ] 最终显示 100%
- [ ] 处理时间合理

### 2.3 超大批量（10000 文件）

验证：
- [ ] 进度条每 10 个文件更新一次
- [ ] 内存稳定，不 OOM
- [ ] 处理可完成

### 2.4 管道输出

```bash
gtoh migrate test_input/ test_output/ | cat
```

验证：
- [ ] 不报错
- [ ] 输出可读（`\r` 不影响）
- [ ] 最终统计正常显示

### 2.5 无文件场景

```bash
mkdir -p empty/"Photos from 2024"
gtoh migrate empty/ output/
```

验证：
- [ ] 显示 `No media files found in year folders.`
- [ ] 不显示进度条
- [ ] exit 0

## 三、回归测试

| 测试 | 命令 | 预期 |
|------|------|------|
| 所有现有测试 | `go test ./...` | 全部通过 |
| 编译检查 | `go build ./...` | 无错误 |
| 原有功能 | `gtoh migrate` 基本流程 | 与之前一致，仅增加进度条 |
| test_matcher | `./test_matcher -folders <dir>` | 不受影响 |

## 四、手动验证

### 4.1 视觉检查

```bash
# 创建 50 个测试文件
gtoh migrate test_50/ output_50/
```

观察：
- [ ] 进度条平滑更新
- [ ] 百分比递增
- [ ] 完成后换行
- [ ] 统计信息正确

### 4.2 日志文件检查

```bash
cat output_50/gtoh.log
```

验证：
- [ ] 无进度条字符
- [ ] SKIP/FAIL 记录完整
- [ ] 时间戳格式正确

### 4.3 性能对比

对比添加进度条前后的处理时间：
- [ ] 100 文件：差异 < 1 秒
- [ ] 1000 文件：差异 < 5 秒
- [ ] 进度条更新不成为性能瓶颈

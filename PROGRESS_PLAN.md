# gtoh migrate 进度条 — 需求文档

## 一、现状分析

### 1.1 当前问题

`migrator.Run()` 在 `filepath.Walk` 中逐个处理文件，无法提前知道文件总数，因此无法展示进度条。

### 1.2 已有进度工具

`internal/progress/logger.go` 已提供 `PrintProgress(current, total int)` 函数，实现带回车符的进度显示。

---

## 二、需求

### 2.1 进度条展示内容

```
Processing: [████████░░░░░░░░░░░░] 40% (120/300) ETA: 2m30s
```

或简化版（与现有 `PrintProgress` 一致）：

```
Processing: 120/300 (40%)
```

### 2.2 展示时机

| 阶段 | 展示内容 |
|------|----------|
| 扫描阶段 | `Scanning files...` |
| 处理阶段 | 进度条实时更新 |
| 完成阶段 | 最终统计（已有） |

### 2.3 更新频率

- 每处理 **1 个文件**更新一次（文件数 < 1000）
- 每处理 **10 个文件**更新一次（文件数 >= 1000）
- 避免频繁刷新导致终端闪烁

---

## 三、实现方案

### 3.1 两阶段处理

将当前的单遍 `filepath.Walk` 拆分为两阶段：

```
阶段1: 扫描计数
  └─ filepath.Walk → 收集所有媒体文件路径 → 得到 totalCount

阶段2: 逐个处理
  └─ 遍历文件列表 → 每处理一个 → 更新进度条
```

### 3.2 数据结构变更

```go
// 新增：文件预处理结果
type FileEntry struct {
    Path     string // 绝对路径
    RelPath  string // 相对路径（用于日志）
    YearFolder string // 所属年份文件夹
}

// Config 新增字段
type Config struct {
    InputDir  string
    OutputDir string
    ShowProgress bool // 是否显示进度条（默认 true）
}
```

### 3.3 流程变更

```
Run(cfg)
  ├─ Phase 1: scanFiles(inputDir) → []FileEntry
  │   └─ filepath.Walk 收集所有媒体文件
  │   └─ 输出: "Found N files in M year folder(s)"
  │
  ├─ Phase 2: processFiles(entries, outputDir, ...)
  │   ├─ 初始化进度条
  │   ├─ for i, entry := range entries:
  │   │   ├─ 处理文件（现有逻辑）
  │   │   └─ if shouldUpdate(i, len(entries)):
  │   │       └─ PrintProgress(i+1, len(entries))
  │   └─ 输出换行
  │
  └─ 输出统计结果
```

### 3.4 进度条更新策略

```go
func shouldUpdate(current, total int) bool {
    if total < 1000 {
        return true // 每个文件都更新
    }
    return current%10 == 0 || current == total // 每10个更新一次
}
```

### 3.5 终端兼容性

| 场景 | 行为 |
|------|------|
| 标准终端（TTY） | 显示进度条（使用 `\r` 覆盖） |
| 管道/重定向 | 不显示进度条（检测 `!isatty`） |
| CI 环境 | 不显示进度条 |

### 3.6 与日志的协调

进度条使用 `\r` 覆盖当前行，log 文件写入不受影响。

**问题**：处理过程中如果调用 `logger.Skip()` 或 `logger.Fail()` 写入日志文件，不影响终端进度条。

但如果需要在终端打印警告信息（如 `progress.Warning`），会破坏进度条显示。

**解决方案**：
- 处理过程中**不在终端打印** SKIP/FAIL 信息
- 所有 SKIP/FAIL 仅写入 log 文件
- 最终统计中汇总展示

---

## 四、输出示例

### 4.1 正常流程

```
Input:  /data/takeout
Output: /data/output

Scanning files...
Found 1,234 files in 3 year folder(s)

Processing: 500/1234 (40%)

Processing complete!
  Scanned:            1234 files
  Processed:          1200 files
  Skipped (no time):  20 files
  Skipped (exists):   10 files
  Failed (exiftool):  3 files
  Failed (other):     1 files
  Log:                /data/output/gtoh.log
```

### 4.2 无文件

```
Input:  /data/empty
Output: /data/output

Scanning files...
No media files found in year folders.
```

### 4.3 管道输出（无进度条）

```
Input:  /data/takeout
Output: /data/output

Found 1,234 files in 3 year folder(s)
Processing complete!
  Scanned:            1234 files
  ...
```

---

## 五、改动范围

| 文件 | 改动 |
|------|------|
| `internal/migrator/migrator.go` | 拆分为两阶段，添加进度回调 |
| `internal/progress/logger.go` | 确认 `PrintProgress` 满足需求 |
| `cmd/gtoh/cmd/migrate.go` | 传递进度回调函数 |

### 5.1 不改动

- `logger.go`（log 文件写入）
- `copier.go`
- `exif_writer.go`
- `hasher.go`
- `metadata.go`

---

## 六、验收标准

- [ ] 100 文件：进度条从 0% 到 100% 平滑更新
- [ ] 1000 文件：进度条每 1% 更新一次
- [ ] 10000 文件：进度条每 10 个文件更新一次，不卡顿
- [ ] 管道输出（`| cat`）：不输出进度条字符
- [ ] 最终统计数字与进度条最终值一致
- [ ] 进度条不干扰 log 文件写入

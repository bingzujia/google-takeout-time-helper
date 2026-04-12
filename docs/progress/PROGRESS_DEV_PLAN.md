# gtoh migrate 进度条 — 开发文档

## 一、现状

### 1.1 已有进度工具

`internal/progress/logger.go` 已提供 `PrintProgress(current, total int)`：
- 使用 `\r` 覆盖当前行
- 显示格式：`🔄 [+++++---------------] 25% (5/20)`
- 进度条宽度 20 字符

### 1.2 当前 migrator 问题

`migrator.Run()` 使用单遍 `filepath.Walk`，无法提前知道文件总数，无法展示进度条。

---

## 二、架构变更

### 2.1 两阶段重构

将 `processYearFolder` 中的 `filepath.Walk` 拆分为：

```
Phase 1: scanFiles(yearFolders) → []FileEntry
  └─ 遍历所有 yearFolders，收集媒体文件路径

Phase 2: processFiles(entries, ...)
  └─ 遍历文件列表，逐个处理，更新进度条
```

### 2.2 新增类型

```go
// FileEntry holds pre-scanned file information.
type FileEntry struct {
    Path       string // 绝对路径
    RelPath    string // 相对于 inputDir 的路径（用于日志）
}
```

### 2.3 Config 变更

```go
type Config struct {
    InputDir     string
    OutputDir    string
    ShowProgress bool // 新增：是否显示进度条，默认 true
}
```

---

## 三、实现细节

### 3.1 scanFiles 函数

```go
func scanFiles(yearFolders []string, inputDir string) []FileEntry {
    var entries []FileEntry
    for _, yf := range yearFolders {
        filepath.Walk(yf, func(path string, info os.FileInfo, err error) error {
            if err != nil || info.IsDir() {
                return nil
            }
            if !mediaExts[strings.ToLower(filepath.Ext(path))] {
                return nil
            }
            relPath, _ := filepath.Rel(inputDir, path)
            entries = append(entries, FileEntry{
                Path:    path,
                RelPath: relPath,
            })
            return nil
        })
    }
    return entries
}
```

### 3.2 processFiles 函数

```go
func processFiles(entries []FileEntry, outputDir, metadataDir string,
    logger *Logger, exifWriter *ExifWriter, stats *Stats, showProgress bool) {

    for i, entry := range entries {
        // 处理文件（现有逻辑，从 processYearFolder 提取）
        processSingleFile(entry, outputDir, metadataDir, logger, exifWriter, stats)

        // 更新进度条
        if showProgress && shouldUpdate(i+1, len(entries)) {
            progress.PrintProgress(i+1, len(entries))
        }
    }

    // 处理完成后输出换行
    if showProgress && len(entries) > 0 {
        fmt.Println()
    }
}
```

### 3.3 shouldUpdate 函数

```go
func shouldUpdate(current, total int) bool {
    if total < 1000 {
        return true // 每个文件都更新
    }
    return current%10 == 0 || current == total // 每10个更新一次
}
```

### 3.4 Run 函数重构

```go
func Run(cfg Config) (*Stats, error) {
    // 1. 检查输出目录
    // 2. 创建输出目录
    // 3. 初始化 logger
    // 4. ClassifyFolder → yearFolders

    // Phase 1: 扫描
    fmt.Println("Scanning files...")
    entries := scanFiles(yearFolders, cfg.InputDir)
    if len(entries) == 0 {
        fmt.Println("No media files found in year folders.")
        return &Stats{}, nil
    }
    fmt.Printf("Found %d files in %d year folder(s)\n\n", len(entries), len(yearFolders))

    // Phase 2: 处理
    exifWriter := &ExifWriter{}
    stats := &Stats{}
    processFiles(entries, cfg.OutputDir, metadataDir, logger, exifWriter, stats, cfg.ShowProgress)

    return stats, nil
}
```

---

## 四、改动文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/migrator/migrator.go` | 重构 | 拆分为 scanFiles + processFiles + processSingleFile |
| `internal/migrator/migrator.go` | 新增 | `FileEntry` 类型、`shouldUpdate` 函数 |
| `internal/migrator/migrator.go` | 修改 | `Config` 增加 `ShowProgress` 字段 |
| `cmd/gtoh/cmd/migrate.go` | 修改 | 传递 `ShowProgress: true` |

### 4.1 不改动

- `internal/migrator/logger.go`
- `internal/migrator/copier.go`
- `internal/migrator/exif_writer.go`
- `internal/migrator/hasher.go`
- `internal/migrator/metadata.go`
- `internal/progress/logger.go`（已有 `PrintProgress` 满足需求）

---

## 五、进度条行为

### 5.1 终端检测

当前 `PrintProgress` 直接使用 `fmt.Printf("\r...")`，在管道输出时仍会输出 `\r` 字符。

**可选优化**（非必须）：
```go
import "golang.org/x/term"

func isTTY() bool {
    return term.IsTerminal(int(os.Stdout.Fd()))
}
```

**当前方案**：不检测 TTY，依赖用户自行判断。管道输出时 `\r` 字符会被终端忽略。

### 5.2 与日志协调

- 进度条仅输出到 stdout
- log 文件写入不受影响
- 处理过程中不在终端打印 SKIP/FAIL（避免破坏进度条）
- 最终统计统一展示

---

## 六、输出示例

### 6.1 正常流程

```
Input:  /data/takeout
Output: /data/output

Scanning files...
Found 1,234 files in 3 year folder(s)

🔄 [++++++++++----------] 50% (617/1234)

Processing complete!
  Scanned:            1234 files
  Processed:          1200 files
  Skipped (no time):  20 files
  Skipped (exists):   10 files
  Failed (exiftool):  3 files
  Failed (other):     1 files
  Log:                /data/output/gtoh.log
```

### 6.2 无文件

```
Input:  /data/empty
Output: /data/output

Scanning files...
No media files found in year folders.
```

---

## 七、实现步骤

1. 在 `migrator.go` 中添加 `FileEntry` 类型
2. 提取 `scanFiles()` 函数
3. 提取 `processSingleFile()` 函数（从 `processYearFolder` 的 Walk 回调中提取）
4. 实现 `processFiles()` 函数（带进度条）
5. 实现 `shouldUpdate()` 函数
6. 重构 `Run()` 为两阶段
7. 删除旧的 `processYearFolder()` 函数
8. 更新 `Config` 添加 `ShowProgress` 字段
9. 更新 `cmd/gtoh/cmd/migrate.go` 传递配置

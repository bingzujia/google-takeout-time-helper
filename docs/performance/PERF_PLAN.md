# gtoh 性能优化 — 开发文档

## 一、现状分析

### 1.1 当前性能瓶颈

| 步骤 | 操作 | 预估耗时 | 瓶颈类型 |
|------|------|----------|----------|
| ParseEXIFTimestamp | 启动 exiftool 进程 | 50-200ms | 进程启动 |
| ParseEXIFGPS | 启动 exiftool 进程 | 50-200ms | 进程启动 |
| IsWriteSupported | 启动 file 进程 | 5-20ms | 进程启动 |
| DetectFileType | 启动 file 进程 | 5-20ms | 进程启动 |
| CopyFile | 读入内存 + 写入磁盘 | 10-500ms | I/O + 内存 |
| exifWriter.WriteAll | 启动 exiftool 进程 | 50-200ms | 进程启动 |
| SHA256File | 读取整个文件 | 10-500ms | I/O |
| 其他 | 正则、JSON、日志 | < 1ms | 可忽略 |

**单文件总耗时：~200-1500ms**（取决于文件大小和 exiftool 响应）

**核心问题**：
1. 每个文件串行处理，无法利用多核 CPU
2. 每个文件启动 3 次 exiftool 进程 + 2 次 file 进程
3. 大文件被读取两次（CopyFile + SHA256File）

---

## 二、优化方案

### 方案 1：并发处理（P0，最大收益）

#### 2.1.1 原理

将 `processFiles` 的串行 for 循环改为 worker pool 并发处理。

```
当前：file1 → file2 → file3 → ...  (N × T)
并发：file1 ─┐
             ├→ worker1
      file2 ─┤
             ├→ worker2   (N × T / workers)
      file3 ─┘
```

#### 2.1.2 实现

```go
func processFiles(entries []FileEntry, outputDir, metadataDir string,
    logger *Logger, exifWriter *ExifWriter, stats *Stats, showProgress bool) {

    workers := runtime.NumCPU()
    if workers > 8 {
        workers = 8 // 限制最大并发数
    }

    var wg sync.WaitGroup
    var mu sync.Mutex // 保护 stats 和 logger
    jobCh := make(chan FileEntry, len(entries))

    // 启动 workers
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for entry := range jobCh {
                processSingleFile(entry, outputDir, metadataDir, logger, exifWriter, stats, &mu)
            }
        }()
    }

    // 分发任务
    for _, entry := range entries {
        jobCh <- entry
    }
    close(jobCh)

    // 等待完成
    wg.Wait()
}
```

#### 2.1.3 注意事项

- `stats` 和 `logger` 需要加锁保护
- 进度条需要加锁或使用原子操作
- 并发数建议 `min(runtime.NumCPU(), 8)`，避免 exiftool 进程过多

#### 2.1.4 预期收益

- 4 核 CPU：~3-4x 加速
- 8 核 CPU：~6-8x 加速

---

### 方案 2：合并 exiftool 读取（P0，高收益）

#### 2.2.1 原理

当前每个文件启动 2 次 exiftool 读取进程：
- `ParseEXIFTimestamp` → `exiftool -j -DateTimeOriginal file`
- `ParseEXIFGPS` → `exiftool -n -j -GPS* file`

合并为一次调用：
```bash
exiftool -j -DateTimeOriginal -GPSLatitude -GPSLongitude file
```

#### 2.2.2 实现

在 `parser` 包新增 `ParseEXIFAll` 函数：

```go
type EXIFInfo struct {
    Timestamp time.Time
    TimestampOk bool
    GPS       GPSInfo
}

func ParseEXIFAll(filePath string) (*EXIFInfo, error) {
    cmd := exec.Command("exiftool", "-j", "-n",
        "-DateTimeOriginal", "-GPSLatitude", "-GPSLongitude", filePath)
    // 解析 JSON 输出，同时提取时间戳和 GPS
}
```

#### 2.2.3 改动范围

- `internal/parser/exiftool.go` — 新增 `ParseEXIFAll`
- `internal/migrator/migrator.go` — 替换 `ParseEXIFTimestamp` + `ParseEXIFGPS` 调用

#### 2.2.4 预期收益

- 减少 1 次 exiftool 进程启动（~50-200ms/文件）
- 对于 10000 文件：节省 ~8-33 分钟

---

### 方案 3：流式复制 + SHA-256（P0，高收益）

#### 2.3.1 原理

当前 `CopyFile` 和 `SHA256File` 各读取一次文件。合并为一次读取，同时完成复制和哈希计算。

```go
func CopyAndHash(src, dst string) (sha256 string, exists bool, err error) {
    // 检查目标是否已存在
    if _, err := os.Stat(dst); err == nil {
        return "", true, nil
    }

    srcF, _ := os.Open(src)
    dstF, _ := os.Create(dst)
    h := sha256.New()
    io.Copy(dstF, io.TeeReader(srcF, h)) // 一次读取，同时写入和哈希
    return hex.EncodeToString(h.Sum(nil)), false, nil
}
```

#### 2.3.2 改动范围

- `internal/migrator/copier.go` — 新增 `CopyAndHash` 函数
- `internal/migrator/migrator.go` — 替换 `CopyFile` + `SHA256File` 调用

#### 2.3.3 预期收益

- 大文件 I/O 减半
- 内存占用从 O(文件大小) 降为 O(buffer_size)
- 500MB 视频：从 ~1s 降到 ~0.5s

---

### 方案 4：合并 file 命令调用（P1，中收益）

#### 2.4.1 原理

当前 `IsWriteSupported` 和 `DetectFileType` 各启动一次 `file` 进程。合并为一次调用，复用结果。

#### 2.4.2 实现

```go
type FileType struct {
    MimeType  string
    NewExt    string // 需要重命名的扩展名，空表示无需重命名
    Supported bool   // exiftool 是否支持写入
}

func DetectFileAll(filePath string) (*FileType, error) {
    cmd := exec.Command("file", "--brief", "--mime-type", filePath)
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    mime := strings.TrimSpace(string(out))
    return &FileType{
        MimeType:  mime,
        NewExt:    mimeToExt(mime),
        Supported: mime != "video/x-ms-wmv",
    }, nil
}
```

#### 2.4.3 改动范围

- `internal/migrator/filetype.go` — 新增 `DetectFileAll`，替换 `DetectFileType` + `IsWriteSupported`
- `internal/migrator/migrator.go` — 更新调用

#### 2.4.4 预期收益

- 减少 1 次 file 进程启动（~5-20ms/文件）

---

### 方案 5：跳过不必要的 exiftool 读取（P1，中收益）

#### 2.5.1 原理

当前即使文件名能解析出时间戳，仍然调用 `ParseEXIFTimestamp` 启动 exiftool。

优化：先尝试文件名解析，失败后才调用 exiftool。

#### 2.5.2 实现

```go
// 当前逻辑（总是调用 exiftool）
exifTimestamp, exifTimeOk := parser.ParseEXIFTimestamp(entry.Path)
filenameTimestamp, filenameTimeOk := parser.ParseFilenameTimestamp(...)

// 优化后（文件名可解析时跳过 exiftool）
filenameTimestamp, filenameTimeOk := parser.ParseFilenameTimestamp(filepath.Base(entry.Path))
var exifTimestamp time.Time
var exifTimeOk bool
if !filenameTimeOk {
    // 文件名无法解析，才调用 exiftool
    exifTimestamp, exifTimeOk = parser.ParseEXIFTimestamp(entry.Path)
}
```

#### 2.5.3 预期收益

- 文件名可解析的文件（IMG/VID/Screenshot 等）：节省 50-200ms
- 对于 Google Takeout 数据，大部分文件名已包含时间戳

---

## 三、实施状态

| 阶段 | 方案 | 预期加速 | 状态 | 备注 |
|------|------|----------|------|------|
| 1 | 方案 3：流式复制+SHA256 | 1.2-1.5x | ✅ 已完成 | `CopyAndHash` 替代 `CopyFile` + `SHA256File` |
| 2 | 方案 5：跳过不必要的 exiftool | 1.1-1.3x | ✅ 已完成 | 文件名优先解析，失败后才调用 exiftool |
| 3 | 方案 4：合并 file 命令 | 1.05x | ✅ 已完成 | `DetectFileAll` 缓存结果，避免重复调用 |
| 4 | 方案 2：合并 exiftool 读取 | 1.5-2x | ✅ 已完成 | `ParseEXIFAll` 同时获取时间戳和 GPS |
| 5 | 方案 1：并发处理 | 3-8x | ✅ 已完成 | worker pool + sync.Mutex + atomic.Int64 |

**已完成组合效果**：方案 2+3+4+5 实施后预计 **2-4x** 加速（不含并发）。
**最终目标**：全部实施后预计 **5-10x** 整体加速。

---

## 四、风险与回退

| 方案 | 风险 | 回退策略 |
|------|------|----------|
| 并发处理 | exiftool 进程过多 | 可配置并发数，默认 4 |
| 合并 exiftool 读取 | JSON 解析失败 | 失败时回退到单独调用 |
| 流式复制 | 无 | 无风险 |
| 合并 file 命令 | 无 | 无风险 |
| 跳过 exiftool 读取 | 文件名解析错误导致漏掉 EXIF 时间戳 | 保留优先级逻辑不变 |

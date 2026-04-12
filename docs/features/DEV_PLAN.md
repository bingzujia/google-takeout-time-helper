# gtoh 核心迁移命令 — 开发文档

## 一、项目现状

### 1.1 已有可复用模块

| 模块 | 功能 | 文件 |
|------|------|------|
| `parser.ParseEXIFTimestamp` | 通过 exiftool 读取 DateTimeOriginal | `internal/parser/exiftool.go` |
| `parser.ParseFilenameTimestamp` | 从文件名解析时间戳（9 种正则模式） | `internal/parser/timestamp.go` |
| `parser.ParseEXIFGPS` | 通过 exiftool 读取 GPS 坐标 | `internal/parser/exifgps.go` |
| `matcher.JSONForFile` | 6+ 步降级策略匹配 JSON 侧车文件 | `internal/matcher/json_matcher.go` |
| `matcher.ResolveTimestamp` | 3 层时间戳优先级：EXIF > Filename > JSON | `internal/matcher/json_matcher.go` |
| `matcher.ResolveGPS` | 2 层 GPS 优先级：EXIF > JSON | `internal/matcher/json_matcher.go` |
| `metadata.ExifToolWriter` | 通过 exiftool 写入时间戳和 GPS | `internal/metadata/writer.go` |
| `organizer.ClassifyFolder` | 识别 yearFolders / albumFolders | `internal/organizer/folder_classify.go` |
| `progress` | 进度和日志输出 | `internal/progress/logger.go` |

### 1.2 需要修复的 Bug

| 问题 | 位置 | 说明 |
|------|------|------|
| `matcher.MatchAll` 不存在 | `cmd/gtoh/cmd/fix_takeout.go:53` | 调用未定义函数，项目无法编译 |

### 1.3 需要新建的功能

| 功能 | 优先级 | 说明 |
|------|--------|------|
| `gtoh` 核心命令 | P0 | 接收 `<input_dir> <output_dir>` 两个参数 |
| 扁平复制文件 | P0 | 复制媒体文件到输出目录根，处理文件名冲突 |
| exiftool 写入 FileModifyDate | P0 | 覆盖文件系统修改时间 |
| SHA-256 计算 | P0 | 对修改后的文件计算哈希 |
| metadata JSON 写入 | P0 | 写入 `metadata/<sha256>.json` |
| log 文件写入 | P0 | 写入 `gtoh.log` |
| 媒体文件遍历 | P0 | 递归遍历 yearFolders 收集媒体文件 |
| 文件类型检测 | P0 | `file` 命令检测实际类型，处理扩展名不匹配 |
| error 目录 | P0 | 失败文件移动到 `output/error/` 保留原始路径 |

---

## 二、架构设计

### 2.1 新增包：`internal/migrator`

```
internal/migrator/
├── migrator.go      # 核心迁移逻辑
├── copier.go        # 文件复制（扁平，冲突处理）
├── exif_writer.go   # exiftool 封装（DateTimeOriginal + GPS + FileModifyDate）
├── hasher.go        # SHA-256 计算
├── metadata.go      # metadata JSON 写入
├── logger.go        # log 文件写入
├── filetype.go      # 文件类型检测（file 命令封装）
└── migrator_test.go # 测试
```

### 2.2 数据流

```
input_dir
  └─ ClassifyFolder() → yearFolders[]
       └─ walk media files
            ├─ JSONForFile() → deviceFolder, deviceType（无 JSON 时为空，记录到 log）
            ├─ ParseEXIFTimestamp() → exifTimestamp
            ├─ ParseFilenameTimestamp() → filenameTimestamp
            ├─ JSON timestamp → jsonTimestamp
            ├─ ParseEXIFGPS() → exifGPS
            ├─ JSON GPS → jsonGPS
            ├─ 确定最终 timestamp（EXIF > Filename > JSON）
            ├─ 确定最终 GPS（EXIF > JSON）
            ├─ copyToOutput() → output_dir/filename.ext
            ├─ detectFileType() → 检测实际文件类型
            ├─ 类型不匹配 → 临时重命名为正确扩展名
            │   ├─ exiftool 写入临时文件
            │   ├─ 恢复原始文件名
            │   └─ 临时重命名冲突 → 移到 error/，记录 log
            ├─ sha256() → metadata/<sha256>.json
            └─ logWrite() → gtoh.log
```

### 2.3 输出目录结构

```
output_dir/
├── IMG_20240315_103000.jpg          # 成功处理的文件（扁平）
├── VID_20240315_103001.mp4
├── metadata/
│   ├── abc123def456....json
│   └── ...
├── error/
│   ├── Photos from 2015/
│   │   ├── DSC_0002.jpg             # 尼康 IFD 损坏
│   │   ├── DSC_0002.jpg.json        # 配对 JSON 同时移动
│   │   └── ...
│   ├── Photos from 2020/
│   │   ├── 评教评学.wmv             # WMV 不支持写入
│   │   └── 评教评学.wmv.json        # 配对 JSON 同时移动
│   └── Photos from 2022/
│       ├── IMG20221110153719.heic   # 扩展名不匹配且重命名冲突
│       └── IMG20221110153719.heic.json  # 配对 JSON 同时移动
└── gtoh.log
```

### 2.4 核心类型

```go
type FileInfo struct {
    OriginalPath   string       // 原始路径
    OutputFilename string       // 输出文件名
    SHA256         string       // 修改后文件的 SHA-256

    // 时间戳（三种来源全部记录）
    ExifTimestamp     time.Time
    FilenameTimestamp time.Time
    JSONTimestamp     time.Time
    FinalTimestamp    time.Time
    TimestampSource   string // "exif" | "filename" | "json" | "none"

    // GPS（两种来源全部记录）
    ExifGPS     GPSInfo
    JSONGPS     GPSInfo
    FinalGPS    GPSInfo
    GPSSource   string // "exif" | "json" | "none"

    // 设备信息
    DeviceFolder string
    DeviceType   string
}

type Stats struct {
    Scanned          int
    Processed        int
    SkippedNoTime    int
    SkippedExists    int
    FailedExiftool   int
    FailedOther      int
}
```

### 2.5 metadata JSON 结构

```json
{
  "original_path": "Photos from 2024/IMG_xxx.jpg",
  "output_filename": "IMG_xxx.jpg",
  "sha256": "abc123...",
  "timestamp": {
    "final": "2024-03-15T10:30:00Z",
    "source": "exif",
    "exif": "2024-03-15T10:30:00Z",
    "filename": "2024-03-15T10:30:00Z",
    "json": "2024-03-15T10:30:00Z"
  },
  "gps": {
    "lat": 30.5085,
    "lon": 104.0395,
    "source": "exif",
    "exif": { "lat": 30.5085, "lon": 104.0395 },
    "json": { "lat": 30.5080, "lon": 104.0390 }
  },
  "device_folder": "Tim_Images",
  "device_type": "ANDROID_PHONE"
}
```

### 2.6 log 文件格式

```
[2024-04-11 10:30:00] SKIP no_timestamp: Photos from 2024/unknown.jpg
[2024-04-11 10:30:01] SKIP file_exists: Photos from 2024/duplicate.jpg
[2024-04-11 10:30:02] FAIL exiftool_write: Photos from 2024/bad.heic (error: ...)
[2024-04-11 10:30:03] FAIL rename_conflict: Photos from 2022/IMG20221110153719.heic (actual: JPEG, target: IMG20221110153719.jpg exists)
[2024-04-11 10:30:04] FAIL filetype_unsupported: Photos from 2020/评教评学.wmv (WMV write not supported)
```

---

## 三、文件类型检测与重命名

### 3.1 `file` 命令输出映射表

| `file` 输出关键字 | 目标扩展名 | 示例 |
|---|---|---|
| `JPEG image data` | `.jpg` | `IMG20221110153719.heic` → `IMG20221110153719.jpg` |
| `PNG image data` | `.png` | `Screenshot_xxx.jpg` → `Screenshot_xxx.png` |
| `RIFF` + `WebP` | `.webp` | `xxx.jpg` → `xxx.webp` |
| `RIFF` + `AVI` | `.avi` | `xxx.jpg` → `xxx.avi` |
| `ISO Media` + `MP4` | `.mp4` | `xxx.jpg` → `xxx.mp4` |
| `ISO Media` + `MOV` | `.mov` | `xxx.jpg` → `xxx.mov` |
| `MPEG sequence` | `.mpg` | — |
| `GIF image` | `.gif` | — |
| `HEIF` | `.heic` | — |

### 3.2 临时重命名策略

```
检测到类型不匹配 → 计算临时文件名（替换扩展名）
  ├─ 临时文件不存在 → 临时重命名 → exiftool 写入 → 恢复原名
  └─ 临时文件已存在 → 移到 error/，记录 log
      └─ error/Photos from 2022/IMG20221110153719.heic
          └─ 配对 JSON 同时移动
```

**关键设计**：
- `handleTypeMismatch()` 返回 `(tmpPath, cleanup, err)`
- `cleanup()` 是闭包函数，负责将临时文件名恢复为原始文件名
- exiftool 写入成功后立即调用 `cleanup()` 恢复原名
- SHA-256 和 metadata 使用恢复后的原始文件名

### 3.3 exiftool 写入策略

```bash
# 添加 -ignoreMinorErrors 处理尼康 IFD 损坏等 minor 错误
exiftool -ignoreMinorErrors -overwrite_original \
  -DateTimeOriginal="2024:03:15 10:30:00" \
  -GPSLatitude=30.5085 \
  -GPSLongitude=104.0395 \
  -FileModifyDate="2024:03:15 10:30:00" \
  output_dir/IMG_xxx.jpg
```

`-ignoreMinorErrors` 处理的错误类型：
- `[minor] Bad NikonScanIFD offset` — 尼康扫描 IFD 损坏
- `[minor] IFD0 pointer references previous IFD0` — PNG IFD 循环引用
- `[minor] Possible garbage at end of file` — MP4 尾部垃圾数据

### 3.4 exiftool 失败处理

```
exiftool 写入失败
  ├─ 文件已复制到 output/
  ├─ 计算 error 目录路径：error/<原始相对路径>
  ├─ 创建 error 子目录
  ├─ 移动图片文件到 error 目录
  ├─ 移动 JSON 侧车文件（如有）到 error 目录
  └─ 记录 log：FAIL exiftool_write: path (error)
```

**注意**：所有失败场景（exiftool 失败、重命名冲突、不支持的格式）都需要将**图片文件和其 JSON 侧车文件同时移动**到 error 目录，保持配对关系完整。

---

## 四、实现步骤

### Step 1: 修复编译错误

- 删除或修复 `fix_takeout.go` 中对 `matcher.MatchAll` 的调用
- 确认 `go build ./...` 通过

### Step 2: 实现 `internal/migrator` 包

按以下顺序实现：

1. `logger.go` — log 文件写入器
2. `filetype.go` — `file` 命令封装，类型检测与扩展名映射
3. `copier.go` — 扁平复制，冲突检测
4. `exif_writer.go` — exiftool 封装（添加 `-ignoreMinorErrors`）
5. `hasher.go` — SHA-256 计算
6. `metadata.go` — metadata JSON 写入
7. `migrator.go` — 核心编排逻辑（两阶段 + 进度条 + error 目录）

### Step 3: 注册 `gtoh` 命令

- 在 `cmd/gtoh/cmd/` 下创建 `migrate.go`
- 在 `root.go` 中注册为默认命令或直接作为根命令

### Step 4: 编写测试

- 单元测试：每个子模块
- 集成测试：完整流程（使用测试图片）

---

## 五、exiftool 命令格式

```bash
# 写入时间戳 + GPS + FileModifyDate（忽略 minor 错误）
exiftool -ignoreMinorErrors -overwrite_original \
  -DateTimeOriginal="2024:03:15 10:30:00" \
  -GPSLatitude=30.5085 \
  -GPSLongitude=104.0395 \
  -FileModifyDate="2024:03:15 10:30:00" \
  output_dir/IMG_xxx.jpg
```

- 无 GPS 时不传 GPS 参数
- 时间格式：`YYYY:MM:DD HH:MM:SS`
- `-overwrite_original` 避免生成 `_original` 备份
- `-ignoreMinorErrors` 忽略 minor 级别错误继续写入

---

## 六、错误处理策略

| 错误类型 | 行为 | 日志 | error 目录 | JSON 处理 |
|----------|------|------|------------|-----------|
| 无时间戳 | 跳过文件 | `SKIP no_timestamp: path` | — | 保留原位置 |
| 文件名冲突 | 跳过文件 | `SKIP file_exists: path` | — | 保留原位置 |
| 类型不匹配+重命名冲突 | 移到 error | `FAIL rename_conflict: path` | `error/<原始路径>` | 同时移动 |
| exiftool 失败 | 移到 error | `FAIL exiftool_write: path (error)` | `error/<原始路径>` | 同时移动 |
| 不支持的格式（WMV） | 移到 error | `FAIL filetype_unsupported: path` | `error/<原始路径>` | 同时移动 |
| IO 错误 | 移到 error | `FAIL io_error: path (error)` | `error/<原始路径>` | 同时移动 |
| JSON 解析失败 | 移到 error | `FAIL json_write: path (error)` | `error/<原始路径>` | 同时移动 |
| 无 JSON 侧车文件 | 记录到 log（不跳过） | `INFO no_json_sidecar: path` | — | — |

### 6.1 错误类型覆盖矩阵

| 实际错误 | 数量 | 处理方案 | 结果 |
|----------|------|----------|------|
| Not a valid HEIC (JPEG) | 1661 | `file` 检测 → 重命名为 `.jpg` | ✅ 继续处理 |
| Not a valid PNG (JPEG) | 350 | `file` 检测 → 重命名为 `.jpg` | ✅ 继续处理 |
| NikonScanIFD 损坏 | 230 | `-ignoreMinorErrors` | ✅ 继续处理 |
| Not a valid JPG (MOV) | 10 | `file` 检测 → 重命名为 `.mov` | ✅ 继续处理 |
| Not a valid JPG (PNG) | 6 | `file` 检测 → 重命名为 `.png` | ✅ 继续处理 |
| Not a valid JPG (RIFF) | 4 | `file` 检测 → 重命名为 `.webp` | ✅ 继续处理 |
| IFD0 pointer 循环引用 | 4 | `-ignoreMinorErrors` | ✅ 继续处理 |
| garbage at end (MP4) | 4 | `-ignoreMinorErrors` | ✅ 继续处理 |
| WMV 不支持写入 | 1 | 移到 error | ✅ 保留 |
| Truncated mdat (MOV) | 1 | 移到 error | ✅ 保留 |
| 重命名后冲突 | 未知 | 移到 error | ✅ 保留 |

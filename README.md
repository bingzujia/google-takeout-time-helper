# Google Takeout Time Helper

> **现已提供跨平台 Go 二进制 `takeout-helper`**，无需 WSL、无需 Bash，Windows / macOS / Linux 均可直接运行。
> 原 Shell 脚本已归档至 `legacy/` 目录，供历史参考。

---

## 安装

### 方式一：下载预编译二进制（推荐）

前往 [Releases](https://github.com/bingzujia/google-takeout-time-helper/releases) 页面下载对应平台的文件：

| 平台 | 文件名 |
|------|--------|
| Windows (x64) | `takeout-helper-windows-amd64.exe` |
| macOS (Intel) | `takeout-helper-darwin-amd64` |
| macOS (Apple Silicon) | `takeout-helper-darwin-arm64` |
| Linux (x64) | `takeout-helper-linux-amd64` |

下载后赋予执行权限（macOS / Linux）：

```bash
chmod +x takeout-helper-darwin-arm64
# 可选：移入 PATH
mv takeout-helper-darwin-arm64 /usr/local/bin/takeout-helper
```

> 默认发布的二进制可直接使用 `migrate` / `classify` / `fix-exif` / `fix-name` / `dedup` / `rename` / `rename-screenshot` / `format-qq-media`。  
> 若要使用 `convert`，需在系统中安装 **`heif-enc`** 与 **`exiftool`**。

### 方式二：从源码编译

```bash
git clone https://github.com/bingzujia/google-takeout-time-helper.git
cd google-takeout-time-helper
make build          # 产物：bin/takeout-helper
```

### 可选：启用 HEIC 转换能力

`takeout-helper convert` 使用系统安装的 **`heif-enc`** 作为 HEIC 编码后端，默认 quality 35（0–100 scale，有损）。超过 4000 万像素的大图会串行处理，以降低内存峰值。

安装依赖（Debian / Ubuntu 示例）：

```bash
sudo apt-get install -y libheif-examples libimage-exiftool-perl
```

macOS：

```bash
brew install libheif exiftool
```

验证 heif-enc 是否可用：

```bash
heif-enc --version
```

---

## 命令

```
takeout-helper migrate           --input-dir <dir> --output-dir <dir>   # 迁移 Google Takeout 照片
takeout-helper classify          --input-dir <dir> --output-dir <dir>   # 按类型分类媒体文件
takeout-helper convert           --input-dir <dir>                      # 将根目录图片原地转换为 HEIC
takeout-helper fix-exif          --input-dir <dir>                      # 同步 DateTimeOriginal → CreateDate & ModifyDate
takeout-helper fix-name          --input-dir <dir>                      # 同步文件名时间戳 → DateTimeOriginal, CreateDate & ModifyDate
takeout-helper dedup             --input-dir <dir>                      # 检测并整理重复图片
takeout-helper rename            --input-dir <dir>                      # 批量重命名照片/视频文件
takeout-helper rename-screenshot --input-dir <dir>                      # 批量重命名截图文件
takeout-helper format-qq-media   --input-dir <dir>                      # 按时间戳格式化 QQ 导出的媒体文件
```

`takeout-helper` 专注于修复 Google Takeout 导出照片的时间戳，并提供分类整理工具。各命令均支持 `--dry-run` 预览模式，不会实际修改文件（`fix-exif` 使用 `--dry-run`）。所有写入 EXIF 元数据的命令均需安装 `exiftool`。

---

## 命令详解

### `takeout-helper migrate` — 迁移 Google Takeout 照片

**用途**：扫描 Google Takeout 的年文件夹（`Photos from XXXX`），将照片拷贝到输出目录，通过 `exiftool` 从 **JSON 元数据文件**写入 `CreateDate` 和 `ModifyDate`，并在 EXIF 中缺少 GPS 时补充 JSON 中的 GPS 坐标。无 JSON 元数据的文件直接拷贝，不写 EXIF。生成 SHA-256 校验的元数据 JSON 文件，日志写入 `takeout-helper-log/migrate-{date}-{index}.log`（在 `--output-dir` 根目录下）。

> **注意**：`migrate` 只写 `CreateDate` / `ModifyDate`，不写 `DateTimeOriginal`。如需同步 `DateTimeOriginal`，请在 `migrate` 后运行 `fix-exif` 或 `fix-name`。

**典型 Google Takeout 目录结构**：

```
Takeout/
└── Google Photos/
    ├── Photos from 2023/
    │   ├── IMG_20230512_143022.jpg
    │   ├── IMG_20230512_143022.jpg.json   ← 包含拍摄时间与 GPS
    │   └── ...
    └── Photos from 2024/
        └── ...
```

**用法**：

```bash
takeout-helper migrate --input-dir "/path/to/Takeout/Google Photos" --output-dir "/path/to/output"
takeout-helper migrate --input-dir "/path/to/Takeout/Google Photos" --output-dir "/path/to/output" --dry-run
```

**预期输出**：

```
Input:  /path/to/Takeout/Google Photos
Output: /path/to/output

Scanning files...
Found 200 files in 2 year folder(s)

🔄 [████████████████████████░░░░░░░░░░░░░░░░] 60% (120/200)

Processing complete!
  Scanned:            200 files
  Processed:          195 files
  Skipped (exists):   1 files
  Failed (exiftool):  1 files
  Failed (other):     0 files
  Manual review:      3 files
  Log:                /path/to/output/takeout-helper-log/migrate-20240115-001.log
```

**时间戳来源**：JSON 元数据文件（`photoTakenTime.timestamp`）

**GPS 来源优先级**：

1. EXIF GPS 坐标（保留已有的，不覆盖）
2. JSON 元数据文件中的 GPS 坐标（仅在 EXIF 缺少时补充）

---

### `takeout-helper classify` — 按类型分类媒体文件

**用途**：扫描 `--input-dir` **根目录下的媒体文件**，根据文件名规则或 EXIF 设备信息，将文件移动到 `--output-dir` 的对应子目录中。

| 目标目录 | 规则 |
|----------|------|
| `camera/` | 文件名匹配相机模式（`IMG_`、`VID_`、`PXL_` 等） |
| `screenshot/` | 文件名包含 `screenshot` |
| `wechat/` | 文件名以 `mmexport` 开头 |
| `seemsCamera/` | 无文件名匹配，但 `exiftool` 检测到 EXIF Make/Model |

不匹配任何规则的文件原地保留，计入 Skipped。

> `classify` 只处理 `--input-dir` 根目录中的常规文件；子目录及其内部文件会被忽略。

**用法**：

```bash
takeout-helper classify --input-dir "/path/to/input" --output-dir "/path/to/output"
takeout-helper classify --input-dir "/path/to/input" --output-dir "/path/to/output" --dry-run
```

**预期输出**：

```
Input:  /path/to/input
Output: /path/to/output

Classification complete!
  Camera:       42 files
  Screenshot:   15 files
  WeChat:        8 files
  SeemsCamera:   3 files
  Skipped:       7 files
```

---

### `takeout-helper convert` — 将根目录图片原地转换为 HEIC

**用途**：扫描 `--input-dir` **根目录下的常规文件**，识别其中可解码的非 HEIC 图片，通过 **`heif-enc`**（quality 35，有损）原地转换为 `.heic`。成功后迁移原图 EXIF 元数据到新文件，再删除原文件；若目标 `.heic` 已存在则跳过；若扩展名与真实类型不符会先纠正，再转为 `.heic`；超过 **4000 万像素**的大图会串行处理以降低内存峰值。

> `convert` 只处理 `--input-dir` 根目录中的常规文件；子目录及其内部文件会被忽略。
>
> 需要系统已安装 `heif-enc`（`libheif-examples`）和 `exiftool`。

**用法**：

```bash
takeout-helper convert --input-dir "/path/to/input"
takeout-helper convert --input-dir "/path/to/input" --dry-run
takeout-helper convert --input-dir "/path/to/input" --workers 1   # 降低并发以进一步节省内存
```

**参数说明**：

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `--dry-run` | false | 仅预览，不修改文件 |
| `--workers` | 2 | 并发转换 worker 数；降低此值可减少内存压力 |

**预期输出**：

```
Input:   /path/to/input
Workers: 2

🔄 [++++++++++++++++++++] 100% (12/12)

HEIC conversion complete!
  Root files scanned:     12
  Converted:               9
  Extension corrected:     2
  Skipped (conflict):      1
  Skipped (already HEIC):  1
  Skipped (unsupported):   0
  Failed:                  1
```

---

### `takeout-helper fix-exif` — 同步 EXIF 日期字段

**用途**：读取目录下媒体文件的 `DateTimeOriginal` 字段，将相同的值写入 `CreateDate` 和 `ModifyDate`（通过 `exiftool` 并发处理，非递归，仅处理第一级文件）。无法解析 EXIF 时自动回落到文件名中的时间戳。处理失败时记录到 `takeout-helper-log/fix-exif-{date}-{index}.log`（在 `--input-dir` 根目录下）。

支持格式：`jpg`、`jpeg`、`png`、`heic`、`heif`、`mp4`、`mov`、`avi`、`3gp`、`mkv`、`webp`。

**用法**：

```bash
takeout-helper fix-exif --input-dir "/path/to/photos"
takeout-helper fix-exif --input-dir "/path/to/photos" --dry-run
```

**预期输出**：

```
Done. Processed: 38, Failed: 0, Skipped: 2
```

---

### `takeout-helper fix-name` — 从文件名同步时间戳

**用途**：解析媒体文件名中的日期时间，与 EXIF `DateTimeOriginal` 对比，仅当文件名时间早于 EXIF 时间（或 EXIF 中无时间戳）时写入 `DateTimeOriginal`、`CreateDate` 和 `ModifyDate`（通过 `exiftool` 并发处理，非递归，仅处理第一级文件）。文件名中没有可解析时间的文件自动跳过。处理失败时记录到 `takeout-helper-log/fix-name-{date}-{index}.log`（在 `--input-dir` 根目录下）。

支持格式：`jpg`、`jpeg`、`png`、`heic`、`heif`、`mp4`、`mov`、`avi`、`3gp`、`mkv`、`webp`。

**用法**：

```bash
takeout-helper fix-name --input-dir "/path/to/photos"
takeout-helper fix-name --input-dir "/path/to/photos" --dry-run
```

**预期输出**：

```
Done. Processed: 24, Failed: 0, Skipped: 5
```

---

### `takeout-helper dedup` — 检测并整理重复图片

**用途**：扫描 `--input-dir` 指定目录下的**一级**图片文件（非递归），通过感知哈希（pHash + dHash 双重校验）检测重复，将每个重复批次移动到 `<input_dir>/dedup/group-001/`、`group-002/` … 等子目录，方便人工审查或删除。

支持格式：`jpg`、`jpeg`、`png`、`gif`、`bmp`、`tiff`、`tif`、`webp`、`heic`、`heif`。

**用法**：

```bash
takeout-helper dedup --input-dir "/path/to/photos"
takeout-helper dedup --input-dir "/path/to/photos" --dry-run
takeout-helper dedup --input-dir "/path/to/photos" --threshold 5          # 更严格的相似度（默认 10）
takeout-helper dedup --input-dir "/path/to/photos" --decode-workers 2     # 限制并发解码数，降低内存峰值
takeout-helper dedup --input-dir "/path/to/photos" --max-decode-mb 200    # 跳过超过 200 MB 的图片
```

**预期输出**：

```
Input:     /path/to/photos
Threshold: 10
Mode:      dry-run (no files will be moved)

[group-001] 3 duplicate file(s):
  /path/to/photos/a.jpg → /path/to/photos/dedup/group-001/a.jpg
  /path/to/photos/b.jpg → /path/to/photos/dedup/group-001/b.jpg
  /path/to/photos/c.jpg → /path/to/photos/dedup/group-001/c.jpg

[group-002] 2 duplicate file(s):
  /path/to/photos/d.jpg → /path/to/photos/dedup/group-002/d.jpg
  /path/to/photos/f.jpg → /path/to/photos/dedup/group-002/f.jpg

Dry-run complete! (no files were moved)
  Images scanned:   6
  Duplicate groups: 2
  Would move:       5 file(s)
```

**参数说明**：

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `--dry-run` | false | 仅预览，不移动文件 |
| `--threshold` | 10 | 哈希距离阈值，越小越严格（pHash 和 dHash 均须 ≤ 阈值才判定为重复） |
| `--no-cache` | false | 禁用哈希缓存，每次都从磁盘重新计算 |
| `--cache-dir` | `<input_dir>/.takeout-helper_cache` | 哈希缓存 DB 目录 |
| `--max-decode-mb` | 500 | 跳过文件体积超过此值（MB）的图片，防止 OOM |
| `--decode-workers` | 0（不限制） | 同时解码图片的最大并发数；设为较小值（如 2）可降低内存峰值 |

---

### `takeout-helper rename` — 批量重命名照片文件

**用途**：扫描 `--input-dir` 指定目录下的**一级**照片/视频文件（非递归），按文件修改时间自动生成标准化名称。

支持格式：`jpg`、`jpeg`、`png`、`gif`、`bmp`、`tiff`、`tif`、`heic`、`heif`、`webp`、`avif`、`raw`、`cr2`、`nef`、`arw`、`dng`、`mp4`、`mov`、`avi`、`mkv`、`wmv`、`flv`、`3gp`、`m4v`、`webm` 等。

**命名规则**：

| 文件类型 | 目标格式 | 示例 |
|---------|---------|------|
| HEIC/HEIF 图片 | `IMG{YYYYMMDD}{HHMMSS}.{ext}` | `IMG20230123104707.heic` |
| 其他图片 | `IMG_{YYYYMMDD}_{HHMMSS}.{ext}` | `IMG_20190403_165110.jpg` |
| 独立视频 | `VID{YYYYMMDD}{HHMMSS}.{ext}` | `VID20190403165110.mp4` |
| 连拍 HEIC | `IMG{YYYYMMDD}{HHMMSS}_BURST{NNN}.{ext}` | `IMG20190207184125_BURST000.heic` |
| 连拍其他 | `IMG_{YYYYMMDD}_{HHMMSS}_BURST{NNN}.{ext}` | `IMG_20190207_184125_BURST000.jpg` |

**连拍检测**：文件名匹配 `YYYYMMDD_HHMMSS_NNN.ext` 且同前缀存在 ≥2 个文件时触发，按原序号升序重编索引（从 `000` 起）。单独存在的同模式文件按普通规则处理。

**MP4 伴侣**：与图片同名（仅扩展名不同）的 `.mp4` 文件，随主图一起重命名，格式与主图一致（扩展名替换为 `.mp4`）。

**冲突处理**：目标文件名已存在时自动追加 `_001`、`_002` … 后缀。

**用法**：

```bash
takeout-helper rename --input-dir ./Photos            # 重命名
takeout-helper rename --input-dir ./Photos --dry-run  # 仅预览
```

**预期输出**：

```
  shot.heic -> IMG20230123104707.heic
  photo.jpg -> IMG_20190403_165110.jpg
Renamed: 42, Skipped: 3, Errors: 0
```

**参数说明**：

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `--input-dir` | （必填） | 目标目录 |
| `--dry-run` | false | 仅预览，不实际修改 |

---

### `takeout-helper rename-screenshot` — 批量重命名截图文件

**用途**：扫描 `--input-dir` 指定目录下的**一级**截图文件（非递归），识别各类截图文件名格式，解析其中的时间戳，统一转换为标准化名称 `Screenshot_YYYY-MM-DD-HH-MM-SS-MS.{ext}`（完整到毫秒精度）。

支持格式：`png`、`jpg`、`jpeg`、`gif`、`bmp`、`tiff`、`tif`、`webp`、`heic`、`heif`。

**支持的截图时间戳格式**：

| 格式 | 例子 | 精度 |
|------|------|------|
| YYYY-MM-DD-HH-MM-SS-MS | `Screenshot_2025-07-18-09-23-54-65.png` | 毫秒 |
| YYYYMMDD_HHMMSS | `screenshot20250718_092354.jpg` | 秒 |
| YYYY-MM-DD_HH-MM-SS | `Screenshot_2025-07-18_09-23-54.png` | 秒 |
| YYYY_M_D_H_M_S（无零补齐） | `screenshot_2025_7_18_9_23_54.png` | 秒 |
| YYYY-MM-DD（仅日期） | `screenshot_2025-07-18.jpg` | 日（时间设为 00:00:00） |
| Unix 时间戳 10 位（秒） | `screenshot1634560000.jpg` | 秒 |
| Unix 时间戳 13 位（毫秒） | `mmscreenshot1727421404387.jpg` | 毫秒 |

**文件检测**：采用宽松的 substring 匹配，检测文件名中是否包含 `"screenshot"`（大小写不敏感）。此策略覆盖各类截图软件生成的格式（`Screenshot_*`、`screenshot_*`、`mmscreenshot*`、`wxscreenshot*` 等）。

**冲突处理**：目标文件名已存在时自动追加 `_001`、`_002` … 后缀。

**无扩展名文件**：保留原有文件名，不强制添加默认扩展名。

**用法**：

```bash
takeout-helper rename-screenshot --input-dir ./Screenshots            # 重命名
takeout-helper rename-screenshot --input-dir ./Screenshots --dry-run  # 仅预览
```

**预期输出**：

```
Input:  /path/to/Screenshots

🔄 [████████████████████████░░░░░░░░░░░░░░░░] 60% (15/25)

Renamed: 20, Skipped: 3, Errors: 0
Log: /path/to/Screenshots/takeout-helper-log/rename-screenshot-20250419-001.log
```

**参数说明**：

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `--input-dir` | （必填） | 目标目录 |
| `--dry-run` | false | 仅预览，不实际修改 |

---

### `takeout-helper format-qq-media` — 格式化 QQ 导出的媒体文件

**用途**：扫描 `--input-dir` 指定目录下的 QQ 导出媒体文件，识别文件内容类型（图片/视频），解析文件名中的时间戳（支持 7 种格式），统一转换为标准化名称 `Image_<unix-ms>.{ext}` 或 `Video_<unix-ms>.{ext}`。

支持图片格式：`jpg`、`jpeg`、`png`、`gif`、`bmp`、`tiff`、`tif`、`webp`、`heic`、`heif`。
支持视频格式：`mp4`、`mov`、`avi`、`mkv`、`webm`、`flv`。

**支持的 QQ 时间戳格式**：

| 模式 | 例子 | 精度 |
|------|------|------|
| `_YYYYMMDD_HHMMSS` | `photo_20170709_002844.jpg` | 秒 |
| 13 位 Unix 毫秒 | `photo_1688017744459.jpg` | 毫秒 |
| `QQ视频YYYYMMDDHHMMSS` | `QQ视频20150720105516.mp4` | 秒 |
| `Record_YYYY-MM-DD-HH-MM-SS` | `Record_2024-12-19-16-07-17.mp4` | 秒 |
| `Snipaste_YYYY-MM-DD_HH-MM-SS` | `Snipaste_2018-09-17_18-07-29.png` | 秒 |
| `tb_image_share_13digits` | `tb_image_share_1661951220361.jpg` | 毫秒 |
| `TIM图片YYYYMMDDHHMMSS` | `TIM图片20181215191143.jpg` | 秒 |
| 文件修改时间（备用） | *无匹配格式时使用* | 秒 |

**文件类型检测**：采用二级策略
1. **首选**：读取文件头（magic bytes）使用 `http.DetectContentType()` 检测真实 MIME 类型
2. **备用**：扩展名匹配（速度更快，重复处理同类文件时命中）

**冲突处理**：目标文件名已存在时自动追加 `_001`、`_002` … 后缀（保存在扩展名前）。

**用法**：

```bash
takeout-helper format-qq-media --input-dir ./QQ_exports            # 格式化
takeout-helper format-qq-media --input-dir ./QQ_exports --dry-run  # 仅预览
```

**预期输出**：

```
Input:  /path/to/QQ_exports

🔄 [████████████████████████░░░░░░░░░░░░░░░░] 60% (15/25)

Renamed: 20, Skipped: 3, Errors: 0
Log: /path/to/QQ_exports/takeout-helper-log/format-qq-media-20250419-001.log
```

**参数说明**：

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `--input-dir` | （必填） | 目标目录 |
| `--dry-run` | false | 仅预览，不实际修改 |

---

## 日志

所有命令均将结构化日志写入 `takeout-helper-log/` 子目录，按日期和递增编号命名：

```
takeout-helper-log/{command}-{YYYYMMDD}-{NNN}.log
```

| 命令 | 日志目录 | 示例路径 |
|------|---------|---------|
| `migrate` | `--output-dir` 根目录 | `output/takeout-helper-log/migrate-20240115-001.log` |
| `classify` | `--output-dir` 根目录 | `sorted/takeout-helper-log/classify-20240115-001.log` |
| `fix-exif` | `--input-dir` 根目录 | `photos/takeout-helper-log/fix-exif-20240115-001.log` |
| `fix-name` | `--input-dir` 根目录 | `photos/takeout-helper-log/fix-name-20240115-001.log` |
| `convert` | `--input-dir` 根目录 | `photos/takeout-helper-log/convert-20240115-001.log` |
| `dedup` | `--input-dir` 根目录 | `photos/takeout-helper-log/dedup-20240115-001.log` |
| `rename` | `--input-dir` 根目录 | `photos/takeout-helper-log/rename-20240115-001.log` |
| `rename-screenshot` | `--input-dir` 根目录 | `screenshots/takeout-helper-log/rename-screenshot-20250419-001.log` |
| `format-qq-media` | `--input-dir` 根目录 | `qq_exports/takeout-helper-log/format-qq-media-20250419-001.log` |

- 同一天多次运行时，编号自动递增（`-001`、`-002`、…）
- `--dry-run` 模式下不产生日志文件
- 日志路径会在命令完成后的摘要中显示

---

## 推荐工作流

处理一份新的 Google Takeout 导出：

```bash
# 1. 迁移照片（从 JSON 写入 CreateDate / ModifyDate + 拷贝到干净的输出目录）
takeout-helper migrate --input-dir "Takeout/Google Photos" --output-dir "output"

# 2. （可选）同步 DateTimeOriginal → CreateDate & ModifyDate
takeout-helper fix-exif --input-dir "output"

# 3. （可选）对没有 DateTimeOriginal 的文件，从文件名补充时间戳
takeout-helper fix-name --input-dir "output"

# 4. （可选）先将根目录图片原地转换为 HEIC
takeout-helper convert --input-dir "output" --dry-run   # 先预览
takeout-helper convert --input-dir "output"             # 需安装 heif-enc 与 exiftool

# 5. （可选）按类型整理分类
takeout-helper classify --input-dir "output" --output-dir "sorted"

# 6. （可选）检测并整理重复图片
takeout-helper dedup --input-dir "output" --dry-run   # 先预览
takeout-helper dedup --input-dir "output"             # 确认后执行

# 7. （可选）批量重命名照片
takeout-helper rename --input-dir "output" --dry-run   # 先预览
takeout-helper rename --input-dir "output"

# 8. （可选）批量重命名截图（已分类出的或单独目录）
takeout-helper rename-screenshot --input-dir "sorted/screenshot" --dry-run  # 先预览
takeout-helper rename-screenshot --input-dir "sorted/screenshot"

# 9. （可选）格式化 QQ 导出的媒体文件（另一数据源）
takeout-helper format-qq-media --input-dir "qq_exports" --dry-run  # 先预览
takeout-helper format-qq-media --input-dir "qq_exports"
```

---

## 注意事项

- **备份优先**：建议在执行前对原始文件进行备份
- **exiftool**：安装 `exiftool` 后可写入 EXIF 元数据（`CreateDate`、`ModifyDate`、`DateTimeOriginal`）和 GPS 坐标；未安装时仅拷贝文件，不写入 EXIF
- **heif-enc（`convert` 必需）**：`convert` 依赖系统安装的 `heif-enc`（`libheif-examples`）；缺少时命令会在启动时给出明确错误提示
- **convert 行为**：仅处理输入目录第一级常规文件；遇到已存在的目标 `.heic` 会跳过，不会覆盖；超过 4000 万像素的大图会串行处理以降低内存峰值
- **convert 内存调优**：默认使用 2 个并发 worker；若仍遇到内存压力，可通过 `--workers 1` 进一步降低并发
- **dedup 内存调优**：默认跳过 >500 MB 的图片（`--max-decode-mb`）；大图库建议通过 `--decode-workers 2` 限制并发解码数，避免多个大图同时在内存中展开
- **dedup 哈希缓存**：默认将 pHash/dHash 缓存于 `<input_dir>/.takeout-helper_cache/`，二次运行无需重新解码；使用 `--no-cache` 强制重算
- **Windows**：直接运行 `.exe`，无需 WSL 或 Bash 环境

---

## 历史参考：原 Shell 脚本

| Shell 脚本 | 功能 |
|------------|------|
| `legacy/fix_takeout_photo_time_wsl.sh` | 修复 Google Takeout 时间戳 |
| `legacy/fix_img_timestamps.sh` | 修复 IMG/VID 文件名时间戳 |
| `legacy/organize_photos.sh` / `legacy/organize_screenshots.sh` / `legacy/organize_wechat.sh` | 按类型整理照片 |
| `legacy/rename_photos.sh` | 按时间戳重命名 |
| `legacy/delete_json_files.sh` | 删除 JSON 附属文件 |

原脚本仅支持 WSL / Linux，新 `takeout-helper` 二进制全平台可用。

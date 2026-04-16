# g_photo_take_out_helper

> **现已提供跨平台 Go 二进制 `gtoh`**，无需 WSL、无需 Bash，Windows / macOS / Linux 均可直接运行。
> 原 Shell 脚本保留于仓库根目录，供历史参考。

---

## 安装

### 方式一：下载预编译二进制（推荐）

前往 [Releases](https://github.com/bingzujia/g_photo_take_out_helper/releases) 页面下载对应平台的文件：

| 平台 | 文件名 |
|------|--------|
| Windows (x64) | `gtoh-windows-amd64.exe` |
| macOS (Intel) | `gtoh-darwin-amd64` |
| macOS (Apple Silicon) | `gtoh-darwin-arm64` |
| Linux (x64) | `gtoh-linux-amd64` |

下载后赋予执行权限（macOS / Linux）：

```bash
chmod +x gtoh-darwin-arm64
# 可选：移入 PATH
mv gtoh-darwin-arm64 /usr/local/bin/gtoh
```

> 默认发布的二进制可直接使用 `migrate` / `classify` / `fix-exif-dates` / `dedup`。  
> 若要使用 `to-heic`，需在系统中安装 **`ffmpeg`**（含 libx265 和 HEIF/HEIC 容器支持）与 **`exiftool`**。

### 方式二：从源码编译

```bash
git clone https://github.com/bingzujia/g_photo_take_out_helper.git
cd g_photo_take_out_helper
make build          # 产物：bin/gtoh
```

### 可选：启用 HEIC 转换能力

`gtoh to-heic` 使用系统安装的 **`ffmpeg`** 作为 HEIC 编码后端，默认采用 CRF 21（≈ 有损质量 80，`medium` 预设）。超过 4000 万像素的大图会以更严格的参数（`-threads 1`、`-pix_fmt yuv420p`）串行处理，以降低内存峰值。

安装依赖（Debian / Ubuntu 示例）：

```bash
sudo apt-get install -y ffmpeg libimage-exiftool-perl
```

macOS：

```bash
brew install ffmpeg exiftool
```

验证 ffmpeg 是否支持 HEIC（需含 `libx265` 编码器与 `heif` 封装器）：

```bash
ffmpeg -encoders 2>/dev/null | grep libx265
ffmpeg -formats  2>/dev/null | grep heif
```

---

## 命令

```
gtoh migrate      <input_dir> <output_dir>   # 迁移 Google Takeout 照片
gtoh classify     <input_dir> <output_dir>   # 按类型分类媒体文件
gtoh to-heic      <input_dir>                # 将根目录图片原地转换为 HEIC
gtoh fix-exif-dates --dir <dir>              # 同步 DateTimeOriginal → CreateDate & ModifyDate
gtoh dedup        <input_dir>                # 检测并整理重复图片
```

`gtoh` 专注于修复 Google Takeout 导出照片的时间戳，并提供分类整理工具。各命令均支持 `--dry-run` 预览模式，不会实际修改文件（`fix-exif-dates` 使用 `--dry-run`）。所有写入 EXIF 元数据的命令均需安装 `exiftool`。

---

## 命令详解

### `gtoh migrate` — 迁移 Google Takeout 照片

**用途**：扫描 Google Takeout 的年文件夹（`Photos from XXXX`），从 EXIF / 文件名 / JSON 元数据中提取时间戳和 GPS 坐标，将照片拷贝到输出目录，通过 `exiftool` 写入 EXIF 元数据，并生成 SHA-256 校验的元数据 JSON 文件。

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
gtoh migrate "/path/to/Takeout/Google Photos" "/path/to/output"
gtoh migrate "/path/to/Takeout/Google Photos" "/path/to/output" --dry-run
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
  Skipped (no time):  3 files
  Skipped (exists):   1 files
  Failed (exiftool):  1 files
  Failed (other):     0 files
  Log:                /path/to/output/gtoh.log
```

**时间戳来源优先级**：

1. EXIF `DateTimeOriginal`（通过 `exiftool` 提取）
2. 文件名中的时间信息（如 `IMG_20230512_143022.jpg`）
3. JSON 元数据文件中的时间

**GPS 来源优先级**：

1. EXIF GPS 坐标（通过 `exiftool` 提取）
2. JSON 元数据文件中的 GPS 坐标

---

### `gtoh classify` — 按类型分类媒体文件

**用途**：扫描 `input_dir` **根目录下的媒体文件**，根据文件名规则或 EXIF 设备信息，将文件移动到 `output_dir` 的对应子目录中。

| 目标目录 | 规则 |
|----------|------|
| `camera/` | 文件名匹配相机模式（`IMG_`、`VID_`、`PXL_` 等） |
| `screenshot/` | 文件名包含 `screenshot` |
| `wechat/` | 文件名以 `mmexport` 开头 |
| `seemsCamera/` | 无文件名匹配，但 `exiftool` 检测到 EXIF Make/Model |

不匹配任何规则的文件原地保留，计入 Skipped。

> `classify` 只处理 `input_dir` 根目录中的常规文件；子目录及其内部文件会被忽略。

**用法**：

```bash
gtoh classify "/path/to/input" "/path/to/output"
gtoh classify "/path/to/input" "/path/to/output" --dry-run
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

### `gtoh to-heic` — 将根目录图片原地转换为 HEIC

**用途**：扫描 `input_dir` **根目录下的常规文件**，识别其中可解码的非 HEIC 图片，通过 **`ffmpeg`**（libx265，CRF 21，`medium` 预设，有损质量约 80）原地转换为 `.heic`。成功后迁移原图 EXIF 元数据（优先使用 FFmpeg 元数据映射）到新文件，再删除原文件；若目标 `.heic` 已存在则跳过；若扩展名与真实类型不符会先纠正，再转为 `.heic`；超过 **4000 万像素**的大图会串行处理，并强制 `-threads 1` 与 `-pix_fmt yuv420p` 以降低内存峰值。

> `to-heic` 只处理 `input_dir` 根目录中的常规文件；子目录及其内部文件会被忽略。
>
> 需要系统已安装 `ffmpeg`（含 libx265 与 HEIF/HEIC 封装支持）和 `exiftool`。

**用法**：

```bash
gtoh to-heic "/path/to/input"
gtoh to-heic "/path/to/input" --dry-run
gtoh to-heic "/path/to/input" --workers 1   # 降低并发以进一步节省内存
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
  Root files scanned:   12
  Converted:            9
  Extension corrected:  2
  Skipped (conflict):   1
  Skipped (already HEIC): 1
  Skipped (unsupported): 0
  Failed:               1
```

---

### `gtoh fix-exif-dates` — 同步 EXIF 日期字段

**用途**：读取目录下媒体文件的 `DateTimeOriginal` 字段，将相同的值写入 `CreateDate` 和 `ModifyDate`（通过 `exiftool` 批量处理，非递归，仅处理第一级文件）。

**用法**：

```bash
gtoh fix-exif-dates --dir "/path/to/photos"
gtoh fix-exif-dates --dir "/path/to/photos" --dry-run
```

**预期输出**：

```
Done. Processed: 38, Skipped: 2
```

---

### `gtoh dedup` — 检测并整理重复图片

**用途**：扫描 `<input_dir>` 下的**一级**图片文件（非递归），通过感知哈希（pHash + dHash 双重校验）检测重复，将每个重复批次移动到 `<input_dir>/dedup/group-001/`、`group-002/` … 等子目录，方便人工审查或删除。

支持格式：`jpg`、`jpeg`、`png`、`gif`、`bmp`、`tiff`、`tif`、`webp`、`heic`、`heif`。

**用法**：

```bash
gtoh dedup "/path/to/photos"
gtoh dedup "/path/to/photos" --dry-run
gtoh dedup "/path/to/photos" --threshold 5   # 更严格的相似度（默认 10）
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

---

## 推荐工作流

处理一份新的 Google Takeout 导出：

```bash
# 1. 迁移照片（修复时间戳 + 拷贝到干净的输出目录）
gtoh migrate "Takeout/Google Photos" "output"

# 2. （可选）补充同步 CreateDate / ModifyDate
gtoh fix-exif-dates --dir "output"

# 3. （可选）先将根目录图片原地转换为 HEIC
gtoh to-heic "output" --dry-run   # 先预览
gtoh to-heic "output"             # 需安装 ffmpeg（含 libx265 + HEIF 支持）

# 4. （可选）按类型整理分类
gtoh classify "output" "sorted"

# 5. （可选）检测并整理重复图片
gtoh dedup "output" --dry-run   # 先预览
gtoh dedup "output"             # 确认后执行
```

---

## 注意事项

- **备份优先**：建议在执行前对原始文件进行备份
- **exiftool**：安装 `exiftool` 后可写入 EXIF 元数据（`DateTimeOriginal`）和 GPS 坐标；未安装时仅拷贝文件，不写入 EXIF
- **ffmpeg（`to-heic` 必需）**：`to-heic` 依赖系统安装的 `ffmpeg`（需含 libx265 编码器与 HEIF/HEIC 封装支持）；缺少时命令会在启动时给出明确错误提示
- **to-heic 行为**：仅处理输入目录第一级常规文件；遇到已存在的目标 `.heic` 会跳过，不会覆盖；超过 4000 万像素的大图会串行处理以降低内存峰值
- **to-heic 内存调优**：默认使用 2 个并发 worker；若仍遇到内存压力，可通过 `--workers 1` 进一步降低并发
- **Windows**：直接运行 `.exe`，无需 WSL 或 Bash 环境

---

## 历史参考：原 Shell 脚本

| Shell 脚本 | 功能 |
|------------|------|
| `fix_takeout_photo_time_wsl.sh` | 修复 Google Takeout 时间戳 |
| `fix_img_timestamps.sh` | 修复 IMG/VID 文件名时间戳 |
| `organize_photos.sh` / `organize_screenshots.sh` / `organize_wechat.sh` | 按类型整理照片 |
| `rename_photos.sh` | 按时间戳重命名 |
| `delete_json_files.sh` | 删除 JSON 附属文件 |

原脚本仅支持 WSL / Linux，新 `gtoh` 二进制全平台可用。

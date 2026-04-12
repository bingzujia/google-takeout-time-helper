# g_photo_take_out_helper 二次开发手册

## 目录

1. [项目概述](#1-项目概述)
2. [目录结构](#2-目录结构)
3. [环境要求与依赖](#3-环境要求与依赖)
4. [脚本执行流程总览](#4-脚本执行流程总览)
5. [核心模块详解](#5-核心模块详解)
   - 5.1 [fix_takeout_photo_time_wsl.sh — Google Takeout 时间戳修复](#51-fix_takeout_photo_time_wslsh--google-takeout-时间戳修复)
   - 5.2 [rename_photos.sh — 照片统一重命名](#52-rename_photossh--照片统一重命名)
   - 5.3 [organize_photos.sh — 相机照片整理](#53-organize_photossh--相机照片整理)
   - 5.4 [organize_screenshots.sh — 截图整理](#54-organize_screenshotssh--截图整理)
   - 5.5 [organize_wechat.sh — 微信媒体整理](#55-organize_wechatsh--微信媒体整理)
   - 5.6 [fix_img_timestamps.sh — IMG/VID 时间戳校正](#56-fix_img_timestampssh--imgvid-时间戳校正)
   - 5.7 [delete_json_files.sh — JSON 文件清理](#57-delete_json_filessh--json-文件清理)
6. [通用设计模式](#6-通用设计模式)
7. [关键函数速查表](#7-关键函数速查表)
8. [扩展开发指南](#8-扩展开发指南)
   - 8.1 [新增支持的文件格式](#81-新增支持的文件格式)
   - 8.2 [新增文件名解析模式](#82-新增文件名解析模式)
   - 8.3 [新增整理分类脚本](#83-新增整理分类脚本)
   - 8.4 [新增命令行参数](#84-新增命令行参数)
9. [数据流与变量约定](#9-数据流与变量约定)
10. [错误处理规范](#10-错误处理规范)
11. [调试与验证](#11-调试与验证)
12. [常见问题](#12-常见问题)

---

## 1. 项目概述

`g_photo_take_out_helper` 是一套专为 **Google Takeout 照片归档** 设计的 Bash 脚本工具集，主要解决 Google Takeout 导出后照片丢失原始拍摄时间戳的问题，并提供按来源分类整理照片的功能。

**核心功能**

| 功能 | 脚本 |
|------|------|
| 从 JSON 元数据恢复照片时间戳 | `fix_takeout_photo_time_wsl.sh` |
| 按文件名校正 IMG/VID 文件时间戳 | `fix_img_timestamps.sh` |
| 将所有照片重命名为统一的时间戳格式 | `rename_photos.sh` |
| 将相机照片归类到 `camera/` 目录 | `organize_photos.sh` |
| 将截图归类到 `screenshot/` 目录 | `organize_screenshots.sh` |
| 将微信媒体归类到 `wechat/` 目录 | `organize_wechat.sh` |
| 删除所有 JSON 元数据文件 | `delete_json_files.sh` |

**许可证**：Apache License 2.0

---

## 2. 目录结构

```
g_photo_take_out_helper/
├── README.md                          # 项目简介
├── LICENSE                            # Apache License 2.0
├── DEVELOPMENT_GUIDE.md               # 本手册
├── fix_takeout_photo_time_wsl.sh      # ★ 核心：从 JSON 恢复时间戳
├── fix_img_timestamps.sh              # ★ 核心：从文件名校正时间戳
├── rename_photos.sh                   # 统一重命名为 IMG/VID 格式
├── organize_photos.sh                 # 整理相机照片 → camera/
├── organize_screenshots.sh            # 整理截图 → screenshot/
├── organize_wechat.sh                 # 整理微信媒体 → wechat/
└── delete_json_files.sh               # 删除 JSON 元数据文件
```

**典型使用场景下的目录结构（运行前）**

```
takeout-root/
├── Photos from 2023/
│   ├── IMG_20230302_112040.jpg
│   ├── IMG_20230302_112040.jpg.json
│   ├── mmexport1491013330299.jpg
│   └── mmexport1491013330299.jpg.json
└── Photos from 2024/
    └── ...
```

---

## 3. 环境要求与依赖

### 运行环境

- **推荐**：WSL Ubuntu 20.04+（Windows Subsystem for Linux）
- **兼容**：原生 Linux（Debian/Ubuntu/CentOS 等）
- **不支持**：原生 macOS（`date -d` 参数不兼容，需改用 `date -j`）

### 系统依赖

| 工具 | 用途 | 安装方式 |
|------|------|----------|
| `bash` ≥ 4.0 | 脚本运行时 | 预装 |
| `find` | 递归查找文件 | 预装 |
| `touch -d` | 修改文件时间戳 | 预装 |
| `date -d` | 日期字符串转 Unix 时间戳 | 预装 |
| `stat -c` | 读取文件修改时间 | 预装 |
| `jq` | 解析 JSON 元数据 | `sudo apt install jq` |

### 检查依赖

```bash
# 检查所有依赖
command -v bash jq find touch date stat
# 检查 touch 是否支持 -d 参数
touch -d "2025-01-01 00:00:00" /tmp/test && rm /tmp/test
```

### 文件系统注意事项

- **NTFS**（通过 WSL 挂载的 Windows 磁盘）：`touch` 修改时间戳可能失败或有精度限制，建议将文件复制到 ext4 分区再处理。
- **FAT32/exFAT**：时间戳精度为 2 秒，可能导致轻微误差。
- **ext4**：完全支持，推荐。

---

## 4. 脚本执行流程总览

以下是处理完整 Google Takeout 归档的推荐执行顺序：

```
步骤 1  fix_takeout_photo_time_wsl.sh   ← 从 JSON 恢复照片时间戳（最重要）
步骤 2  rename_photos.sh                ← 统一重命名为 IMGyyyyMMddHHmmss 格式
步骤 3  fix_img_timestamps.sh           ← 校正重命名后文件的时间戳（按文件名）
步骤 4  organize_photos.sh              ← 相机照片 → camera/
        organize_screenshots.sh         ← 截图 → screenshot/（可并行）
        organize_wechat.sh              ← 微信媒体 → wechat/（可并行）
步骤 5  delete_json_files.sh            ← 清理 JSON 元数据文件（可选）
```

**数据流图**

```
Google Takeout 压缩包
       ↓ 解压
Photos from XXXX/ 目录（含 *.jpg + *.json）
       ↓ fix_takeout_photo_time_wsl.sh
照片文件时间戳已校正
       ↓ rename_photos.sh
IMG20230302112040.jpg 格式文件
       ↓ organize_*.sh
camera/ / screenshot/ / wechat/ 分类目录
       ↓ delete_json_files.sh
干净的照片库（无 JSON 冗余文件）
```

---

## 5. 核心模块详解

### 5.1 `fix_takeout_photo_time_wsl.sh` — Google Takeout 时间戳修复

这是最复杂、最核心的脚本，负责将 Google Takeout 导出的 JSON 元数据中记录的 `photoTakenTime.timestamp` 写回对应照片文件的修改时间。

#### 执行流程

```
1. 检查运行环境（WSL/Linux）
2. 检查 jq 和 find 依赖
3. 枚举所有 "Photos from*" 目录
4. 遍历每个目录下的 *.json 文件
   ├── 解析 JSON 文件名 → 推导出对应的照片文件名 (base_name)
   ├── 候选文件列表生成（精确匹配 → 模糊匹配）
   ├── 从文件名提取时间戳（优先）
   ├── 回退：从 JSON 的 photoTakenTime.timestamp 读取时间戳
   └── 调用 touch -d "@$timestamp" 修改文件时间
5. 输出统计结果
```

#### JSON 文件名解析规则（正则优先级）

脚本通过分析 JSON 文件名来定位对应的照片文件，支持以下格式：

| 优先级 | JSON 文件名格式 | 示例 | 推导出的照片文件名 |
|--------|----------------|------|-------------------|
| 1 | `basename.ext.suffix(N).json` | `IMG_20240913.jpg.supplemental-metadata(1).json` | `IMG_20240913(1).jpg` |
| 2 | `basename(N).json` | `IMG_20240913(1).json` | `IMG_20240913(1)` |
| 3 | `basename.ext.suffix.json` | `IMG_20240913.jpg.supplemental-metadata.json` | `IMG_20240913.jpg` |
| 4 | `basename..json` | `IMG_20240913..json` | `IMG_20240913` |
| 5 | `basename.json` | `IMG_20240913.json` | `IMG_20240913` |

解析逻辑位于脚本第 134–156 行，使用 Bash `[[ =~ ]]` 正则语法，捕获组存入 `$BASH_REMATCH`。

#### 时间戳提取优先级

1. **文件名解析**（高优先级）：从照片文件名中的时间信息直接解析，避免 JSON 中记录的是 UTC 而导致时区偏差。
2. **JSON 读取**（兜底）：`jq -r '.photoTakenTime.timestamp'` 读取 Unix 时间戳。

**支持的文件名时间格式**（第 324–366 行）：

```bash
# 格式1: IMG_20230302_112040（含下划线）
[[ "$photo_name" =~ ([0-9]{8})_([0-9]{6}) ]]

# 格式2: IMG20230123102606（无下划线）
[[ "$photo_name" =~ ([0-9]{8})([0-9]{6}) ]]

# 格式3: WP_20131010_074（时间仅3位）
[[ "$photo_name" =~ ([0-9]{8})_([0-9]{3,6}) ]]

# 格式4: Screenshot_2016-02-28-13-06-34（带连字符）
[[ "$photo_name" =~ ([0-9]{4})-([0-9]{2})-([0-9]{2})-([0-9]{2})-([0-9]{2})-([0-9]{2}) ]]

# 格式5: Screenshot_20210803-084525（混合连字符）
[[ "$photo_name" =~ ([0-9]{8})-([0-9]{6}) ]]

# 格式6: mmexport1491013330299（13位毫秒时间戳）
[[ "$photo_name" =~ mmexport([0-9]{13}) ]]
```

#### 模糊匹配策略

当精确匹配失败时（候选列表为空），脚本执行模糊匹配：

```bash
find "$json_dir" -maxdepth 1 -type f -name "${base_name}*" ! -name "*.json"
```

- 若结果**唯一**：直接使用该文件。
- 若结果**为空且含编号**：展开 `${prefix}*\(${suffix}\)*` 通配符再匹配。
- 若结果**多个**：对每个候选文件执行精确名称比对，优先匹配与 JSON 编号一致的文件。

#### 命令行参数

| 参数 | 说明 |
|------|------|
| `--dry-run` | 仅统计将要修改的文件，不实际修改时间戳 |

---

### 5.2 `rename_photos.sh` — 照片统一重命名

将当前目录中的媒体文件重命名为 `IMG{yyyyMMddHHmmss}.ext` 或 `VID{yyyyMMddHHmmss}.ext` 格式。

#### 关键函数

**`is_image_file(filename)`** / **`is_video_file(filename)`**

- 遍历扩展名数组 `IMAGE_EXTENSIONS` / `VIDEO_EXTENSIONS`，对文件扩展名做逐一匹配。
- 扩展时只需在数组中添加新扩展名。

**`get_file_creation_time(file)`**

- 使用 `stat -c "%Y"` 获取文件修改时间（Unix 时间戳），再通过 `date -d "@{}"` 格式化为 `yyyyMMddHHmmss`（14位纯数字）。

**`generate_new_filename(original_file, creation_time)`**

- 根据文件类型选择 `IMG` 或 `VID` 前缀。
- 检测目标文件名是否已存在，若存在则对秒数进行递增，并自动处理进位（秒→分→时→日→月→年）。
- 最多尝试 999 次，超出则报错退出。

**`rename_file(file)`**

- 组合上述函数完成单文件重命名。
- 跳过 `.sh` 脚本文件和已符合目标格式（`IMG/VID` + 14位数字）的文件。

#### Dry-run 实现方式

脚本在解析到 `--dry-run` 参数后，使用函数**覆盖（override）**技术重定义 `rename_file()`，使其仅打印操作日志而不调用 `mv`。

```bash
if [ "$dry_run" = true ]; then
    rename_file() {
        # 只打印，不移动
        log_info "将重命名: $basename -> $new_filename"
    }
fi
```

---

### 5.3 `organize_photos.sh` — 相机照片整理

将符合手机/相机命名规律的照片移动到 `camera/` 子目录。

#### 文件识别逻辑 `is_camera_file(filename)`

两步过滤：
1. **扩展名过滤**：文件扩展名必须在 `EXTENSIONS` 数组中。
2. **前缀模式匹配**：文件名（`basename`）必须匹配 `PATTERNS` 数组中的某个 glob 模式。

当前支持的前缀模式：

| 模式 | 来源 |
|------|------|
| `WP_*` | Windows Phone |
| `IMG_*` | 通用（带下划线） |
| `VID_*` | 视频（带下划线） |
| `P_*` | 部分国产机型 |
| `PXL_*` | Google Pixel |
| `DSC_*` | 数码单反/卡片机 |
| `IMG*` | 通用（无下划线，含 rename_photos.sh 输出） |
| `[0-9]{8}_[0-9]{6}*` | 纯时间戳命名 |

#### 目录处理范围

| 函数 | 处理范围 |
|------|----------|
| `process_current_directory()` | 仅当前目录（`$SCRIPT_DIR`） |
| `process_photos_directories()` | 上级目录（`$BASE_DIR`）中 `maxdepth 2` 的 `Photos*` 目录 |

默认执行两个函数；使用 `--current` 参数只执行 `process_current_directory()`。

#### 冲突处理

目标文件已存在时，追加时间戳后缀再移动：

```bash
destination="${name_without_ext}_$(date +%Y%m%d_%H%M%S).${extension}"
```

---

### 5.4 `organize_screenshots.sh` — 截图整理

逻辑结构与 `organize_photos.sh` 完全一致，差别仅在识别规则：

- 文件名中包含 `screenshot`（大小写不敏感）即视为截图。

```bash
if [[ "${basename,,}" == *"screenshot"* ]]; then
    return 0  # 匹配截图
fi
```

目标目录为 `screenshot/`（而非 `camera/`）。

---

### 5.5 `organize_wechat.sh` — 微信媒体整理

逻辑结构与 `organize_photos.sh` 完全一致，差别仅在识别规则：

- 文件名以 `mmexport` 开头（微信导出的文件命名固定以此开头）。

目标目录为 `wechat/`。

---

### 5.6 `fix_img_timestamps.sh` — IMG/VID 时间戳校正

专门处理形如 `IMG20250409084814.MOV`、`IMG_20250727_141938.MOV` 的文件，从文件名直接校正时间戳，无需 JSON 文件。

#### `extract_timestamp(filename)` 函数

支持两种模式，返回 `YYYY-MM-DD HH:MM:SS` 格式字符串：

```bash
# 模式1: IMGyyyyMMddHHmmss（无下划线）
^(IMG|VID)([0-9]{4})([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})$

# 模式2: IMG_yyyyMMdd_HHmmss（带下划线）
^(IMG|VID)_([0-9]{4})([0-9]{2})([0-9]{2})_([0-9]{2})([0-9]{2})([0-9]{2})$
```

#### 幂等性保护

每个文件在应用时间戳前，先对比当前文件时间戳与目标时间戳：

```bash
time_diff=$((target_timestamp - current_mtime))
if [[ $time_diff -ge -1 ]] && [[ $time_diff -le 1 ]]; then
    ((skipped_files++))  # 误差 ≤1 秒，跳过
    continue
fi
```

确保重复运行脚本是安全且高效的。

#### 文件扫描方式

使用 `eval find` 构建动态扩展名过滤条件，将结果写入临时文件 `/tmp/img_files_$$`，处理完毕后删除：

```bash
eval "find . -type f \( $find_pattern \) -print0" > "$temp_file_list"
```

> ⚠️ 二次开发注意：`eval` 的使用要防范路径中的特殊字符。如需修改此处，建议改用数组拼接 `find` 参数的方式（见下文扩展指南）。

---

### 5.7 `delete_json_files.sh` — JSON 文件清理

递归删除当前目录及子目录中所有 `.json` 文件。

- 执行前打印将删除的文件数量，给用户确认机会（通过进度条展示而非交互式确认）。
- 对每个文件的删除失败给出详细诊断（权限、文件系统类型）。

---

## 6. 通用设计模式

所有脚本遵循以下统一模式，二次开发时应保持一致：

### 6.1 颜色日志

```bash
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }
```

### 6.2 进度条

```bash
show_progress() {
    local current=$1 total=$2 width=40
    local percentage=$((current * 100 / total))
    local completed=$((current * width / total))
    local remaining=$((width - completed))
    printf "\r🔄 ["
    printf "%*s" $completed | tr ' ' '█'  # 或 '+'
    printf "%*s" $remaining | tr ' ' '░'  # 或 '-'
    printf "] %d%% (%d/%d)" $percentage $current $total
}
```

### 6.3 命令行参数解析

```bash
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)    show_usage; exit 0 ;;
        -d|--dry-run) dry_run=true; shift ;;
        -l|--list)    list_only=true; shift ;;
        -c|--current) current_only=true; shift ;;
        -a|--all)     current_only=false; shift ;;
        *)            log_error "未知选项: $1"; show_usage; exit 1 ;;
    esac
done
```

### 6.4 安全文件遍历（NUL 分隔）

防止文件名中的空格、换行等特殊字符破坏遍历：

```bash
while IFS= read -r -d '' file; do
    # 处理 $file
done < <(find . -type f -print0)
```

### 6.5 Dry-run 实现

通过在参数解析后**重定义核心操作函数**（`rename_file` / `move_to_camera` 等）来实现 dry-run，主业务逻辑代码无需修改。

### 6.6 计数器模式

```bash
total=0; processed=0; skipped=0; errors=0
# 操作成功: ((processed++))
# 操作跳过: ((skipped++))
# 操作失败: ((errors++))
# 结束时输出统计
```

---

## 7. 关键函数速查表

| 脚本 | 函数名 | 行号（参考） | 功能 |
|------|--------|-------------|------|
| `fix_takeout_photo_time_wsl.sh` | `show_progress()` | ~94 | 进度条显示 |
| `fix_takeout_photo_time_wsl.sh` | JSON 名称解析块 | ~134 | 解析 JSON 文件名推导照片名 |
| `fix_takeout_photo_time_wsl.sh` | 候选文件生成块 | ~162 | 精确 + 模糊匹配候选文件 |
| `fix_takeout_photo_time_wsl.sh` | 时间戳提取块 | ~324 | 从文件名解析时间戳 |
| `rename_photos.sh` | `is_image_file()` | ~60 | 图片扩展名判断 |
| `rename_photos.sh` | `is_video_file()` | ~77 | 视频扩展名判断 |
| `rename_photos.sh` | `get_file_creation_time()` | ~105 | 读取文件修改时间 |
| `rename_photos.sh` | `generate_new_filename()` | ~120 | 生成带冲突处理的目标文件名 |
| `rename_photos.sh` | `rename_file()` | ~215 | 执行单文件重命名 |
| `organize_photos.sh` | `is_camera_file()` | ~73 | 相机文件模式匹配 |
| `organize_photos.sh` | `move_to_camera()` | ~103 | 移动文件到 camera/ |
| `fix_img_timestamps.sh` | `extract_timestamp()` | ~51 | 从文件名提取日期时间 |
| `fix_img_timestamps.sh` | `validate_datetime()` | ~89 | 验证日期时间有效性 |

---

## 8. 扩展开发指南

### 8.1 新增支持的文件格式

**在 `rename_photos.sh` 中新增图片格式**（以 `avif` 为例）：

```bash
declare -a IMAGE_EXTENSIONS=(
    # ... 已有格式 ...
    "avif" "AVIF"  # ← 新增
)
```

**在 `organize_photos.sh` / `organize_screenshots.sh` / `organize_wechat.sh` 中**，修改 `EXTENSIONS` 数组，方法相同。

**在 `fix_img_timestamps.sh` 中**，修改第 108 行的 `file_extensions` 字符串：

```bash
file_extensions="jpg jpeg png gif bmp tiff mov mp4 avi mkv wmv flv webm m4v 3gp avif"  # 追加 avif
```

### 8.2 新增文件名解析模式

#### 在 `fix_takeout_photo_time_wsl.sh` 中新增时间戳格式

在第 324–366 行的 `if-elif` 链末尾添加新格式：

```bash
# 示例：新增 YYYY_MMDD_HHmmss 格式（如 2023_0302_112040）
elif [[ "$photo_name" =~ ([0-9]{4})_([0-9]{4})_([0-9]{6}) ]]; then
    raw_date="${BASH_REMATCH[1]}${BASH_REMATCH[2]}"   # 20230302
    raw_time="${BASH_REMATCH[3]}"                      # 112040
    timestamp=$(date -d "${raw_date:0:4}-${raw_date:4:2}-${raw_date:6:2} ${raw_time:0:2}:${raw_time:2:2}:${raw_time:4:2}" +%s 2>/dev/null)
```

#### 在 `fix_img_timestamps.sh` 中新增文件名格式

在 `extract_timestamp()` 函数末尾的 `return 1` 前添加新的 `if` 分支：

```bash
# 模式3: CAM_yyyyMMdd_HHmmss
if [[ $name_without_ext =~ ^(CAM)_([0-9]{4})([0-9]{2})([0-9]{2})_([0-9]{2})([0-9]{2})([0-9]{2})$ ]]; then
    echo "${BASH_REMATCH[2]}-${BASH_REMATCH[3]}-${BASH_REMATCH[4]} ${BASH_REMATCH[5]}:${BASH_REMATCH[6]}:${BASH_REMATCH[7]}"
    return 0
fi
```

### 8.3 新增整理分类脚本

以新增 `organize_raw.sh`（整理 RAW 格式文件）为例，按以下模板创建：

```bash
#!/bin/bash

# 整理 RAW 格式照片脚本 - 将 RAW 文件移动到 raw/ 目录

# ── 1. 颜色与日志函数（直接复制其他 organize_*.sh 的头部）──────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# ── 2. 目录变量 ──────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RAW_DIR="$SCRIPT_DIR/raw"

# ── 3. 目标目录 ──────────────────────────────────────────────────────────────
declare -a RAW_EXTENSIONS=("cr2" "nef" "arw" "dng" "raw" "CR2" "NEF" "ARW" "DNG" "RAW")
total_moved=0; total_skipped=0

# ── 4. 识别函数 ──────────────────────────────────────────────────────────────
is_raw_file() {
    local ext="${1##*.}"
    for e in "${RAW_EXTENSIONS[@]}"; do [[ "$ext" == "$e" ]] && return 0; done
    return 1
}

# ── 5. 移动函数（含冲突处理）────────────────────────────────────────────────
move_to_raw() {
    local src="$1" fname=$(basename "$1") dest="$RAW_DIR/$fname"
    [[ -f "$dest" ]] && dest="$RAW_DIR/${fname%.*}_$(date +%Y%m%d_%H%M%S).${fname##*.}"
    if mv "$src" "$dest"; then log_success "移动: $fname -> raw/"; ((total_moved++))
    else log_error "移动失败: $fname"; fi
}

# ── 6. 其余逻辑（dry-run / 参数解析 / main）参照 organize_photos.sh 复制修改 ──
```

### 8.4 新增命令行参数

在参数解析的 `case` 语句中追加新 case，并在 `show_usage()` 中添加说明：

```bash
# 在 case 语句中：
-v|--verbose)
    verbose=true
    shift
    ;;

# 在 show_usage 中：
echo "  -v, --verbose  详细模式，显示每个文件的处理信息"
```

---

## 9. 数据流与变量约定

### 核心全局变量

| 变量 | 类型 | 含义 |
|------|------|------|
| `DRY_RUN` | `0/1` | 预览模式标志（`fix_takeout_photo_time_wsl.sh`） |
| `dry_run` | `true/false` | 预览模式标志（其他脚本） |
| `current_only` | `true/false` | 是否只处理当前目录 |
| `SCRIPT_DIR` | 路径字符串 | 脚本所在目录的绝对路径 |
| `total_files` | 整数 | 发现的目标文件总数 |
| `processed` / `processed_files` | 整数 | 成功处理的文件数 |
| `skipped` / `skipped_files` | 整数 | 跳过的文件数 |
| `error_files` | 整数 | 处理失败的文件数 |

### 时间戳格式约定

| 格式 | 示例 | 使用场景 |
|------|------|----------|
| Unix 时间戳（秒） | `1677717640` | `touch -d "@$timestamp"` |
| 人类可读格式 | `2023-03-02 11:20:40` | `touch -d "$timestamp"` |
| 14位紧凑格式 | `20230302112040` | 文件名组成部分 |

---

## 10. 错误处理规范

### 文件操作失败处理模板

```bash
if touch -d "@$timestamp" "$photo_file" 2>/dev/null; then
    ((processed++))
else
    error_msg=$(touch -d "@$timestamp" "$photo_file" 2>&1)
    echo -e "\n❌ 失败: $(basename "$photo_file")"
    echo "   错误信息: $error_msg"
    
    # 诊断：文件权限
    ls -la "$photo_file" 2>/dev/null && echo "   文件信息: $?"
    
    # 诊断：文件系统
    fs_type=$(df -T "$photo_file" 2>/dev/null | tail -1 | awk '{print $2}')
    echo "   文件系统: $fs_type"
    [[ "$fs_type" == "ntfs" || "$fs_type" == "vfat" ]] && \
        echo "   注意: $fs_type 可能不完全支持时间戳修改"
    
    ((skipped++))
fi
```

### 依赖缺失处理

```bash
if ! command -v jq &> /dev/null; then
    echo "❌ 错误: 请先安装 jq"
    echo "运行: sudo apt update && sudo apt install jq"
    exit 1
fi
```

---

## 11. 调试与验证

### 使用 Dry-run 模式

所有核心脚本都支持 dry-run，**生产运行前必须先执行 dry-run 验证**：

```bash
bash fix_takeout_photo_time_wsl.sh --dry-run
bash rename_photos.sh --dry-run
bash organize_photos.sh --dry-run
bash organize_screenshots.sh --dry-run
bash organize_wechat.sh --dry-run
```

### 调试单个文件

```bash
# 在脚本中添加临时调试输出
set -x  # 开启 bash 调试模式（打印每条命令）
# ... 要调试的代码 ...
set +x  # 关闭调试模式
```

### 验证时间戳修改结果

```bash
# 查看文件的修改时间
ls -la Photos\ from\ 2023/

# 以人类可读格式查看
stat --format="%y %n" Photos\ from\ 2023/*.jpg

# 批量确认
find . -name "*.jpg" -newer /tmp/reference_file
```

### 测试 JSON 解析

```bash
# 手动测试 jq 解析
jq -r '.photoTakenTime.timestamp' "Photos from 2023/IMG_20230302_112040.jpg.json"

# 转换为人类可读时间
jq -r '.photoTakenTime.timestamp' file.json | xargs -I{} date -d "@{}"
```

### 验证正则匹配

```bash
# 在 Bash 中测试正则
photo_name="IMG_20230302_112040.jpg"
if [[ "$photo_name" =~ ([0-9]{8})_([0-9]{6}) ]]; then
    echo "日期: ${BASH_REMATCH[1]}, 时间: ${BASH_REMATCH[2]}"
fi
```

---

## 12. 常见问题

### Q1：`touch: cannot touch 'xxx': Operation not permitted`

**原因**：文件位于 NTFS 或 exFAT 分区，WSL 对其时间戳修改权限受限。

**解决**：将文件复制到 WSL 的 ext4 家目录（如 `~/photos`）后再运行脚本，处理完毕后复制回去。

```bash
cp -r /mnt/d/Takeout/Photos ~/photos/
cd ~/photos
bash /path/to/fix_takeout_photo_time_wsl.sh
cp -r ~/photos /mnt/d/TakeoutFixed/
```

### Q2：脚本报 `⚠️ 跳过：xxx (找不到对应的照片文件)`

**原因**：JSON 文件名格式不在已支持的解析规则中，导致 `base_name` 推导错误。

**排查**：手动运行正则测试（见第 11 节），然后在第 134–156 行中添加新的解析分支。

### Q3：`jq` 未安装

```bash
sudo apt update && sudo apt install -y jq
```

### Q4：`rename_photos.sh` 运行后文件名出现冲突递增

**原因**：多张照片的文件修改时间恰好相同（精确到秒）。

**说明**：这是正常行为，脚本会自动在秒数上递增以避免冲突，不会丢失文件。

### Q5：想在 macOS 上运行

macOS 的 `date` 和 `stat` 命令参数与 GNU 版本不兼容，需做如下替换：

| Linux 命令 | macOS 等价命令 |
|-----------|--------------|
| `date -d "@$ts"` | `date -r "$ts"` |
| `date -d "$str" +%s` | `date -j -f "%Y-%m-%d %H:%M:%S" "$str" +%s` |
| `stat -c "%Y" "$f"` | `stat -f "%m" "$f"` |

建议通过 `brew install coreutils` 安装 GNU 工具链，然后将 `gdate` / `gstat` 加入 `PATH` 以 `date` / `stat` 命名。

### Q6：如何只处理某个特定年份的目录

在 `fix_takeout_photo_time_wsl.sh` 中，修改第 63 行的 `find` 条件：

```bash
# 原始：所有 "Photos from*" 目录
find . -type d -name "Photos from*" -print0

# 修改为：只处理 2023 年
find . -type d -name "Photos from 2023*" -print0
```

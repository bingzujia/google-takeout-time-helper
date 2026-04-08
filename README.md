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

### 方式二：从源码编译

```bash
git clone https://github.com/bingzujia/g_photo_take_out_helper.git
cd g_photo_take_out_helper
make build          # 产物：bin/gtoh
```

---

## 命令总览

```
gtoh
├── fix-takeout   修复 Google Takeout 照片时间戳（读取 JSON 元数据）
├── fix-img       修复 IMG/VID 文件时间戳（从文件名提取）
├── organize      按类型整理照片到子目录
├── rename        按修改时间批量重命名
└── clean-json    删除所有 JSON 附属文件
```

所有命令均支持 `--dry-run` 参数，可在不修改任何文件的情况下预览将要执行的操作。

---

## 命令详解

### `gtoh fix-takeout` — 修复 Google Takeout 时间戳

**用途**：Google Takeout 导出的照片，文件时间往往是下载时间而非拍摄时间。此命令读取每张照片对应的 `.json` 元数据文件，将照片的 EXIF 时间（`DateTimeOriginal`）以及文件系统时间（mtime）都修正为真实拍摄时间。如果系统安装了 `exiftool`，还会同步写入 GPS 坐标；否则仅修改文件系统时间。

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
# 预览（不修改任何文件）
gtoh fix-takeout --dir "/path/to/Takeout/Google Photos" --dry-run

# 实际执行
gtoh fix-takeout --dir "/path/to/Takeout/Google Photos"

# 在当前目录执行（目录下含 "Photos from*" 子目录）
gtoh fix-takeout
```

**预期输出**：

```
ℹ Processing /path/to/Photos from 2023
🔄 [████████████████████████░░░░░░░░░░░░░░░░] 60% (120/200)
✅ Done. Matched: 200, Unmatched JSON: 3
```

- `Matched`：成功找到对应照片并写入时间戳的 JSON 数量
- `Unmatched JSON`：找不到对应照片文件的 JSON 数量（可忽略，通常是封面图等）

**匹配逻辑**（多级回退）：

1. 精确名匹配（`photo.jpg.json` → `photo.jpg`）
2. 修饰符变体（`-已修改`、`-edited` 等后缀的编辑版本）
3. 模糊匹配回退（目录中仅一个文件匹配时采用）
4. 带编号截断名匹配（处理 Google Takeout 对长文件名的截断问题）

---

### `gtoh fix-img` — 修复 IMG/VID 文件时间戳

**用途**：针对文件名中直接包含时间信息的照片/视频（如从安卓手机传出的文件），从文件名提取时间并修正文件系统的修改时间（mtime）。无需 JSON 文件，无需 exiftool。

**支持的文件名格式**：

| 格式 | 示例 |
|------|------|
| `IMGyyyyMMddHHmmss` | `IMG20250409084814.jpg` |
| `IMG_yyyyMMdd_HHmmss` | `IMG_20250727_141938.jpg` |
| `VIDyyyyMMddHHmmss` | `VID20250409084814.mp4` |
| `VID_yyyyMMdd_HHmmss` | `VID_20250727_141938.mp4` |

**用法**：

```bash
# 预览
gtoh fix-img --dir ./photos --dry-run

# 实际执行（递归处理当前目录所有子目录）
gtoh fix-img --dir ./photos

# 当前目录
gtoh fix-img
```

**预期输出**：

```
✅ Done. Fixed: 87, Skipped: 12
```

- `Fixed`：成功更新时间戳的文件数量
- `Skipped`：时间戳已经正确（误差 ≤ 1 秒）或文件名无法解析的文件数量

---

### `gtoh organize` — 按类型整理照片

**用途**：将散落在一个目录中的照片按类型（相机照片、截图、微信图片）分别移动到对应子目录，方便归档管理。

**模式说明**：

| `--mode` | 匹配规则 | 目标目录 |
|----------|----------|----------|
| `camera` | 文件名前缀为 `WP_`、`IMG`、`VID`、`PXL_`、`DSC_`，或格式为 `YYYYMMDD_HHmmss` | `<dir>/camera/` |
| `screenshot` | 文件名（不区分大小写）包含 `screenshot` | `<dir>/screenshot/` |
| `wechat` | 文件名前缀为 `mmexport` | `<dir>/wechat/` |

**用法**：

```bash
# 预览相机照片整理结果
gtoh organize --mode camera --dir ./photos --dry-run

# 仅列出匹配文件，不移动
gtoh organize --mode screenshot --dir ./photos --list

# 实际整理微信图片（包含子目录）
gtoh organize --mode wechat --dir ./photos --recursive

# 指定自定义目标目录
gtoh organize --mode camera --dir ./photos --dest ./backup/camera
```

**预期输出**：

```
✅ Done. Moved: 143, Skipped: 0
```

- 若目标目录中已存在同名文件，自动在文件名中追加时间戳后缀（如 `IMG_001_20250101120000.jpg`）避免覆盖

---

### `gtoh rename` — 按时间戳重命名

**用途**：将照片/视频统一重命名为 `IMG yyyyMMddHHmmss.ext` / `VIDyyyyMMddHHmmss.ext` 格式，以文件的修改时间（mtime）为依据，便于按时间排序。

**命名规则**：

- 图片文件 → `IMGyyyyMMddHHmmss.jpg`（如 `IMG20230512143022.jpg`）
- 视频文件 → `VIDyyyyMMddHHmmss.mp4`（如 `VID20230512143022.mp4`）
- 同一秒内有多个文件时，依次递增 1 秒避免冲突（使用标准 `time.Add(time.Second)`，不会出现进位错误）

**用法**：

```bash
# 预览重命名结果
gtoh rename --dir ./photos --dry-run

# 列出文件与建议的新名称
gtoh rename --dir ./photos --list

# 实际执行
gtoh rename --dir ./photos
```

**预期输出（--list 模式）**：

```
IMG_20230512_143022.jpg  →  IMG20230512143022.jpg
IMG_20230512_143025.jpg  →  IMG20230512143025.jpg
...
```

**预期输出（执行后）**：

```
✅ Done. Renamed: 56, Skipped: 4
```

- `Skipped`：文件名已符合格式，无需重命名

---

### `gtoh clean-json` — 删除 JSON 附属文件

**用途**：Google Takeout 中每张照片都有一个 `.json` 元数据文件。在完成 `fix-takeout` 时间戳修复后，这些 JSON 文件便不再需要，可批量删除以节省空间。

**用法**：

```bash
# 预览将要删除的文件数量（不实际删除）
gtoh clean-json --dir ./photos --dry-run

# 实际删除（会先显示文件数量并要求确认）
gtoh clean-json --dir ./photos

# 跳过确认直接删除
gtoh clean-json --dir ./photos --yes
```

**预期输出**：

```
⚠ Found 247 JSON files. Proceed? [y/N]: y
✅ Done. Deleted: 247, Failed: 0
```

---

## 推荐工作流

处理一份新的 Google Takeout 导出，建议按以下顺序执行：

```bash
# 1. 修复照片时间戳（最重要）
gtoh fix-takeout --dir "Takeout/Google Photos"

# 2. 清理 JSON 文件（可选）
gtoh clean-json --dir "Takeout/Google Photos" --yes

# 3. 整理照片到子目录（可选）
gtoh organize --mode camera --dir "Takeout/Google Photos" --recursive
gtoh organize --mode screenshot --dir "Takeout/Google Photos" --recursive
gtoh organize --mode wechat --dir "Takeout/Google Photos" --recursive
```

---

## 注意事项

- **备份优先**：建议在执行前对原始文件进行备份，或先用 `--dry-run` 预览效果
- **exiftool**：`fix-takeout` 在有 `exiftool` 时可写入 EXIF 元数据（`DateTimeOriginal`）和 GPS 坐标；无 `exiftool` 时仅修改文件系统时间（mtime），对大多数场景已足够
- **Windows**：直接运行 `.exe`，无需 WSL 或 Bash 环境

---

## 历史参考：原 Shell 脚本

| Shell 脚本 | 对应 gtoh 命令 |
|------------|---------------|
| `fix_takeout_photo_time_wsl.sh` | `gtoh fix-takeout` |
| `fix_img_timestamps.sh` | `gtoh fix-img` |
| `organize_photos.sh` / `organize_screenshots.sh` / `organize_wechat.sh` | `gtoh organize --mode ...` |
| `rename_photos.sh` | `gtoh rename` |
| `delete_json_files.sh` | `gtoh clean-json` |

原脚本仅支持 WSL / Linux，新 `gtoh` 二进制全平台可用。

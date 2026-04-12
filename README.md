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

## 命令

```
gtoh migrate <input_dir> <output_dir>
```

`gtoh` 专注于修复 Google Takeout 导出照片的时间戳。`migrate` 命令读取每张照片对应的 `.json` 元数据文件，结合 EXIF 和文件名提取真实拍摄时间，将文件拷贝到输出目录并写入 EXIF 元数据（需安装 `exiftool`）。

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

## 推荐工作流

处理一份新的 Google Takeout 导出：

```bash
# 迁移照片（修复时间戳 + 拷贝到干净的输出目录）
gtoh migrate "Takeout/Google Photos" "output"
```

---

## 注意事项

- **备份优先**：建议在执行前对原始文件进行备份
- **exiftool**：安装 `exiftool` 后可写入 EXIF 元数据（`DateTimeOriginal`）和 GPS 坐标；未安装时仅拷贝文件，不写入 EXIF
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

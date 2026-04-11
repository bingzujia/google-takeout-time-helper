# gtoh 核心迁移命令 — 测试检查清单

## 一、单元测试

### 1.1 filetype_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| JPEG 检测 | JPEG 文件（任意扩展名） | 返回 `.jpg` |
| PNG 检测 | PNG 文件（任意扩展名） | 返回 `.png` |
| WebP 检测 | RIFF/WebP 文件 | 返回 `.webp` |
| MOV 检测 | ISO Media/MOV 文件 | 返回 `.mov` |
| MP4 检测 | ISO Media/MP4 文件 | 返回 `.mp4` |
| 未知类型 | 二进制文件 | 返回空字符串 |
| 扩展名匹配 | `.jpg` + JPEG 内容 | 返回空（无需重命名） |
| 扩展名不匹配 | `.heic` + JPEG 内容 | 返回 `.jpg` |

### 1.2 copier_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 正常复制 | 单文件 → 空目录 | 文件存在，内容一致 |
| 文件名冲突 | 同名文件已存在 | 返回 exists=true |
| 特殊文件名 | 含空格/中文/括号 | 正确复制 |
| 大文件 | 10MB 文件 | 正确复制，内存不爆炸 |

### 1.3 exif_writer_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 写入时间戳 | JPEG + 有效时间 | exiftool 读取 DateTimeOriginal 正确 |
| 写入 GPS | JPEG + 有效坐标 | exiftool 读取 GPS 正确 |
| 写入时间+GPS | JPEG + 时间 + GPS | 两者都正确 |
| 无 GPS | JPEG + 时间（无 GPS） | 时间写入，不写 GPS |
| minor 错误忽略 | 尼康 IFD 损坏文件 | 继续写入，不报错 |
| FileModifyDate | JPEG + 时间 | stat 获取的 mtime 正确 |
| 不支持格式 | WMV 文件 | 返回错误 |

### 1.4 hasher_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 已知内容 | "hello world" | SHA-256 匹配已知值 |
| 空文件 | 0 字节文件 | 正确计算空文件哈希 |
| 大文件 | 10MB 文件 | 正确计算，不 OOM |

### 1.5 metadata_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 完整信息 | 所有字段有值 | JSON 包含所有字段 |
| 无 GPS | GPS 为空 | JSON 省略 gps 字段 |
| 无 device | device 为空 | JSON 省略 device_folder/device_type |
| 部分时间戳 | 仅 EXIF 有值 | timestamp.exif 有值，filename/json 省略 |

### 1.6 logger_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 写入 SKIP | skip 记录 | log 文件包含 SKIP 行 |
| 写入 FAIL | fail 记录 | log 文件包含 FAIL 行 |
| 写入 INFO | info 记录 | log 文件包含 INFO 行 |
| 时间格式 | 任意时间 | `[YYYY-MM-DD HH:MM:SS]` 格式 |

### 1.7 migrator_test.go

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 完整流程 | 包含 JSON 的图片 | 复制 + exiftool + metadata + log |
| 无 JSON | 无侧车文件的图片 | 继续处理，deviceFolder 为空，记录 INFO |
| 无时间戳 | 无法解析时间的文件 | 跳过，记录到 log |
| 文件名冲突 | 输出目录已有同名文件 | 跳过，记录到 log |
| 空目录 | 无媒体文件 | 正常退出，计数为 0 |
| 多个 yearFolders | 2 个年份文件夹 | 所有文件都处理 |
| 类型不匹配 | `.heic` 实际是 JPEG | 重命名为 `.jpg`，继续处理 |
| 重命名冲突 | 重命名后目标已存在 | 图片+JSON 移到 error，记录 log |
| exiftool 失败 | 不支持的格式 | 图片+JSON 移到 error，记录 log |

## 二、集成测试

### 2.1 真实 Google Takeout 数据

准备测试数据：
```
test_takeout/
├── Photos from 2024/
│   ├── IMG_20240315_103000.jpg
│   ├── IMG_20240315_103000.jpg.json
│   ├── VID_20240315_103001.mp4
│   ├── VID_20240315_103001.mp4.json
│   ├── unknown_file.jpg          # 无 JSON
│   └── fake.heic                 # 实际是 JPEG（类型不匹配）
├── Photos from 2023/
│   └── IMG_20230101_120000.jpg
└── Album A/                       # 应被忽略
    └── photo.jpg
```

测试步骤：
1. `gtoh migrate test_takeout/ output/`
2. 验证 output/ 结构
3. 验证 metadata/ 内容
4. 验证 gtoh.log 内容
5. 验证 exiftool 写入结果
6. 验证 error/ 目录（如有失败文件）

### 2.2 边界场景

| 场景 | 预期 |
|------|------|
| 输入目录不存在 | 报错 exit 1 |
| 输出目录非空 | 提示并 exit 1 |
| 无 yearFolders | 提示并 exit 0 |
| 所有文件都无时间戳 | 全部跳过，processed=0 |
| 文件名全部冲突 | 全部跳过，skipped_exists=N |
| 所有文件类型不匹配 | 全部重命名后处理 |
| 重命名全部冲突 | 全部移到 error |

## 三、手动验证

### 3.1 exiftool 验证

```bash
# 检查输出文件的 EXIF 信息
exiftool -DateTimeOriginal -FileModifyDate -GPSLatitude -GPSLongitude output/IMG_xxx.jpg
```

### 3.2 metadata JSON 验证

```bash
# 检查 metadata 内容
cat output/metadata/*.json | jq .
```

### 3.3 log 文件验证

```bash
# 检查日志内容
cat output/gtoh.log
```

### 3.4 SHA-256 验证

```bash
# 验证 metadata 中的 SHA-256 与实际文件匹配
sha256sum output/IMG_xxx.jpg
cat output/metadata/*.json | jq -r '.sha256'
```

### 3.5 error 目录验证

```bash
# 检查 error 目录结构
find output/error/ -type f

# 验证图片和 JSON 配对
ls output/error/"Photos from 2024"/
# 应同时包含图片和对应的 .json 文件
```

### 3.6 文件类型检测验证

```bash
# 验证重命名后的文件类型
file output/IMG20221110153719.jpg
# 应显示 "JPEG image data"
```

## 四、性能测试

| 测试 | 规模 | 预期 |
|------|------|------|
| 小批量 | 100 文件 | < 30 秒 |
| 中批量 | 1000 文件 | < 5 分钟 |
| 大批量 | 10000 文件 | 可完成，内存稳定 |
| 大文件 | 单个 500MB 视频 | 正确处理，不 OOM |

## 五、回归测试

| 测试 | 命令 | 预期 |
|------|------|------|
| 所有现有测试 | `go test ./...` | 全部通过 |
| 编译检查 | `go build ./...` | 无错误 |
| test_matcher | `./test_matcher -folders <dir>` | 正常工作 |
| test_matcher dedup | `./test_matcher -dedup <dir>` | 正常工作 |
| 原有 migrate 功能 | `gtoh migrate <in> <out>` | 进度条正常显示 |

## 六、错误类型覆盖验证

使用实际日志中的错误类型构造测试文件：

| 错误类型 | 测试文件 | 预期行为 |
|----------|----------|----------|
| Not a valid HEIC (JPEG) | `.heic` 扩展名 + JPEG 内容 | 重命名为 `.jpg`，继续处理 |
| Not a valid PNG (JPEG) | `.png` 扩展名 + JPEG 内容 | 重命名为 `.jpg`，继续处理 |
| NikonScanIFD 损坏 | 尼康相机扫描的损坏 JPG | `-ignoreMinorErrors` 继续写入 |
| Not a valid JPG (MOV) | `.jpg` 扩展名 + MOV 内容 | 重命名为 `.mov`，继续处理 |
| Not a valid JPG (PNG) | `.jpg` 扩展名 + PNG 内容 | 重命名为 `.png`，继续处理 |
| Not a valid JPG (RIFF) | `.jpg` 扩展名 + WebP 内容 | 重命名为 `.webp`，继续处理 |
| IFD0 pointer 循环引用 | 损坏的 PNG 文件 | `-ignoreMinorErrors` 继续写入 |
| garbage at end (MP4) | 尾部有垃圾数据的 MP4 | `-ignoreMinorErrors` 继续写入 |
| WMV 不支持写入 | WMV 视频文件 | 移到 error 目录 |
| Truncated mdat (MOV) | 截断的 MOV 文件 | 移到 error 目录 |
| 重命名后冲突 | 多个 `.heic` 都是 JPEG，重命名后同名 | 移到 error 目录 |

# gtoh 文件类型处理 — 测试检查清单

## 一、单元测试

### 1.1 handleTypeMismatch 测试

| 测试用例 | 输入文件 | 预期行为 |
|----------|----------|----------|
| 类型匹配 | `normal.jpg` (JPEG) | 返回原路径，cleanup 为空操作 |
| 类型不匹配 | `fake.heic` (JPEG) | 返回临时路径 `.jpg`，cleanup 恢复 `.heic` |
| 类型不匹配 | `fake2.png` (JPEG) | 返回临时路径 `.jpg`，cleanup 恢复 `.png` |
| 检测失败 | 无法读取的文件 | 返回错误 |

### 1.2 cleanup 函数测试

| 测试用例 | 场景 | 预期 |
|----------|------|------|
| 正常 cleanup | exiftool 成功后 | 文件恢复原名 |
| 多次 cleanup | 调用两次 cleanup | 不报错（第二次是 no-op） |
| cleanup 后文件存在 | cleanup 后检查 | 原路径文件存在，临时路径不存在 |

### 1.3 重命名冲突测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 无冲突 | 临时文件名不存在 | 临时重命名成功 |
| 有冲突 | 临时文件名已存在 | 移到 error，记录 log |
| 冲突+JSON | 临时文件名已存在且有 JSON | 图片+JSON 同时移到 error |

## 二、集成测试

### 2.1 类型不匹配成功处理

准备测试数据：
```
test_input/Photos from 2024/
├── fake.heic          # 实际是 JPEG
├── fake.heic.json
├── fake2.png          # 实际是 JPEG
└── fake2.png.json
```

测试步骤：
1. `gtoh migrate test_input/ output/`
2. 验证 `output/fake.heic` 存在（保持原名）
3. 验证 `output/fake.heic` 包含 EXIF 数据
4. 验证 metadata 中 `output_filename: fake.heic`
5. 验证 `output/fake2.png` 存在（保持原名）
6. 验证 gtoh.log 无错误记录

### 2.2 重命名冲突场景

准备测试数据：
```
test_input/Photos from 2024/
├── a.heic             # 实际是 JPEG
├── a.heic.json
├── b.heic             # 实际是 JPEG（与 a.heic 内容相同）
└── b.heic.json
```

测试步骤：
1. 先处理 `a.heic` → 临时重命名为 `a.jpg` → exiftool → 恢复为 `a.heic`
2. 处理 `b.heic` → 临时重命名为 `b.jpg` → 冲突（`a.jpg` 已存在？不，已恢复为 `a.heic`）

**注意**：由于 cleanup 会恢复原名，所以多个 `.heic` 都是 JPEG 时不会冲突。
只有当输出目录已有同名 `.jpg` 文件时才会冲突。

### 2.3 真实冲突场景

准备测试数据：
```
output/ (pre-existing)
└── fake.jpg           # 已存在的文件

test_input/Photos from 2024/
├── fake.heic          # 实际是 JPEG
└── fake.heic.json
```

测试步骤：
1. `gtoh migrate test_input/ output/`
2. 验证 `output/fake.heic` 移到 `output/error/Photos from 2024/fake.heic`
3. 验证 `output/error/Photos from 2024/fake.heic.json` 存在
4. 验证 gtoh.log 包含 `FAIL rename_conflict`

### 2.4 不支持格式场景

准备测试数据：
```
test_input/Photos from 2020/
├── video.wmv
└── video.wmv.json
```

测试步骤：
1. `gtoh migrate test_input/ output/`
2. 验证 `output/error/Photos from 2020/video.wmv` 存在
3. 验证 `output/error/Photos from 2020/video.wmv.json` 存在
4. 验证 gtoh.log 包含 `FAIL filetype_unsupported`

## 三、手动验证

### 3.1 EXIF 写入验证

```bash
# 验证类型不匹配的文件 EXIF 正确写入
exiftool -DateTimeOriginal -GPSLatitude -GPSLongitude output/fake.heic
# 应显示正确的时间戳和 GPS 信息
```

### 3.2 文件名验证

```bash
# 验证输出文件保持原始扩展名
ls output/
# 应显示 fake.heic，而非 fake.jpg
```

### 3.3 metadata 验证

```bash
cat output/metadata/*.json | jq '.output_filename'
# 应显示 "fake.heic"，而非 "fake.jpg"
```

### 3.4 SHA-256 验证

```bash
sha256sum output/fake.heic
cat output/metadata/*.json | jq -r '.sha256'
# 两者应匹配
```

## 四、回归测试

| 测试 | 命令 | 预期 |
|------|------|------|
| 所有现有测试 | `go test ./...` | 全部通过 |
| 编译检查 | `go build ./...` | 无错误 |
| 正常 JPEG 处理 | `gtoh migrate` 正常文件 | 与之前一致 |
| 进度条 | 处理过程中显示进度 | 正常工作 |

## 五、错误类型覆盖验证

使用实际日志中的错误类型构造测试文件：

| 错误类型 | 测试文件 | 预期行为 |
|----------|----------|----------|
| Not a valid HEIC (JPEG) | `.heic` 扩展名 + JPEG 内容 | 临时重命名为 `.jpg` → exiftool → 恢复 `.heic` |
| Not a valid PNG (JPEG) | `.png` 扩展名 + JPEG 内容 | 临时重命名为 `.jpg` → exiftool → 恢复 `.png` |
| Not a valid JPG (MOV) | `.jpg` 扩展名 + MOV 内容 | 临时重命名为 `.mov` → exiftool → 恢复 `.jpg` |
| Not a valid JPG (PNG) | `.jpg` 扩展名 + PNG 内容 | 临时重命名为 `.png` → exiftool → 恢复 `.jpg` |
| Not a valid JPG (RIFF) | `.jpg` 扩展名 + WebP 内容 | 临时重命名为 `.webp` → exiftool → 恢复 `.jpg` |
| WMV 不支持写入 | WMV 视频文件 | 直接移到 error，不临时重命名 |

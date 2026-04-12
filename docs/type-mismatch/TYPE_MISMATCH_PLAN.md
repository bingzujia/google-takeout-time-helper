# gtoh 文件类型处理 — 临时重命名方案

## 一、问题背景

Google Takeout 导出的文件可能存在扩展名与实际内容不匹配的情况：
- `IMG20221110153719.heic` → 实际是 JPEG 内容
- `Screenshot_xxx.jpg` → 实际是 PNG 内容

exiftool 写入时会检测文件内容，发现扩展名不匹配则拒绝写入。

## 二、方案：临时重命名 → exiftool → 改回原名

### 2.1 流程

```
fake.heic (实际 JPEG)
  → 复制到 output/fake.heic
  → DetectFileType() 检测到实际类型为 JPEG
  → 临时重命名：output/fake.heic → output/fake.jpg
  → exiftool 写入 output/fake.jpg ✅
  → 改回原名：output/fake.jpg → output/fake.heic
  → SHA-256 计算 output/fake.heic
  → metadata 记录 output_filename: fake.heic
```

### 2.2 优势

| 对比项 | 永久重命名 | 临时重命名（本方案） |
|--------|-----------|---------------------|
| 输出文件名 | `fake.jpg` | `fake.heic`（保持原名） |
| 用户感知 | 扩展名变了 | 扩展名不变 |
| metadata 记录 | `output_filename: fake.jpg` | `output_filename: fake.heic` |
| 后续处理 | 需要手动处理 | 无需额外处理 |

### 2.3 可行性

- exiftool 只关心文件内容，不关心扩展名
- 临时重命名和改回都在同一进程内完成
- 即使中途失败，也有 error 目录兜底

---

## 三、实现细节

### 3.1 修改 `handleTypeMismatch` 函数

```go
// handleTypeMismatch detects actual file type, temporarily renames for exiftool,
// and returns a cleanup function to restore the original name.
func handleTypeMismatch(dstPath string) (tmpPath string, cleanup func(), err error) {
    newExt, err := DetectFileType(dstPath)
    if err != nil || newExt == "" {
        return dstPath, func() {}, nil
    }

    // Temporary rename
    base := strings.TrimSuffix(dstPath, filepath.Ext(dstPath))
    tmpPath = base + newExt
    if err := os.Rename(dstPath, tmpPath); err != nil {
        return "", nil, fmt.Errorf("temp rename: %w", err)
    }

    // Cleanup function to restore original name
    cleanup = func() {
        os.Rename(tmpPath, dstPath)
    }

    return tmpPath, cleanup, nil
}
```

### 3.2 修改 `processSingleFile` 流程

```go
// Step 3f: Detect file type and temporarily rename for exiftool
exifPath, cleanupRename, err := handleTypeMismatch(dstPath)
if err != nil {
    stats.FailedOther++
    logger.Fail("rename_error", entry.RelPath, err.Error())
    moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
    return
}

// Step 3g: exiftool write (use exifPath, which may have different extension)
hasGPS := finalGPS.Has
if err := exifWriter.WriteAll(exifPath, finalTimestamp, hasGPS, finalGPS.Lat, finalGPS.Lon); err != nil {
    stats.FailedExif++
    logger.Fail("exiftool_write", entry.RelPath, err.Error())
    cleanupRename() // restore original name before moving to error
    moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
    return
}

// Restore original filename
cleanupRename()

// Step 3h: Compute SHA-256 (on the file with original name)
sha256, err := SHA256File(dstPath)
```

### 3.3 重命名冲突处理

如果临时重命名后的目标文件已存在（例如多个 `.heic` 实际都是 JPEG，临时重命名后都叫 `fake.jpg`）：

```
检测到类型不匹配 → 计算临时文件名
  ├─ 临时文件不存在 → 临时重命名，exiftool 写入，改回原名
  └─ 临时文件已存在 → 移到 error/，记录 log
      └─ error/Photos from 2022/IMG20221110153719.heic
          └─ 同时移动 JSON 侧车文件
```

### 3.4 不支持格式处理

对于 exiftool 不支持写入的格式（如 WMV）：

```
检测到不支持格式 → 直接移到 error/，不进行临时重命名
```

---

## 四、改动文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/migrator/migrator.go` | 修改 | `handleTypeMismatch` 返回 cleanup 函数 |
| `internal/migrator/migrator.go` | 修改 | `processSingleFile` 调用 cleanup 恢复原名 |
| `internal/migrator/filetype.go` | 无变化 | 保持现有 `DetectFileType` 和 `IsWriteSupported` |

### 4.1 不改动

- `internal/migrator/logger.go`
- `internal/migrator/copier.go`
- `internal/migrator/exif_writer.go`
- `internal/migrator/hasher.go`
- `internal/migrator/metadata.go`

---

## 五、输出示例

### 5.1 正常流程（类型不匹配但成功处理）

```
output/
├── IMG20221110153719.heic    # 保持原名，但内容已写入 EXIF
├── Screenshot_xxx.jpg        # 保持原名
├── metadata/
│   └── abc123....json        # output_filename: IMG20221110153719.heic
└── gtoh.log                  # 无错误记录
```

### 5.2 重命名冲突

```
output/
├── fake.jpg                  # 已存在的文件
├── error/
│   └── Photos from 2022/
│       ├── fake.heic         # 冲突文件（实际也是 JPEG）
│       └── fake.heic.json    # 配对 JSON
└── gtoh.log
    [2024-04-11 10:30:03] FAIL rename_conflict: Photos from 2022/fake.heic (actual: jpg, target: fake.jpg exists)
```

---

## 六、实现步骤

1. 修改 `handleTypeMismatch` 返回 `(tmpPath string, cleanup func(), err error)`
2. 修改 `processSingleFile` 调用 `cleanup()` 恢复原名
3. 更新错误处理：exiftool 失败时先 `cleanup()` 再移到 error
4. 确认 metadata 中 `output_filename` 使用原始文件名
5. 测试验证

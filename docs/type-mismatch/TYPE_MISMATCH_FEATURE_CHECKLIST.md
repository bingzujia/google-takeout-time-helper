# gtoh 文件类型处理 — 功能验证清单

## 一、临时重命名核心逻辑

- [ ] **类型检测**：`DetectFileType` 正确识别实际文件类型
- [ ] **临时重命名**：扩展名不匹配时临时重命名为正确扩展名
- [ ] **exiftool 写入**：使用临时文件名成功写入 EXIF
- [ ] **恢复原名**：exiftool 成功后恢复原始文件名
- [ ] **SHA-256 计算**：基于恢复原名后的文件计算哈希
- [ ] **metadata 记录**：`output_filename` 使用原始文件名

## 二、类型检测覆盖

- [ ] **JPEG 检测**：`.heic`/`.png` 实际是 JPEG → 返回 `.jpg`
- [ ] **PNG 检测**：`.jpg` 实际是 PNG → 返回 `.png`
- [ ] **WebP 检测**：`.jpg` 实际是 WebP → 返回 `.webp`
- [ ] **MOV 检测**：`.jpg` 实际是 MOV → 返回 `.mov`
- [ ] **MP4 检测**：`.jpg` 实际是 MP4 → 返回 `.mp4`
- [ ] **类型匹配**：扩展名与实际类型一致 → 返回空字符串

## 三、重命名冲突处理

- [ ] **冲突检测**：临时重命名目标文件已存在时检测冲突
- [ ] **冲突处理**：图片 + JSON 同时移到 error 目录
- [ ] **冲突记录**：`FAIL rename_conflict: path (actual: jpg, target: xxx.jpg exists)`
- [ ] **error 目录结构**：保留原始路径 `error/Photos from 2022/fake.heic`
- [ ] **JSON 同步移动**：配对 JSON 同时移到 error

## 四、不支持格式处理

- [ ] **WMV 检测**：`IsWriteSupported` 返回 false
- [ ] **直接移到 error**：不进行临时重命名
- [ ] **记录日志**：`FAIL filetype_unsupported: path`

## 五、错误恢复

- [ ] **exiftool 失败**：先 `cleanup()` 恢复原名，再移到 error
- [ ] **临时重命名失败**：移到 error，不执行 cleanup
- [ ] **cleanup 幂等**：多次调用 cleanup 不报错

## 六、输出文件验证

- [ ] **文件名保持**：输出文件扩展名与输入一致
- [ ] **EXIF 写入**：`exiftool -DateTimeOriginal` 正确写入
- [ ] **GPS 写入**：`exiftool -GPSLatitude/GPSLongitude` 正确写入
- [ ] **FileModifyDate**：文件系统修改时间正确覆盖

## 七、metadata JSON 验证

- [ ] **output_filename**：使用原始文件名（非临时文件名）
- [ ] **SHA-256**：基于恢复原名后的文件计算
- [ ] **timestamp 来源**：三种来源全部记录
- [ ] **GPS 来源**：两种来源全部记录

## 八、日志文件验证

- [ ] **无错误记录**：类型不匹配但成功处理时不记录错误
- [ ] **冲突记录**：重命名冲突时记录 FAIL
- [ ] **不支持格式**：WMV 等不支持格式记录 FAIL

## 九、边界条件

- [ ] **多个相同类型**：多个 `.heic` 都是 JPEG，临时重命名冲突 → 移到 error
- [ ] **特殊文件名**：含空格、中文、括号的文件名正确处理
- [ ] **大文件**：>100MB 视频正确处理
- [ ] **只读文件**：权限不足时记录错误并移到 error

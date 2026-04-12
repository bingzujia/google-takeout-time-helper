# gtoh 核心迁移命令 — 功能验证清单

## 一、基础功能

- [ ] **命令签名**：`gtoh migrate <input_dir> <output_dir>` 接收两个位置参数
- [ ] **参数校验**：缺少参数时输出用法提示并 exit 1
- [ ] **输出目录检查**：非空时提示并 exit 1
- [ ] **输出目录创建**：不存在时自动创建
- [ ] **文件夹分类**：正确识别 `Photos from XXXX` 年份文件夹
- [ ] **忽略 albumFolders**：不处理非年份文件夹
- [ ] **进度条显示**：扫描阶段显示文件总数，处理阶段实时更新进度

## 二、媒体文件处理

- [ ] **递归扫描**：遍历 yearFolders 下所有子目录
- [ ] **跳过 JSON**：不处理 `.json` 文件
- [ ] **支持格式**：jpg/jpeg/png/gif/bmp/tiff/heic/heif/webp/mp4/mov/avi/mkv/wmv
- [ ] **JSON 侧车匹配**：通过 `matcher.JSONForFile` 找到对应 JSON
- [ ] **无 JSON 继续处理**：无 JSON 时 deviceFolder 为空但不跳过
- [ ] **无 JSON 记录到 log**：`INFO no_json_sidecar: path`

## 三、时间戳提取

- [ ] **EXIF 时间戳**：`parser.ParseEXIFTimestamp` 正确读取
- [ ] **文件名时间戳**：`parser.ParseFilenameTimestamp` 正确解析
- [ ] **JSON 时间戳**：从 `photoTakenTime.timestamp` 正确读取
- [ ] **优先级正确**：EXIF > Filename > JSON
- [ ] **全部记录**：metadata JSON 中记录三种来源的值
- [ ] **无时间戳跳过**：三者均无时跳过并记录

## 四、GPS 提取

- [ ] **EXIF GPS**：`parser.ParseEXIFGPS` 正确读取
- [ ] **JSON GPS**：从 `geoData` 正确读取
- [ ] **优先级正确**：EXIF > JSON
- [ ] **全部记录**：metadata JSON 中记录两种来源的值
- [ ] **GPS (0,0) 过滤**：无效 GPS 视为无 GPS
- [ ] **无 GPS 继续处理**：不因为无 GPS 而跳过文件

## 五、文件复制

- [ ] **扁平输出**：所有文件输出到 output_dir 根目录
- [ ] **文件名冲突检测**：目标已存在时跳过
- [ ] **冲突记录到 log**：`SKIP file_exists: path`

## 六、文件类型检测与重命名

- [ ] **`file` 命令检测**：正确识别实际文件类型
- [ ] **扩展名映射**：JPEG→.jpg, PNG→.png, RIFF/WebP→.webp, ISO Media/MP4→.mp4 等
- [ ] **类型匹配不重命名**：扩展名与实际类型一致时不重命名
- [ ] **类型不匹配重命名**：`.heic` 实际是 JPEG → 重命名为 `.jpg`
- [ ] **重命名冲突检测**：目标文件名已存在时检测冲突
- [ ] **重命名冲突处理**：图片 + JSON 同时移到 error 目录
- [ ] **冲突记录到 log**：`FAIL rename_conflict: path (actual: JPEG, target: xxx.jpg exists)`

## 七、exiftool 写入

- [ ] **DateTimeOriginal**：正确写入 EXIF 时间戳
- [ ] **FileModifyDate**：正确覆盖文件系统修改时间
- [ ] **GPS 写入**：有 GPS 时写入 GPSLatitude/GPSLongitude
- [ ] **无 GPS 不写**：无 GPS 时不传 GPS 参数
- [ ] **-ignoreMinorErrors**：忽略 minor 级别错误继续写入
- [ ] **失败处理**：exiftool 失败时图片 + JSON 移到 error 目录
- [ ] **不生成备份**：使用 `-overwrite_original`

## 八、error 目录

- [ ] **目录结构**：`output/error/<原始相对路径>/` 保留原始目录结构
- [ ] **图片移动**：失败文件移动到 error 目录
- [ ] **JSON 同步移动**：有 JSON 侧车文件时同时移动
- [ ] **重命名冲突**：图片 + JSON 同时移到 error
- [ ] **exiftool 失败**：图片 + JSON 同时移到 error
- [ ] **不支持格式**：图片 + JSON 同时移到 error

## 九、SHA-256 与 metadata

- [ ] **SHA-256 计算**：基于修改后的文件
- [ ] **metadata 目录**：输出到 `output_dir/metadata/`
- [ ] **文件名格式**：`<sha256>.json`
- [ ] **JSON 内容完整**：包含 original_path、timestamp（三种来源）、gps（两种来源）、device 信息
- [ ] **可选字段省略**：缺失的 GPS/device 字段不写入

## 十、日志文件

- [ ] **log 文件路径**：`output_dir/gtoh.log`
- [ ] **SKIP 记录**：无时间戳、文件冲突
- [ ] **FAIL 记录**：exiftool 失败、重命名冲突、不支持格式
- [ ] **INFO 记录**：无 JSON 侧车文件
- [ ] **时间戳格式**：`[YYYY-MM-DD HH:MM:SS]`

## 十一、统计输出

- [ ] **scanned**：扫描文件总数
- [ ] **processed**：成功处理数
- [ ] **skipped_no_timestamp**：无时间戳跳过数
- [ ] **skipped_exists**：文件冲突跳过数
- [ ] **failed_exiftool**：exiftool 失败数
- [ ] **failed_other**：其他错误数
- [ ] **终端输出格式**：与需求文档一致

## 十二、边界条件

- [ ] **空输入目录**：无 yearFolders 时正常退出
- [ ] **大文件处理**：>100MB 视频正确处理
- [ ] **特殊文件名**：含空格、中文、括号的文件名正确处理
- [ ] **只读文件**：权限不足时记录错误并移到 error
- [ ] **符号链接**：不跟随符号链接
- [ ] **并发安全**：metadata 目录并发写入安全

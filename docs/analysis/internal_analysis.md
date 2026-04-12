# `internal/` 目录详细实现逻辑分析

---

## 一、包总览与依赖关系

```
internal/
├── cleaner/        # JSON 文件清理器（无外部依赖）
├── matcher/        # JSON-照片配对器（依赖 parser）
├── metadata/       # 元数据写入器（无外部依赖）
├── organizer/      # 文件分类整理器（无外部依赖）
├── parser/         # 文件名时间戳解析器（无外部依赖）
│   ├── timestamp.go       # 通用时间戳解析（9种模式）
│   ├── timestamp_test.go  # 通用解析测试
│   ├── imgname.go         # IMG/VID 专用时间戳解析（2种模式）
│   └── imgname_test.go    # IMG/VID 解析测试
├── progress/       # 进度条与日志输出（无外部依赖）
└── renamer/        # 文件重命名器（无外部依赖）
```

**依赖关系**：仅 `matcher` 依赖 `parser`，其余包均只依赖标准库。

---

## 二、`parser/timestamp.go` — 通用文件名时间戳解析

### 2.1 核心数据结构

```go
type pattern struct {
    re    *regexp.Regexp                    // 正则表达式
    parse func(m []string) (time.Time, bool) // 解析函数
}
```

### 2.2 8 种解析模式（严格按数组顺序逐一尝试）

> **注意**：以下为 `timestamp.go` 实际代码中的模式。`merged_script_design.md` 2.1 中的 12 种格式是合并后的设计规范，其中格式 1-2（IMG/VID 专用锚定模式）由 `imgname.go` 独立处理，格式 3-11 对应本节的 8 种模式（加上 mmexport 的 3 种子变体）。

| 优先级 | 正则表达式 | 匹配示例 | 时间提取方式 | 特殊处理 | 对应设计规范 |
|--------|-----------|----------|-------------|----------|-------------|
| 1 | `(?i)(?:IMG\|VID\|WP\|P\|PXL\|DSC\|MVIMG\|PANO\|BURST)_(\d{8})_(\d{6})` | `IMG_20230302_112040` | `parseDateTime8_6(m[1], m[2])` | 不区分大小写，支持 9 种前缀：`IMG`, `VID`, `WP`, `P`, `PXL`, `DSC`, `MVIMG`, `PANO`, `BURST` | 2.1 格式 3 |
| 2 | `(?i)(?:IMG\|VID\|MVIMG\|PANO\|BURST)(\d{8})(\d{6})` | `IMG20230123102606` | `parseDateTime8_6(m[1], m[2])` | 无下划线紧凑格式，支持 5 种前缀：`IMG`, `VID`, `MVIMG`, `PANO`, `BURST` | 2.1 格式 4 |
| 3 | `(?i)WP_(\d{8})_(\d{3,6})` | `WP_20131010_074` | 右侧补零至 6 位后调用 `parseDateTime8_6` | `074` → `074000` → `07:40:00`（**注意**：设计规范 2.1 格式 5 要求时间不足 6 位时默认 12:00:00，当前代码实现为右侧补零，两者不一致） | 2.1 格式 5 |
| 4 | `(?i)Screenshot_(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})` | `Screenshot_2016-02-28-13-06-34` | `parseComponents(m[1]~m[6])` | 6 组数字用连字符分隔 | 2.1 格式 7 |
| 5 | `(?i)Screenshot_(\d{8})-(\d{6})` | `Screenshot_20210803-084525` | `parseDateTime8_6(m[1], m[2])` | 日期时间用单个连字符分隔 | 2.1 格式 8 |
| 6 | `(?i)mmexport(\d{10})\d{0,3}(?:[(-].*)?` | `mmexport1491013330299` | 取前 10 位作为秒级 Unix 时间戳，调用 `time.Unix(sec, 0).UTC()` | 可选的后缀数字或 `(-` 开头的任意内容（含中文后缀、括号编号等） | 2.1 格式 9-11（3 种子变体合并为 1 个正则） |
| 7 | `^(\d{8})_(\d{6})~\d+` | `20151120_120004~2` | `parseDateTime8_6(m[1], m[2])` | 必须以日期开头（`^` 锚定），忽略 `~` 后的连拍序号 | 2.1 格式 6 |
| 8 | `^(\d{8})_(\d{6})` | `20151120_120004` | `parseDateTime8_6(m[1], m[2])` | 纯日期_时间格式，必须锚定开头（`^`） | 2.1 格式 3（纯数字日期变体） |

**模式排列顺序说明**：
- 模式 7 必须在模式 8 之前，否则 `20151120_120004~2` 会被模式 8 提前匹配（模式 8 的正则 `^(\d{8})_(\d{6})` 会匹配到 `20151120_120004` 部分，忽略 `~2`）
- 模式 1 和模式 2 均包含 `IMG`/`VID` 前缀，但模式 1 要求下划线分隔（`IMG_`），模式 2 为紧凑格式（`IMG` 直接接数字），两者不会冲突
- 模式 3（WP 短时分秒）必须在模式 1 之后，因为 `WP_20131010_074` 也会被模式 1 的正则尝试匹配（但模式 1 要求 6 位时间，`074` 只有 3 位，不匹配）

### 2.3 `ParseFilenameTimestamp` 执行流程

```
输入: filename (如 "path/to/IMG_20230302_112040.jpg")
│
├─ 步骤1: 提取纯文件名（去路径、去扩展名）
│   └─ base = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
│       ├─ 输入 "path/to/IMG_20230302_112040.jpg" → "IMG_20230302_112040"
│       ├─ 输入 "WP_20131010_074.jpg" → "WP_20131010_074"
│       ├─ 输入 "Screenshot_2016-02-28-13-06-34.png" → "Screenshot_2016-02-28-13-06-34"
│       └─ 输入 "mmexport1491013330299-已修改.jpg" → "mmexport1491013330299-已修改"
│
├─ 步骤2: 按优先级遍历 8 种模式（对应设计规范 2.1 格式 3-11）
│   │
│   ├─ 对每种模式:
│   │   ├─ m = p.re.FindStringSubmatch(base)
│   │   │
│   │   ├─ m == nil → 继续下一种模式
│   │   │
│   │   └─ m != nil → 调用 p.parse(m)
│   │       │
│   │       ├─ parseDateTime8_6(date, timeStr)
│   │       │   ├─ len(date) != 8 → 返回 (zero, false)
│   │       │   ├─ len(timeStr) != 6 → 返回 (zero, false)
│   │       │   └─ 调用 parseComponents(date[0:4], date[4:6], date[6:8],
│   │       │                           timeStr[0:2], timeStr[2:4], timeStr[4:6])
│   │       │
│   │       └─ parseComponents(year, month, day, hour, min, sec)
│   │           ├─ 对 6 个参数逐一调用 strconv.Atoi
│   │           │   ├─ 任一转换失败 → 返回 (zero, false)
│   │           │   └─ 全部成功 → 得到 6 个整数
│   │           └─ time.Date(year, month, day, hour, min, sec, 0, time.UTC)
│   │               └─ 返回 (t, true)
│   │
│   └─ 所有模式均不匹配 → 返回 (time.Time{}, false)
│
└─ 输出: (time.Time, bool)
```

### 2.4 `parseDateTime8_6` 分支清单

| 输入条件 | 分支 | 返回值 |
|----------|------|--------|
| `len(date) != 8` | 长度校验失败 | `(time.Time{}, false)` |
| `len(timeStr) != 6` | 长度校验失败 | `(time.Time{}, false)` |
| 两者长度均正确 | 调用 `parseComponents` | 取决于 `parseComponents` 结果 |

### 2.5 `parseComponents` 分支清单

| 输入条件 | 分支 | 返回值 |
|----------|------|--------|
| `year` 无法转为整数 | `strconv.Atoi` 失败 | `(time.Time{}, false)` |
| `month` 无法转为整数 | `strconv.Atoi` 失败 | `(time.Time{}, false)` |
| `day` 无法转为整数 | `strconv.Atoi` 失败 | `(time.Time{}, false)` |
| `hour` 无法转为整数 | `strconv.Atoi` 失败 | `(time.Time{}, false)` |
| `min` 无法转为整数 | `strconv.Atoi` 失败 | `(time.Time{}, false)` |
| `sec` 无法转为整数 | `strconv.Atoi` 失败 | `(time.Time{}, false)` |
| 6 个参数均转换成功 | 调用 `time.Date` | `(t, true)`，时区为 UTC |

---

## 三、`parser/imgname.go` — IMG/VID 专用时间戳解析

### 3.1 2 种解析模式（严格按数组顺序逐一尝试）

> **对应设计规范**：本节 2 种模式对应 `merged_script_design.md` 2.1 中的格式 1-2（IMG/VID 专用锚定模式）。

| 优先级 | 正则表达式 | 匹配示例 | 提取的时间 | 对应设计规范 |
|--------|-----------|----------|------------|-------------|
| 1 | `(?i)^(?:IMG\|VID)_(\d{8})_(\d{6})` | `IMG_20250727_141938` | `2025-07-27 14:19:38` | 2.1 格式 2 |
| 2 | `(?i)^(?:IMG\|VID)(\d{8})(\d{6})` | `IMG20250409084814` | `2025-04-09 08:48:14` | 2.1 格式 1 |

### 3.2 `ParseIMGVIDFilename` 执行流程

```
输入: filename (如 "VID20201231235959.mp4")
│
├─ 步骤1: 提取纯文件名（去路径、去扩展名）
│   └─ base = "VID20201231235959"
│
├─ 步骤2: 遍历 2 种模式
│   ├─ 模式1: 正则匹配 base
│   │   ├─ 匹配成功 → parseDateTime8_6 → 返回结果
│   │   └─ 匹配失败 → 继续模式2
│   │
│   └─ 模式2: 正则匹配 base
│       ├─ 匹配成功 → parseDateTime8_6 → 返回结果
│       └─ 匹配失败 → 返回 (time.Time{}, false)
│
└─ 输出: (time.Time, bool)
```

### 3.3 与 `timestamp.go` 的协同关系

| 维度 | `timestamp.go` | `imgname.go` |
|------|---------------|-------------|
| 对应设计规范 | 2.1 格式 3-11（通用模式） | 2.1 格式 1-2（IMG/VID 专用锚定模式） |
| 支持前缀 | 9 种（IMG, VID, WP, P, PXL, DSC, MVIMG, PANO, BURST）+ Screenshot + mmexport | 仅 2 种（IMG, VID） |
| 模式数量 | 8 种 | 2 种 |
| 锚定开头 | 部分模式锚定（模式 7-8 用 `^`） | 全部锚定（均用 `^`） |
| 正则严格度 | 部分模式不锚定开头，可能在文件名任意位置匹配 | 必须从文件名开头匹配 |
| 调用方 | `matcher.resolveTimestamp` | 未被其他包调用（独立入口） |

**两者合起来覆盖设计规范 2.1 的全部 11 种时间戳格式**：
- `imgname.go` 处理格式 1-2：`^(IMG|VID)...` 锚定开头，精确匹配
- `timestamp.go` 处理格式 3-11：通用模式，覆盖 Screenshot、mmexport、WP、纯数字日期等

---

## 四、`matcher/json_matcher.go` — JSON-照片配对器

### 4.1 核心数据结构

```go
type GooglePhoto struct {
    PhotoTakenTime struct { Timestamp string } `json:"photoTakenTime"`
    GeoData struct {
        Latitude, Longitude, Altitude float64
    } `json:"geoData"`
}

type MatchResult struct {
    JSONFile, PhotoFile string
    Timestamp           time.Time
    Lat, Lon, Alt       float64
}
```

### 4.2 `MatchAll` 执行流程

```
输入: dir (目录路径)
│
├─ 步骤1: 读取目录内容
│   └─ entries, err := os.ReadDir(dir)
│       ├─ err != nil → 返回 (nil, nil, err)
│       └─ err == nil → 继续
│
├─ 步骤2: 构建 nonJSON 映射（小写文件名 → 完整路径）
│   │
│   ├─ 遍历 entries 中每个条目 e:
│   │   ├─ e.IsDir() == true → 跳过（continue）
│   │   ├─ filepath.Ext(e.Name()) 等于 ".json"（不区分大小写） → 跳过
│   │   └─ 其他情况 → nonJSON[strings.ToLower(e.Name())] = filepath.Join(dir, e.Name())
│   │
│   └─ 结果: nonJSON = {"img_2023.jpg": "/path/IMG_2023.jpg", ...}
│
├─ 步骤3: 遍历所有 .json 文件进行配对
│   │
│   ├─ 遍历 entries 中每个条目 e:
│   │   │
│   │   ├─ 分支A: e.IsDir() == true → 跳过
│   │   │
│   │   ├─ 分支B: 扩展名不是 ".json" → 跳过
│   │   │
│   │   └─ 分支C: 是 .json 文件 → 执行配对流程
│   │       │
│   │       ├─ 步骤C1: 解析 JSON
│   │       │   ├─ gp, err := parseJSON(jsonPath)
│   │       │   │   ├─ err != nil（文件读取失败 / JSON 格式错误）
│   │       │   │   │   → unmatched = append(unmatched, jsonPath)
│   │       │   │   │   → continue
│   │       │   │   └─ err == nil → 继续
│   │       │
│   │       ├─ 步骤C2: 推导照片文件名
│   │       │   └─ baseName = deriveBaseName(name)
│   │       │       └─ 直接去掉 .json 扩展名
│   │       │
│   │       ├─ 步骤C3: 查找对应照片文件
│   │       │   ├─ photoPath, found := findPhoto(baseName, nonJSON, dir)
│   │       │   │   ├─ found == false → unmatched = append(unmatched, jsonPath) → continue
│   │       │   │   └─ found == true → 继续
│   │       │
│   │       ├─ 步骤C4: 解析时间戳
│   │       │   └─ ts := resolveTimestamp(photoPath, gp)
│   │       │
│   │       └─ 步骤C5: 构建 MatchResult
│   │           └─ results = append(results, MatchResult{
│   │                   JSONFile: jsonPath,
│   │                   PhotoFile: photoPath,
│   │                   Timestamp: ts,
│   │                   Lat: gp.GeoData.Latitude,
│   │                   Lon: gp.GeoData.Longitude,
│   │                   Alt: gp.GeoData.Altitude,
│   │               })
│   │
│   └─ 遍历结束
│
└─ 输出: (results []MatchResult, unmatched []string, nil)
```

### 4.3 `findPhoto` 的 4 种匹配策略（严格按顺序尝试）

```
输入: baseName (如 "IMG_20240913_162956.jpg"), nonJSON 映射, dir
│
├─ 策略1: 精确匹配
│   ├─ 查找: nonJSON[strings.ToLower(baseName)]
│   ├─ 找到 → 返回 (path, true)
│   └─ 未找到 → 继续策略2
│
├─ 策略2: 变体后缀匹配
│   │
│   ├─ 前提: baseName 必须有扩展名（ext != ""）
│   │   ├─ ext == "" → 跳过策略2，继续策略3
│   │   └─ ext != "" → 执行以下循环:
│   │       │
│   │       ├─ stem = 去掉扩展名的部分
│   │       │
│   │       ├─ 遍历 5 种后缀（按顺序）:
│   │       │   │
│   │       │   ├─ 后缀 = "-已修改" → candidate = stem + "-已修改" + ext → 查找 nonJSON
│   │       │   │   ├─ 找到 → 返回 (path, true)
│   │       │   │   └─ 未找到 → 继续下一个后缀
│   │       │   │
│   │       │   ├─ 后缀 = "-编辑" → candidate = stem + "-编辑" + ext → 查找
│   │       │   │   ├─ 找到 → 返回 (path, true)
│   │       │   │   └─ 未找到 → 继续
│   │       │   │
│   │       │   ├─ 后缀 = "-修改" → candidate = stem + "-修改" + ext → 查找
│   │       │   │   ├─ 找到 → 返回 (path, true)
│   │       │   │   └─ 未找到 → 继续
│   │       │   │
│   │       │   ├─ 后缀 = "-edited" → candidate = stem + "-edited" + ext → 查找
│   │       │   │   ├─ 找到 → 返回 (path, true)
│   │       │   │   └─ 未找到 → 继续
│   │       │   │
│   │       │   └─ 后缀 = "-modified" → candidate = stem + "-modified" + ext → 查找
│   │       │       ├─ 找到 → 返回 (path, true)
│   │       │       └─ 未找到 → 继续策略3
│   │       │
│   │       └─ 5 种后缀均未匹配 → 继续策略3
│   │
│   └─ 前提不满足（无扩展名） → 直接继续策略3
│
├─ 策略3: 模糊前缀匹配（glob）
│   │
│   ├─ matches = globPrefix(baseName, nonJSON)
│   │   └─ 遍历 nonJSON 中所有条目，收集所有小写名称以 baseName 小写开头的文件
│   │
│   ├─ len(matches) == 0 → 继续策略4
│   ├─ len(matches) == 1 → 返回 (matches[0], true)（唯一匹配，安全）
│   └─ len(matches) > 1 → 继续策略4（多结果不采用，避免歧义）
│
├─ 策略4: 截断匹配（truncation）
│   │
│   ├─ 前提: stem 中必须包含 "(数字)" 模式
│   │   │
│   │   ├─ 步骤4.1: 查找最后一个 "(" 的位置
│   │   │   ├─ idx = strings.LastIndex(stem, "(")
│   │   │   ├─ idx < 0（无左括号） → 返回 ("", false)
│   │   │   └─ idx >= 0 → 继续
│   │   │
│   │   ├─ 步骤4.2: 查找匹配的 ")" 的位置
│   │   │   ├─ closingOffset = strings.Index(stem[idx:], ")")
│   │   │   ├─ closingOffset < 0（无右括号） → 返回 ("", false)
│   │   │   └─ closingOffset >= 0 → 继续
│   │   │
│   │   ├─ 步骤4.3: 提取并验证括号内的数字
│   │   │   ├─ numStr = stem[idx+1 : idx+closingOffset]
│   │   │   ├─ strconv.Atoi(numStr) 失败 → 返回 ("", false)
│   │   │   └─ 成功 → 继续
│   │   │
│   │   ├─ 步骤4.4: 拆分 stem
│   │   │   ├─ suffix = stem[idx:]（如 "(1)"）
│   │   │   └─ nameBase = stem[:idx]（括号前的部分）
│   │   │
│   │   └─ 步骤4.5: 尝试逐步截断 nameBase
│   │       │
│   │       ├─ 循环 i 从 1 到 min(len(nameBase), 10):
│   │       │   │
│   │       │   ├─ candidate = nameBase[:len(nameBase)-i] + suffix + ext
│   │       │   │   └─ 转为小写后查找 nonJSON
│   │       │   │
│   │       │   ├─ 找到 → 返回 (path, true)
│   │       │   └─ 未找到 → 继续下一次循环（再截断 1 个字符）
│   │       │
│   │       └─ 循环结束仍未找到 → 返回 ("", false)
│   │
│   └─ 前提不满足（无括号模式） → 返回 ("", false)
│
└─ 所有策略均失败 → 返回 ("", false)
```

### 4.4 `truncationMatch` 示例

```
输入: stem = "Screenshot_2016-02-28-13-06-348", ext = ".png"
│
├─ 找到 "(" 位置 → idx = 34（假设）
├─ 找到 ")" 位置 → numStr = "1"
├─ suffix = "(1)", nameBase = "Screenshot_2016-02-28-13-06-348"
│
├─ i=1: candidate = "Screenshot_2016-02-28-13-06-34" + "(1)" + ".png"
│       = "screenshot_2016-02-28-13-06-34(1).png" → 查找 nonJSON
│       └─ 找到 → 返回
│
├─ i=2: candidate = "Screenshot_2016-02-28-13-06-3" + "(1)" + ".png"
│       └─ 未找到 → 继续
│
├─ ... 最多尝试 10 次
│
└─ 全部失败 → 返回 ("", false)
```

### 4.5 `resolveTimestamp` 执行流程

```
输入: photoPath (照片文件路径), gp (*GooglePhoto)
│
├─ 步骤1: 尝试从文件名解析时间戳
│   ├─ t, ok := parser.ParseFilenameTimestamp(filepath.Base(photoPath))
│   ├─ ok == true → 返回 t（文件名优先级最高）
│   └─ ok == false → 继续步骤2
│
├─ 步骤2: 尝试从 JSON 解析时间戳
│   ├─ gp.PhotoTakenTime.Timestamp == "" → 返回 time.Time{}（零值）
│   └─ gp.PhotoTakenTime.Timestamp != ""
│       ├─ strconv.ParseInt(gp.PhotoTakenTime.Timestamp, 10, 64) 失败 → 返回 time.Time{}
│       └─ 成功 → 返回 time.Unix(sec, 0).UTC()
│
└─ 输出: time.Time（可能为零值）
```

---

## 五、`metadata/writer.go` — 元数据写入器

### 5.1 接口定义

```go
type MetadataWriter interface {
    WriteTimestamp(filePath string, t time.Time) error
    WriteGPS(filePath string, lat, lon, alt float64) error
}
```

### 5.2 `NewWriter` 工厂函数执行流程

```
│
├─ exec.LookPath("exiftool")
│   ├─ 找到 exiftool 可执行文件 → 返回 &ExifToolWriter{exiftoolPath: path}
│   └─ 未找到（err != nil） → 返回 &NativeWriter{}
│
└─ 输出: MetadataWriter 接口实现
```

### 5.3 `ExifToolWriter.WriteTimestamp` 执行流程

```
输入: filePath, t (time.Time)
│
├─ 步骤1: 格式化时间
│   └─ formatted = t.Format("2006:01:02 15:04:05")
│       └─ 例: "2023:03:02 11:20:40"
│
├─ 步骤2: 构建 exiftool 命令
│   └─ 参数列表（共 6 个）:
│       ├─ 参数1: exiftool 可执行文件路径
│       ├─ 参数2: "-overwrite_original"（覆盖原文件，不创建备份）
│       ├─ 参数3: "-DateTimeOriginal=<formatted>"（EXIF 拍摄时间）
│       ├─ 参数4: "-CreateDate=<formatted>"（文件创建时间）
│       ├─ 参数5: "-ModifyDate=<formatted>"（文件修改时间）
│       └─ 参数6: filePath（目标文件路径）
│
├─ 步骤3: 执行命令
│   └─ out, err := cmd.CombinedOutput()
│       ├─ err != nil → 返回 fmt.Errorf("exiftool WriteTimestamp %q: %w\n%s", filePath, err, out)
│       └─ err == nil → 返回 nil
│
└─ 输出: error
```

### 5.4 `ExifToolWriter.WriteGPS` 执行流程

```
输入: filePath, lat, lon, alt
│
├─ 步骤1: 确定纬度参考
│   ├─ lat >= 0 → latRef = "N"（北纬），lat 不变
│   └─ lat < 0 → latRef = "S"（南纬），lat = -lat（取绝对值）
│
├─ 步骤2: 确定经度参考
│   ├─ lon >= 0 → lonRef = "E"（东经），lon 不变
│   └─ lon < 0 → lonRef = "W"（西经），lon = -lon（取绝对值）
│
├─ 步骤3: 确定海拔参考
│   ├─ alt >= 0 → altRef = "0"（海平面以上），alt 不变
│   └─ alt < 0 → altRef = "1"（海平面以下），alt = -alt（取绝对值）
│
├─ 步骤4: 构建 exiftool 命令
│   └─ 参数列表（共 9 个）:
│       ├─ 参数1: exiftool 可执行文件路径
│       ├─ 参数2: "-overwrite_original"
│       ├─ 参数3: "-GPSLatitude=<lat>"（纬度绝对值）
│       ├─ 参数4: "-GPSLatitudeRef=<latRef>"（N/S）
│       ├─ 参数5: "-GPSLongitude=<lon>"（经度绝对值）
│       ├─ 参数6: "-GPSLongitudeRef=<lonRef>"（E/W）
│       ├─ 参数7: "-GPSAltitude=<alt>"（海拔绝对值）
│       ├─ 参数8: "-GPSAltitudeRef=<altRef>"（0/1）
│       └─ 参数9: filePath
│
├─ 步骤5: 执行命令
│   ├─ err != nil → 返回 fmt.Errorf("exiftool WriteGPS %q: %w\n%s", filePath, err, out)
│   └─ err == nil → 返回 nil
│
└─ 输出: error
```

### 5.5 `NativeWriter` 行为

| 方法 | 实现 | 效果 |
|------|------|------|
| `WriteTimestamp(filePath, t)` | `os.Chtimes(filePath, t, t)` | 同时设置 atime 和 mtime 为 t |
| `WriteGPS(_, _, _, _)` | 直接返回 `nil` | 无操作（GPS 写入需要 exiftool） |

---

## 六、`cleaner/cleaner.go` — JSON 文件清理器

### 6.1 `Run` 执行流程

```
输入: cfg (Config{Dir, DryRun})
│
├─ 步骤1: 递归遍历目录
│   └─ filepath.WalkDir(cfg.Dir, func(path, d, err) error { ... })
│       │
│       ├─ 回调函数对每个条目执行:
│       │   │
│       │   ├─ 分支A: err != nil（遍历过程中出现错误）
│       │   │   └─ 返回 err（终止遍历）
│       │   │
│       │   ├─ 分支B: d.IsDir() == true（是目录）
│       │   │   └─ 返回 nil（跳过，继续遍历）
│       │   │
│       │   ├─ 分支C: 扩展名不是 ".json"（不区分大小写）
│       │   │   └─ 返回 nil（跳过，继续遍历）
│       │   │
│       │   └─ 分支D: 是 .json 文件
│       │       │
│       │       ├─ 子分支D1: cfg.DryRun == true
│       │       │   ├─ 打印 "would delete: <path>"
│       │       │   ├─ result.Deleted++
│       │       │   └─ 返回 nil（不实际删除）
│       │       │
│       │       └─ 子分支D2: cfg.DryRun == false
│       │           ├─ os.Remove(path)
│       │           │   ├─ err != nil → result.Failed++
│       │           │   └─ err == nil → result.Deleted++
│       │           └─ 返回 nil
│       │
│       └─ 遍历结束
│
├─ 步骤2: 返回结果
│   └─ return result, err（err 为 WalkDir 的最终错误，可能为 nil）
│
└─ 输出: (Result{Deleted, Failed}, error)
```

### 6.2 分支决策表

| 条件组合 | 行为 | 计数器变化 |
|----------|------|-----------|
| 遍历错误 | 终止遍历 | 无变化 |
| 是目录 | 跳过 | 无变化 |
| 非 .json 文件 | 跳过 | 无变化 |
| .json 文件 + DryRun=true | 打印路径，不删除 | Deleted++ |
| .json 文件 + DryRun=false + 删除成功 | 实际删除 | Deleted++ |
| .json 文件 + DryRun=false + 删除失败 | 保留文件 | Failed++ |

---

## 七、`renamer/renamer.go` — 文件重命名器

### 7.1 扩展名集合

**图像扩展名（16 种）**：`jpg`, `jpeg`, `png`, `gif`, `bmp`, `tiff`, `tif`, `heic`, `heif`, `webp`, `avif`, `raw`, `cr2`, `nef`, `arw`, `dng`

**视频扩展名（21 种）**：`mp4`, `mov`, `avi`, `mkv`, `wmv`, `flv`, `3gp`, `m4v`, `webm`, `mpg`, `mpeg`, `asf`, `rm`, `rmvb`, `vob`, `ts`, `mts`, `m2ts`

### 7.2 `Run` 执行流程

```
输入: cfg (Config{Dir, DryRun})
│
├─ 步骤1: 读取目录（非递归）
│   └─ entries, err := os.ReadDir(cfg.Dir)
│       ├─ err != nil → 返回 (Result{}, fmt.Errorf("read dir: %w", err))
│       └─ err == nil → 继续
│
├─ 步骤2: 遍历每个条目
│   │
│   ├─ 分支A: e.IsDir() == true → 跳过（continue）
│   │
│   ├─ 分支B: 扩展名不在 imageExts 或 videoExts 中 → 跳过
│   │   └─ ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
│   │   └─ !imageExts[ext] && !videoExts[ext] → continue
│   │
│   ├─ 分支C: 获取文件信息失败
│   │   ├─ info, err := e.Info()
│   │   ├─ err != nil → result.Errors++ → continue
│   │   └─ err == nil → 继续
│   │
│   └─ 分支D: 正常处理
│       │
│       ├─ mtime = info.ModTime()
│       ├─ prefix = "VID"（如果扩展名在 videoExts 中）或 "IMG"（否则）
│       ├─ newName = generateName(cfg.Dir, prefix, mtime, "."+ext, name)
│       │
│       ├─ 子分支D1: newName == name（名称未变化）
│       │   └─ result.Skipped++ → continue
│       │
│       ├─ 子分支D2: cfg.DryRun == true
│       │   ├─ 打印 "would rename: <old> -> <new>"
│       │   └─ result.Renamed++ → continue
│       │
│       └─ 子分支D3: cfg.DryRun == false
│           ├─ os.Rename(fullPath, filepath.Join(cfg.Dir, newName))
│           │   ├─ err != nil → result.Errors++
│           │   └─ err == nil → result.Renamed++
│           └─ continue
│
└─ 输出: (Result{Renamed, Skipped, Errors}, nil)
```

### 7.3 `generateName` 执行流程

```
输入: dir, prefix ("IMG"/"VID"), t (time.Time), ext (如 ".jpg"), currentName
│
├─ 循环 i 从 0 到 998（共 999 次尝试）:
│   │
│   ├─ candidateTime = t.Add(time.Duration(i) * time.Second)
│   ├─ candidate = prefix + candidateTime.Format("20060102150405") + ext
│   │   └─ 例: "IMG20230302112040.jpg"（i=0）
│   │          "IMG20230302112041.jpg"（i=1）
│   │
│   ├─ 分支A: candidate == currentName
│   │   └─ 返回 currentName（名称已正确，无需重命名）
│   │
│   ├─ 分支B: os.Stat(candidate) 返回 os.IsNotExist(err) == true
│   │   └─ 返回 candidate（找到不冲突的新名称）
│   │
│   └─ 分支C: 文件已存在 → 继续下一次循环（i++）
│
├─ 循环结束（999 次尝试均未找到合适名称）
│   └─ 返回 currentName（保持原名）
│
└─ 输出: string（新文件名或原文件名）
```

### 7.4 分支决策表

| 条件组合 | 行为 | 计数器变化 |
|----------|------|-----------|
| 是目录 | 跳过 | 无变化 |
| 扩展名不在白名单中 | 跳过 | 无变化 |
| 获取文件信息失败 | 跳过 | Errors++ |
| 生成的名称与原名称相同 | 跳过 | Skipped++ |
| DryRun=true | 打印映射，不重命名 | Renamed++ |
| DryRun=false + 重命名成功 | 实际重命名 | Renamed++ |
| DryRun=false + 重命名失败 | 保留原名 | Errors++ |
| 999 次尝试均未找到不冲突名称 | 保持原名 | Skipped++ |

---

## 八、`organizer/organizer.go` — 文件分类整理器

### 8.1 模式定义

```go
type Mode string

const (
    ModeCamera     Mode = "camera"     // 相机照片
    ModeScreenshot Mode = "screenshot" // 截图
    ModeWechat     Mode = "wechat"     // 微信导出
)
```

### 8.2 前缀与模式匹配规则

**相机前缀（8 种）**：`WP_`, `IMG_`, `IMG`, `VID_`, `VID`, `P_`, `PXL_`, `DSC_`

**相机日期模式**：`^\d{8}_\d{6}`（如 `20151120_120004`）

### 8.3 `Run` 执行流程

```
输入: cfg (Config{Mode, SourceDirs, DestDir, DryRun, Recursive})
│
├─ 步骤1: 创建目标目录
│   └─ os.MkdirAll(cfg.DestDir, 0o755)
│       ├─ err != nil → 返回 (Result{}, err)
│       └─ err == nil → 继续
│
├─ 步骤2: 遍历所有源目录
│   └─ 对每个 srcDir in cfg.SourceDirs:
│       └─ walkDir(srcDir, cfg, &result)
│           ├─ err != nil → 返回 (Result{}, err)
│           └─ err == nil → 继续下一个源目录
│
└─ 输出: (result, nil)
```

### 8.4 `walkDir` 执行流程

```
输入: dir, cfg, result
│
├─ 步骤1: 读取目录（非递归）
│   └─ entries, err := os.ReadDir(dir)
│       ├─ err != nil → 返回 err
│       └─ err == nil → 继续
│
├─ 步骤2: 遍历每个条目
│   │
│   ├─ 分支A: e.IsDir() == true 且 cfg.Recursive == true
│   │   └─ 递归调用 walkDir(fullPath, cfg, result)
│   │       ├─ err != nil → 返回 err
│   │       └─ err == nil → 继续
│   │
│   ├─ 分支B: e.IsDir() == true 且 cfg.Recursive == false
│   │   └─ 跳过（不进入子目录）
│   │
│   ├─ 分支C: e.IsDir() == false 且 matches(e.Name(), cfg.Mode) == false
│   │   └─ 跳过（文件不符合当前模式）
│   │
│   └─ 分支D: e.IsDir() == false 且 matches(e.Name(), cfg.Mode) == true
│       └─ moveFile(fullPath, e.Name(), cfg, result)
│
└─ 输出: nil
```

### 8.5 `matches` 函数的完整分支逻辑

```
输入: name (文件名), mode (Mode)
│
├─ 预处理:
│   ├─ ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
│   ├─ lowerName = strings.ToLower(name)
│   └─ baseName = strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
│
├─ 根据 mode 分支:
│   │
│   ├─ 情况A: mode == ModeCamera
│   │   │
│   │   ├─ 前提: ext 必须在 imageExts 或 videoExts 中
│   │   │   ├─ 不在 → 返回 false
│   │   │   └─ 在 → 继续
│   │   │
│   │   ├─ 条件1: name 以 8 种相机前缀之一开头（不区分大小写）
│   │   │   ├─ "WP_" → 返回 true
│   │   │   ├─ "IMG_" → 返回 true
│   │   │   ├─ "IMG" → 返回 true
│   │   │   ├─ "VID_" → 返回 true
│   │   │   ├─ "VID" → 返回 true
│   │   │   ├─ "P_" → 返回 true
│   │   │   ├─ "PXL_" → 返回 true
│   │   │   └─ "DSC_" → 返回 true
│   │   │
│   │   └─ 条件2: baseName 匹配正则 ^\d{8}_\d{6}
│   │       └─ 匹配 → 返回 true
│   │
│   ├─ 情况B: mode == ModeScreenshot
│   │   │
│   │   ├─ 前提: ext 必须在 imageExts 中（仅限图像）
│   │   │   ├─ 不在 → 返回 false
│   │   │   └─ 在 → 继续
│   │   │
│   │   └─ 条件: lowerName 包含 "screenshot" 子串
│   │       ├─ 包含 → 返回 true
│   │       └─ 不包含 → 返回 false
│   │
│   ├─ 情况C: mode == ModeWechat
│   │   │
│   │   ├─ 前提: ext 必须在 imageExts 或 videoExts 中
│   │   │   ├─ 不在 → 返回 false
│   │   │   └─ 在 → 继续
│   │   │
│   │   └─ 条件: lowerName 以 "mmexport" 开头
│   │       ├─ 是 → 返回 true
│   │       └─ 否 → 返回 false
│   │
│   └─ 情况D: mode 为其他值（未知模式）
│       └─ 返回 false
│
└─ 输出: bool
```

### 8.6 `matches` 完整决策表

| mode | 扩展名条件 | 名称条件 | 结果 |
|------|-----------|----------|------|
| ModeCamera | 不在 imageExts/videoExts 中 | — | false |
| ModeCamera | 在 imageExts/videoExts 中 | 以 8 种相机前缀之一开头 | true |
| ModeCamera | 在 imageExts/videoExts 中 | baseName 匹配 `^\d{8}_\d{6}` | true |
| ModeCamera | 在 imageExts/videoExts 中 | 不满足以上名称条件 | false |
| ModeScreenshot | 不在 imageExts 中 | — | false |
| ModeScreenshot | 在 imageExts 中 | 包含 "screenshot" | true |
| ModeScreenshot | 在 imageExts 中 | 不包含 "screenshot" | false |
| ModeWechat | 不在 imageExts/videoExts 中 | — | false |
| ModeWechat | 在 imageExts/videoExts 中 | 以 "mmexport" 开头 | true |
| ModeWechat | 在 imageExts/videoExts 中 | 不以 "mmexport" 开头 | false |
| 其他 | — | — | false |

### 8.7 `moveFile` 执行流程

```
输入: src (源路径), name (文件名), cfg, result
│
├─ 步骤1: 解析目标路径
│   └─ destPath = resolveDestPath(cfg.DestDir, name)
│
├─ 步骤2: DryRun 检查
│   ├─ cfg.DryRun == true → result.Moved++ → 返回 nil
│   └─ cfg.DryRun == false → 继续
│
├─ 步骤3: 尝试移动文件
│   ├─ os.Rename(src, destPath)
│   │   ├─ err == nil → result.Moved++ → 返回 nil
│   │   └─ err != nil → 进入回退逻辑
│   │
│   └─ 回退: 复制后删除
│       ├─ copyFile(src, destPath)
│       │   ├─ err != nil → result.Skipped++ → 返回 nil
│       │   └─ err == nil → 继续
│       │
│       └─ os.Remove(src)
│           ├─ err != nil → （忽略，已复制成功）
│           └─ err == nil → 继续
│       │
│       └─ result.Moved++ → 返回 nil
│
└─ 输出: error（始终返回 nil）
```

### 8.8 `resolveDestPath` 执行流程

```
输入: destDir, name
│
├─ target = filepath.Join(destDir, name)
│
├─ 检查 target 是否存在:
│   ├─ os.Stat(target) 返回 os.IsNotExist(err) == true
│   │   └─ 返回 target（无冲突）
│   │
│   └─ 文件已存在 → 生成防冲突名称
│       ├─ now = time.Now()
│       ├─ stem = 去掉扩展名的部分
│       ├─ ext = 扩展名
│       ├─ target = stem + "_" + now.Format("20060102150405") + ext
│       └─ 返回 target
│
└─ 输出: string（目标文件路径）
```

### 8.9 `copyFile` 执行流程

```
输入: src, dst
│
├─ 步骤1: 读取源文件内容
│   └─ data, err := os.ReadFile(src)
│       ├─ err != nil → 返回 err
│       └─ err == nil → 继续
│
├─ 步骤2: 获取源文件权限
│   └─ info, err := os.Stat(src)
│       ├─ err != nil → 返回 err
│       └─ err == nil → 继续
│
├─ 步骤3: 写入目标文件
│   └─ os.WriteFile(dst, data, info.Mode())
│       ├─ err != nil → 返回 err
│       └─ err == nil → 返回 nil
│
└─ 输出: error
```

---

## 九、`progress/logger.go` — 进度条与日志输出

### 9.1 日志函数

| 函数 | 输出格式 | 用途 |
|------|----------|------|
| `Info(format, args...)` | `ℹ️  <format>` | 一般信息 |
| `Success(format, args...)` | `✅ <format>` | 成功提示 |
| `Warning(format, args...)` | `⚠️  <format>` | 警告提示 |
| `Error(format, args...)` | `❌ <format>` | 错误提示 |

### 9.2 `PrintProgress` 执行流程

```
输入: current, total
│
├─ 分支A: total == 0 → 直接返回（不输出）
│
└─ 分支B: total > 0
    ├─ pct = current * 100 / total（整数除法）
    ├─ barWidth = 20
    ├─ filled = pct * 20 / 100（已填充的 + 号数量）
    ├─ bar = "+" 重复 filled 次 + "-" 重复 (20-filled) 次
    └─ 输出: "\r🔄 [<bar>] <pct>% (<current>/<total>)"
        └─ 使用 \r 实现光标回退，覆盖上一次输出
```

### 9.3 进度条输出示例

| current | total | pct | 输出 |
|---------|-------|-----|------|
| 0 | 100 | 0 | `🔄 [--------------------] 0% (0/100)` |
| 25 | 100 | 25 | `🔄 [+++++---------------] 25% (25/100)` |
| 50 | 100 | 50 | `🔄 [++++++++++----------] 50% (50/100)` |
| 100 | 100 | 100 | `🔄 [++++++++++++++++++++] 100% (100/100)` |

---

## 十、跨包调用关系图

```
main.go (外部)
│
├──→ parser.ParseFilenameTimestamp(filename)        # 从文件名提取时间戳
│       └──→ parseDateTime8_6(date, timeStr)
│               └──→ parseComponents(y, m, d, H, M, S)
│
├──→ parser.ParseIMGVIDFilename(filename)            # IMG/VID 专用时间戳提取
│       └──→ parseDateTime8_6(date, timeStr)         # 复用 timestamp.go 的函数
│
├──→ matcher.MatchAll(dir)                           # JSON-照片配对
│       ├──→ parseJSON(path)                         # 解析 JSON 文件
│       ├──→ deriveBaseName(jsonName)                # 推导照片文件名
│       ├──→ findPhoto(baseName, nonJSON, dir)       # 4 种策略查找照片
│       │       ├──→ globPrefix(prefix, nonJSON)     # 模糊前缀匹配
│       │       └──→ truncationMatch(stem, ext, m)   # 截断匹配
│       └──→ resolveTimestamp(photoPath, gp)         # 解析时间戳
│               └──→ parser.ParseFilenameTimestamp() # 优先从文件名提取
│
├──→ metadata.NewWriter()                            # 自动选择写入器
│       ├──→ ExifToolWriter.WriteTimestamp()         # 使用 exiftool 写入时间
│       ├──→ ExifToolWriter.WriteGPS()               # 使用 exiftool 写入 GPS
│       ├──→ NativeWriter.WriteTimestamp()           # 使用 os.Chtimes 写入时间
│       └──→ NativeWriter.WriteGPS()                 # 无操作
│
├──→ cleaner.Run(cfg)                                # 递归删除 JSON 文件
│
├──→ renamer.Run(cfg)                                # 按时间重命名文件
│       └──→ generateName(...)                       # 生成不冲突的新名称
│
├──→ organizer.Run(cfg)                              # 按模式分类整理文件
│       ├──→ walkDir(...)                            # 遍历目录
│       │       └──→ matches(name, mode)             # 判断文件是否匹配模式
│       └──→ moveFile(...)                           # 移动/复制文件
│               └──→ resolveDestPath(...)            # 解析目标路径（防冲突）
│
└──→ progress.PrintProgress(current, total)           # 打印进度条
    progress.Info/Success/Warning/Error()             # 打印日志
```
# `internal/matcher/json_matcher.go` 详细实现逻辑分析

---

## 一、文件总览

```
internal/matcher/
└── json_matcher.go    # JSON-照片配对器（493 行）
```

**依赖关系**：仅依赖 `parser` 包（调用 `ParseFilenameTimestamp`），其余均为标准库。

---

## 二、核心数据结构

### 2.1 `GooglePhoto` — JSON 侧车文件解析结构

```go
type GooglePhoto struct {
    PhotoTakenTime struct {
        Timestamp string `json:"timestamp"`
    } `json:"photoTakenTime"`
    GeoData struct {
        Latitude  float64 `json:"latitude"`
        Longitude float64 `json:"longitude"`
        Altitude  float64 `json:"altitude"`
    } `json:"geoData"`
    CameraMake  string `json:"cameraMake"`
    CameraModel string `json:"cameraModel"`
}
```

**字段说明**：

| JSON 路径 | Go 字段 | 数据类型 | 用途 |
|-----------|---------|----------|------|
| `.photoTakenTime.timestamp` | `PhotoTakenTime.Timestamp` | 字符串（Unix 秒级时间戳） | 拍摄时间 |
| `.geoData.latitude` | `GeoData.Latitude` | 浮点数 | 纬度（十进制度） |
| `.geoData.longitude` | `GeoData.Longitude` | 浮点数 | 经度（十进制度） |
| `.geoData.altitude` | `GeoData.Altitude` | 浮点数 | 海拔（米） |
| `.cameraMake` | `CameraMake` | 字符串 | 设备制造商（如 `"Apple"`） |
| `.cameraModel` | `CameraModel` | 字符串 | 设备型号（如 `"iPhone 14 Pro"`） |

### 2.2 `JSONLookupResult` — 查找结果结构

```go
type JSONLookupResult struct {
    JSONFile    string    // 匹配的 JSON 文件完整路径
    Timestamp   time.Time // 提取的拍摄时间（解析失败时为零值）
    Lat         float64   // 纬度
    Lon         float64   // 经度
    Alt         float64   // 海拔
    CameraMake  string    // 设备制造商
    CameraModel string    // 设备型号
}
```

### 2.3 包级变量

**编辑后缀列表（15 种语言）**：

| 序号 | 语言 | 后缀 |
|------|------|------|
| 1 | 中文（简体） | `-已修改` |
| 2 | 中文（简体） | `-编辑` |
| 3 | 中文（简体） | `-修改` |
| 4 | 英语/美式 | `-edited` |
| 5 | 英语/美式 | `-effects` |
| 6 | 英语/美式 | `-smile` |
| 7 | 英语/美式 | `-mix` |
| 8 | 波兰语 | `-edytowane` |
| 9 | 德语 | `-bearbeitet` |
| 10 | 荷兰语 | `-bewerkt` |
| 11 | 日语 | `-編集済み` |
| 12 | 意大利语 | `-modificato` |
| 13 | 法语 | `-modifié` |
| 14 | 西班牙语 | `-ha editado` |
| 15 | 加泰罗尼亚语 | `-editat` |

**Supplemental 后缀列表（5 种截断变体）**：

| 序号 | 后缀 | 说明 |
|------|------|------|
| 1 | `supplemental-met` | 51 字符限制截断 |
| 2 | `supplemental-metadata` | 完整形式 |
| 3 | `supplemen` | 进一步截断 |
| 4 | `supp` | 短截断 |
| 5 | `s` | 最短截断 |

**正则表达式**：

| 变量 | 正则 | 用途 |
|------|------|------|
| `bracketSwapRegex` | `\(\d+\)\.` | 匹配 `(数字).` 模式，用于括号位置交换 |
| `supplementalRegex` | `^(.+)\.supp[a-z]*\.json$` | 匹配 supplemental 后缀的 JSON 文件（未直接使用，仅作声明） |

---

## 三、`JSONForFile` — 顶层查找函数

### 3.1 函数签名

```go
func JSONForFile(photoPath string) *JSONLookupResult
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `photoPath` | `string` | 照片文件完整路径 |

| 返回值 | 类型 | 说明 |
|--------|------|------|
| `*JSONLookupResult` | 指针 | 找到 JSON 时返回结果，否则返回 `nil` |

### 3.2 完整执行流程

```
输入: photoPath (照片文件路径)
│
├─ 阶段1: 提取目录和文件名
│   ├─ dir = filepath.Dir(photoPath)
│   └─ name = filepath.Base(photoPath)
│
├─ 阶段2: 执行 5 种基础变换方法
│   │
│   ├─ 方法列表:
│   │   1. methodIdentity — 无变换
│   │   2. methodShortenName — 51 字符截断
│   │   3. methodBracketSwap — 括号位置交换
│   │   4. methodRemoveExtra — 移除编辑后缀
│   │   5. methodNoExtension — 移除扩展名
│   │
│   └─ 对每种方法:
│       ├─ transformedName = method(name)
│       ├─ jsonPath = dir + "/" + transformedName + ".json"
│       ├─ os.Stat(jsonPath) 检查文件是否存在
│       │   ├─ 存在 → 调用 parseJSONLookup(jsonPath) → 返回结果
│       │   └─ 不存在 → 继续下一种方法
│
├─ 阶段3: 双点号 JSON 命名（Strategy 4b）
│   ├─ doubleDotPath = dir + "/" + name + "..json"
│   ├─ os.Stat(doubleDotPath) 检查
│   │   ├─ 存在 → 调用 parseJSONLookup(doubleDotPath) → 返回结果
│   │   └─ 不存在 → 继续
│
├─ 阶段4: Supplemental 后缀匹配（Strategy 5）
│   │
│   ├─ Step 5a: 对原始名尝试 5 种已知后缀
│   │   └─ 对每个 suffix in supplementalSuffixes:
│   │       ├─ jsonPath = dir + "/" + name + "." + suffix + ".json"
│   │       ├─ 存在 → 返回结果
│   │       └─ 不存在 → 继续
│   │
│   ├─ Step 5a2: 对 RemoveExtra 变换后的名尝试 5 种已知后缀
│   │   ├─ cleanedName = methodRemoveExtra(name)
│   │   ├─ cleanedName != name ?
│   │   │   ├─ 是 → 对每个 suffix 尝试 cleanedName + "." + suffix + ".json"
│   │   │   │   ├─ 存在 → 返回结果
│   │   │   │   └─ 不存在 → 继续
│   │   │   └─ 否 → 跳过
│   │
│   ├─ Step 5b: 正则兜底扫描（原始名）
│   │   ├─ pattern = "^" + QuoteMeta(name) + "\.su[a-z-]*(\(\d+\))?\.json$"
│   │   ├─ os.ReadDir(dir) 扫描目录
│   │   └─ 对每个文件:
│   │       ├─ pattern.MatchString(e.Name()) ?
│   │       │   ├─ 是 → 返回结果
│   │       │   └─ 否 → 继续
│   │
│   ├─ Step 5b2: 正则兜底扫描（cleaned 名）
│   │   ├─ cleanedName != name ?
│   │   │   ├─ 是 → pattern = "^" + QuoteMeta(cleanedName) + "\.su[a-z-]*(\(\d+\))?\.json$"
│   │   │   │   └─ 扫描目录，匹配则返回
│   │   │   └─ 否 → 跳过
│   │
│   └─ Step 5c: 编号重复文件特殊处理
│       ├─ 照片名匹配 "^(.+)\((\d+)\)\.(\w+)$" ?
│       │   ├─ 是 → 提取 baseName, num, ext
│       │   │   ├─ numPattern = "^" + baseName + "\." + ext + "\.su[a-z-]*\(" + num + "\)\.json$"
│       │   │   └─ 扫描目录，匹配则返回
│       │   └─ 否 → 跳过
│
└─ 所有策略均失败 → 返回 nil
```

### 3.3 策略执行决策表

| 输入照片名 | 匹配的策略 | 查找的 JSON 路径 |
|-----------|-----------|-----------------|
| `photo.jpg` | 1. Identity | `photo.jpg.json` |
| `超长文件名...jpg`（>46 字符） | 2. ShortenName | 截断后的名称 + `.json` |
| `image(11).jpg` | 3. BracketSwap | `image.jpg(11).json` |
| `photo-edited.jpg` | 4. RemoveExtra | `photo.jpg.json` |
| `photo.jpg` | 4b. Double-dot | `photo.jpg..json` |
| `photo.jpg` | 5a. Supplemental | `photo.jpg.supplemental-metadata.json` 等 |
| `IMG_20210629_114736-已修改.jpg` | 5a2. Cleaned + Supplemental | `IMG_20210629_114736.jpg.supplemental-metadata.json` |
| `photo.jpg` | 5b. Regex | 扫描 `photo.jpg.su*.json` |
| `IMG_20210629_114736-已修改.jpg` | 5b2. Cleaned Regex | 扫描 `IMG_20210629_114736.jpg.su*.json` |
| `IMG20240405102259(1).heic` | 5c. Numbered | `IMG20240405102259.heic.supplemental-metadata(1).json` |
| `20030616.jpg` | 5. NoExtension | `20030616.json` |

---

## 四、5 种基础变换方法详解

### 4.1 `methodIdentity` — 无变换

```go
func methodIdentity(filename string) string { return filename }
```

| 输入 | 输出 |
|------|------|
| `photo.jpg` | `photo.jpg` |
| 任意值 | 原样返回 |

### 4.2 `methodShortenName` — 51 字符截断

```go
func methodShortenName(filename string) string {
    if len(filename)+len(".json") > 51 {
        return filename[:46]
    }
    return filename
}
```

**分支清单**：

| 条件 | 操作 | 示例 |
|------|------|------|
| `len(filename) + 5 > 51` | 截断到前 46 字符 | `very_long_filename...jpg`（67 字符）→ `very_long_filename...forty_six`（46 字符） |
| `len(filename) + 5 <= 51` | 原样返回 | `photo.jpg`（9 字符）→ `photo.jpg` |

### 4.3 `methodBracketSwap` — 括号位置交换

```go
func methodBracketSwap(filename string) string {
    matches := bracketSwapRegex.FindAllStringIndex(filename, -1)
    if len(matches) == 0 { return filename }
    lastMatch := matches[len(matches)-1]
    bracketWithDot := filename[lastMatch[0]:lastMatch[1]]  // "(11)."
    bracket := strings.TrimSuffix(bracketWithDot, ".")     // "(11)"
    withoutBracket := filename[:lastMatch[0]] + filename[lastMatch[0]+len(bracket):]
    return withoutBracket + bracket
}
```

**分支清单**：

| 条件 | 操作 | 示例 |
|------|------|------|
| 无 `(数字).` 匹配 | 原样返回 | `normal.jpg` → `normal.jpg` |
| 有匹配 | 取最后一个匹配，将 `(N)` 从扩展名前移到末尾 | `image(11).jpg` → `image.jpg(11)` |
| 多个匹配 | 取最后一个 | `image(3).(2)(3).jpg` → `image(3).(2).jpg(3)` |

### 4.4 `methodRemoveExtra` — 移除编辑后缀

```go
func methodRemoveExtra(filename string) string {
    filename = nfcNormalize(filename)
    for _, extra := range extraFormats {
        if strings.Contains(filename, extra) {
            return replaceLast(filename, extra, "")
        }
    }
    return filename
}
```

**分支清单**：

| 条件 | 操作 | 示例 |
|------|------|------|
| 包含 `-已修改` | 移除最后一次出现 | `photo-已修改.jpg` → `photo.jpg` |
| 包含 `-编辑` | 移除最后一次出现 | `photo-编辑.jpg` → `photo.jpg` |
| 包含 `-修改` | 移除最后一次出现 | `photo-修改.jpg` → `photo.jpg` |
| 包含 `-edited` | 移除最后一次出现 | `photo-edited.jpg` → `photo.jpg` |
| ...（共 15 种后缀） | 同上 | ... |
| 不包含任何后缀 | 原样返回 | `normal.jpg` → `normal.jpg` |

### 4.5 `methodNoExtension` — 移除扩展名

```go
func methodNoExtension(filename string) string {
    ext := filepath.Ext(filename)
    return strings.TrimSuffix(filename, ext)
}
```

| 输入 | 输出 |
|------|------|
| `20030616.jpg` | `20030616` |
| `archive.tar.gz` | `archive.tar` |
| `noext` | `noext` |

---

## 五、Supplemental 策略详解

### 5.1 Step 5a — 已知后缀精确匹配

对原始文件名尝试 5 种已知 supplemental 后缀：

| 尝试顺序 | 构造的 JSON 路径 |
|---------|-----------------|
| 1 | `name + ".supplemental-met.json"` |
| 2 | `name + ".supplemental-metadata.json"` |
| 3 | `name + ".supplemen.json"` |
| 4 | `name + ".supp.json"` |
| 5 | `name + ".s.json"` |

### 5.2 Step 5a2 — Cleaned 名的已知后缀匹配

对 `methodRemoveExtra` 变换后的文件名尝试同样的 5 种后缀：

| 输入 | cleanedName | 尝试的 JSON 路径 |
|------|-------------|-----------------|
| `IMG_20210629_114736-已修改.jpg` | `IMG_20210629_114736.jpg` | `IMG_20210629_114736.jpg.supplemental-metadata.json` 等 |

### 5.3 Step 5b — 正则兜底扫描（原始名）

正则：`^<escapedName>\.su[a-z-]*(\(\d+\))?\.json$`

覆盖范围：
- `.su.json`（2 字母）
- `.sup.json`（3 字母）
- `.supplemental.json`（13 字母）
- `.supplemental-metadata.json`（19 字母）
- `.supplemental-met.json`（16 字母，含连字符）
- 带编号：`.su(1).json`, `.supplemental-metadata(1).json`

### 5.4 Step 5b2 — 正则兜底扫描（cleaned 名）

同 Step 5b，但使用 `methodRemoveExtra` 变换后的文件名。

### 5.5 Step 5c — 编号重复文件特殊处理

处理 `(N)` 从照片名移动到 JSON 后缀的场景：

| 照片名 | 提取的 baseName | 提取的 num | 提取的 ext | 匹配的 JSON 模式 |
|--------|----------------|-----------|-----------|-----------------|
| `IMG20240405102259(1).heic` | `IMG20240405102259` | `1` | `heic` | `IMG20240405102259.heic.su*(1).json` |

---

## 六、辅助函数详解

### 6.1 `parseJSONLookup` — JSON 文件解析

```go
func parseJSONLookup(jsonPath string) *JSONLookupResult
```

**执行流程**：

```
输入: jsonPath
│
├─ os.ReadFile(jsonPath)
│   ├─ err != nil → 返回 nil
│   └─ err == nil → 继续
│
├─ json.Unmarshal(data, &gp)
│   ├─ err != nil → 返回 nil
│   └─ err == nil → 继续
│
├─ 构建 JSONLookupResult:
│   ├─ JSONFile = jsonPath
│   ├─ Lat = gp.GeoData.Latitude
│   ├─ Lon = gp.GeoData.Longitude
│   ├─ Alt = gp.GeoData.Altitude
│   ├─ CameraMake = gp.CameraMake
│   └─ CameraModel = gp.CameraModel
│
├─ gp.PhotoTakenTime.Timestamp != "" ?
│   ├─ 是 → strconv.ParseInt(...)
│   │   ├─ 成功 → result.Timestamp = time.Unix(sec, 0).UTC()
│   │   └─ 失败 → Timestamp 保持零值
│   └─ 否 → Timestamp 保持零值
│
└─ 返回 result
```

### 6.2 `ResolveTimestamp` — 时间戳解析

```go
func ResolveTimestamp(photoPath string, gp *GooglePhoto) time.Time
```

**优先级**：

| 优先级 | 来源 | 条件 |
|--------|------|------|
| 1 | 文件名解析（`parser.ParseFilenameTimestamp`） | 匹配 8 种时间戳格式 |
| 2 | JSON `.photoTakenTime.timestamp` | 文件名无法解析时回退 |
| 3 | 零值 `time.Time{}` | 两者均失败 |

### 6.3 `replaceLast` — 替换最后一次出现

```go
func replaceLast(s, old, new string) string
```

| 输入 (s, old, new) | 输出 |
|---------------------|------|
| `("my-edited-photo-edited.jpg", "-edited", "")` | `my-edited-photo.jpg` |
| `("normal.jpg", "-edited", "")` | `normal.jpg` |
| `("a-b-c.jpg", "-", "_")` | `a-b_c.jpg` |

### 6.4 `nfcNormalize` — NFC Unicode 规范化

**执行流程**：

```
输入: s
│
├─ 检查是否纯 ASCII（所有字节 <= 127）
│   ├─ 是 → 原样返回
│   └─ 否 → 继续
│
├─ 遍历 rune 序列
│   ├─ 当前 rune + 下一个 rune 是组合标记（unicode.Mn）?
│   │   ├─ 是 → composePair(base, combining)
│   │   │   ├─ 返回组合字符 → 添加到结果，跳过 2 个 rune
│   │   │   └─ 返回 0 → 添加当前 rune，继续
│   │   └─ 否 → 添加当前 rune，继续
│
└─ 返回组合后的字符串
```

### 6.5 `composePair` — 字符组合

支持的组合（仅限 U+0301 组合锐音符）：

| 基础字符 | 组合后 | 示例 |
|---------|--------|------|
| `e` | `é` (U+00E9) | `modifi\u0301` → `modifié` |
| `E` | `É` (U+00C9) | — |
| `a` | `á` (U+00E1) | — |
| `A` | `Á` (U+00C1) | — |
| `i` | `í` (U+00ED) | — |
| `I` | `Í` (U+00CD) | — |
| `o` | `ó` (U+00F3) | — |
| `O` | `Ó` (U+00D3) | — |
| `u` | `ú` (U+00FA) | — |
| `U` | `Ú` (U+00DA) | — |
| `c` | `ć` (U+0107) | — |
| `C` | `Ć` (U+0106) | — |
| `n` | `ń` (U+0144) | — |
| `N` | `Ń` (U+0143) | — |
| `s` | `ś` (U+015B) | — |
| `S` | `Ś` (U+015A) | — |
| `z` | `ź` (U+017A) | — |
| `Z` | `Ź` (U+0179) | — |
| `l` | `ĺ` (U+013A) | — |
| `L` | `Ĺ` (U+0139) | — |
| `r` | `ŕ` (U+0155) | — |
| `R` | `Ŕ` (U+0154) | — |

---

## 七、与 Dart 参考文档的架构差异

| 维度 | Go 实现 | Dart 参考 |
|------|---------|----------|
| **查找方向** | 从照片出发，查找对应 JSON | 从照片出发，查找对应 JSON |
| **策略数量** | 9+ 种（5 种基础 + 双点号 + 5 种 supplemental + 正则 + 编号处理） | 5-7 种（取决于 tryhard） |
| **tryhard 参数** | 无（始终执行全部策略） | 有（普通模式 5 种，激进模式 7 种） |
| **编辑后缀** | 15 种（含中文 3 种） | 12 种（无中文） |
| **Supplemental 处理** | 完整实现（已知后缀 + 正则兜底 + cleaned 名 + 编号处理） | 无 |
| **双点号 JSON** | 支持（`filename.ext..json`） | 无 |
| **NFC 规范化** | 内置简化实现（22 种组合） | 使用 `unorm_dart` 库 |

---

## 八、完整策略链（按执行顺序）

| 序号 | 策略 | 变换逻辑 | 示例 |
|------|------|---------|------|
| 1 | Identity | 无变换 | `photo.jpg` → `photo.jpg.json` |
| 2 | ShortenName | 超长截断到 46 字符 | `very_long...jpg` → `very_long...46.json` |
| 3 | BracketSwap | `(N).ext` → `.ext(N)` | `image(11).jpg` → `image.jpg(11).json` |
| 4 | RemoveExtra | 移除 15 种编辑后缀 | `photo-edited.jpg` → `photo.jpg.json` |
| 5 | NoExtension | 移除扩展名 | `20030616.jpg` → `20030616.json` |
| 4b | Double-dot | 双点号 JSON | `photo.jpg` → `photo.jpg..json` |
| 5a | Supplemental（原始名） | 5 种已知后缀 | `photo.jpg` → `photo.jpg.supplemental-metadata.json` |
| 5a2 | Supplemental（cleaned 名） | cleaned 名 + 5 种后缀 | `IMG_...-已修改.jpg` → `IMG_....jpg.supplemental-metadata.json` |
| 5b | 正则兜底（原始名） | 扫描 `name.su*.json` | 扫描目录 |
| 5b2 | 正则兜底（cleaned 名） | 扫描 `cleaned.su*.json` | 扫描目录 |
| 5c | 编号重复处理 | `base(N).ext` → `base.ext.su*(N).json` | `IMG...(1).heic` → `IMG.heic.supplemental-metadata(1).json` |

# JSON 文件查找详细实现逻辑

## 概述

`json_extractor.dart` 负责从 Google Takeout 的 JSON 侧车文件中提取照片拍摄时间。核心难点在于：**原始文件名经过 Google 处理后可能发生变化**，导致 `.json` 文件名与图片文件名不完全匹配。该模块实现了 **7 步降级查找策略**，逐步尝试各种文件名变换来定位对应的 JSON 文件。

---

## 一、顶层函数 `jsonExtractor`

### 函数签名
```dart
Future<DateTime?> jsonExtractor(File file, {bool tryhard = false})
```

### 参数说明
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `file` | `File` | 是 | 无 | 目标图片文件 |
| `tryhard` | `bool` | 否 | `false` | 是否启用激进模式（启用额外的第 6、7 步查找策略） |

### 返回值
| 返回值 | 说明 |
|--------|------|
| `DateTime` | 成功提取到拍摄时间 |
| `null` | 查找失败（JSON 文件不存在）或解析失败（JSON 格式错误/编码问题/标签缺失） |

### 执行流程

```
步骤 1: 调用 _jsonForFile(file, tryhard: tryhard) 查找对应的 JSON 文件
   ↓
步骤 2: 判断 jsonFile 是否为 null
   ├── 是 → 返回 null（未找到 JSON 文件）
   └── 否 → 继续步骤 3
   ↓
步骤 3: 读取 JSON 文件内容 jsonFile.readAsString()
   ↓
步骤 4: 解析 JSON jsonDecode(...)
   ↓
步骤 5: 提取时间戳 data['photoTakenTime']['timestamp']
   ↓
步骤 6: 转换为整数 int.parse(...)
   ↓
步骤 7: 转换为 DateTime DateTime.fromMillisecondsSinceEpoch(epoch * 1000)
   ↓
步骤 8: 返回 DateTime 对象
```

### 异常处理（捕获三种异常，全部返回 `null`）

| 异常类型 | 触发条件 | 具体场景 |
|----------|----------|----------|
| `FormatException` | JSON 格式错误 | JSON 内容不符合规范，如缺少引号、括号不匹配 |
| `FileSystemException` | 文件编码问题 | 文件内容无法用 UTF-8 解码（Issue #143） |
| `NoSuchMethodError` | 标签缺失 | JSON 中不存在 `photoTakenTime` 或其子字段 `timestamp` |

---

## 二、核心查找函数 `_jsonForFile`

### 函数签名
```dart
Future<File?> _jsonForFile(File file, {required bool tryhard})
```

### 参数说明
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | `File` | 是 | 目标图片文件 |
| `tryhard` | `bool` | 是 | 是否启用激进模式 |

### 返回值
| 返回值 | 说明 |
|--------|------|
| `File` | 找到的 JSON 文件对象 |
| `null` | 所有查找策略都失败 |

### 执行流程

```
步骤 1: 获取文件所在目录
   dir = Directory(p.dirname(file.path))
   ↓
步骤 2: 获取文件名（含扩展名）
   name = p.basename(file.path)
   ↓
步骤 3: 构建方法列表（根据 tryhard 参数动态决定）
   ↓
步骤 4: 遍历方法列表，对每个 method 执行：
   ├── 4a: 对 name 应用变换方法 → transformedName = method(name)
   ├── 4b: 构造候选 JSON 文件路径 → jsonFile = File(p.join(dir.path, '${transformedName}.json'))
   ├── 4c: 检查文件是否存在 → await jsonFile.exists()
   │   ├── 存在 → 立即返回 jsonFile（短路返回）
   │   └── 不存在 → 继续下一个方法
   ↓
步骤 5: 所有方法都失败 → 返回 null
```

---

## 三、方法列表详解

### 方法列表的构建规则

方法列表是一个 Dart 列表字面量，使用 **集合展开操作符** (`...[]`) 根据 `tryhard` 参数动态包含/排除方法：

```dart
for (final method in [
    (String s) => s,           // 方法 1: 始终包含
    _shortenName,              // 方法 2: 始终包含
    _bracketSwap,              // 方法 3: 始终包含
    _removeExtra,              // 方法 4: 始终包含
    _noExtension,              // 方法 5: 始终包含
    if (tryhard) ...[          // 条件展开
      _removeExtraRegex,       // 方法 6: 仅 tryhard=true 时包含
      _removeDigit,            // 方法 7: 仅 tryhard=true 时包含
    ]
])
```

### 方法列表的两种形态

| 模式 | 包含的方法 | 方法数量 |
|------|------------|----------|
| **普通模式** (`tryhard=false`) | 方法 1~5 | 5 个 |
| **激进模式** (`tryhard=true`) | 方法 1~7 | 7 个 |

---

## 四、各变换方法详细实现

### 方法 1: 无变换（恒等函数）

**代码：**
```dart
(String s) => s
```

**逻辑：**
- 输入什么，输出什么
- 直接尝试原始文件名

**查找路径：**
```
输入: photo.jpg
输出: photo.jpg
查找: photo.jpg.json
```

**适用场景：** 文件名未经 Google 处理，原始文件名与 JSON 侧车文件名完全匹配

---

### 方法 2: `_shortenName` — 文件名截断

**代码：**
```dart
String _shortenName(String filename) => '$filename.json'.length > 51
    ? filename.substring(0, 51 - '.json'.length)
    : filename;
```

**逻辑分支：**

| 条件 | 计算方式 | 操作 |
|------|----------|------|
| `'$filename.json'.length > 51` | 文件名 + `.json` 的总长度超过 51 字符 | 截断 `filename` 到前 `46` 个字符（`51 - 5 = 46`） |
| `'$filename.json'.length <= 51` | 文件名 + `.json` 的总长度不超过 51 字符 | 返回原始 `filename`，不做任何修改 |

**查找路径示例：**

| 输入文件名 | 条件判断 | 输出 | 查找的 JSON 路径 |
|-----------|----------|------|-----------------|
| `photo.jpg` | 10 <= 51 | `photo.jpg` | `photo.jpg.json` |
| `very_long_filename_that_exceeds_fifty_one_characters_limit.jpg` | 67 > 51 | `very_long_filename_that_exceeds_forty_six` | `very_long_filename_that_exceeds_forty_six.json` |

**设计原因：** Google Takeout 对文件名有 51 字符的限制，超长文件名会被截断。原始长文件名的 JSON 侧车文件使用的是截断后的名称。

---

### 方法 3: `_bracketSwap` — 括号位置交换

**代码：**
```dart
String _bracketSwap(String filename) {
  final match = RegExp(r'\(\d+\)\.').allMatches(filename).lastOrNull;
  if (match == null) return filename;
  final bracket = match.group(0)!.replaceAll('.', '');
  final withoutBracket = filename.replaceLast(bracket, '');
  return '$withoutBracket$bracket';
}
```

**逻辑分支：**

| 步骤 | 操作 | 条件/结果 |
|------|------|-----------|
| 1 | 用正则 `\(\d+\)\.` 匹配 `(数字).` 模式 | 匹配形如 `(11).`、`(1).`、`(123).` 的字符串 |
| 2 | 取 `lastOrNull`（最后一个匹配） | 无匹配 → 返回原始文件名，结束 |
| 3 | 从匹配结果中移除点号 | `(11).` → `(11)` |
| 4 | 从原文件名中移除最后一次出现的括号部分 | `image(11).jpg` → `image.jpg` |
| 5 | 将括号部分追加到文件名末尾 | `image.jpg` + `(11)` → `image.jpg(11)` |

**查找路径示例：**

| 输入文件名 | 正则匹配结果 | bracket | withoutBracket | 输出 | 查找的 JSON 路径 |
|-----------|-------------|---------|----------------|------|-----------------|
| `image(11).jpg` | `(11).` | `(11)` | `image.jpg` | `image.jpg(11)` | `image.jpg(11).json` |
| `photo(1).png` | `(1).` | `(1)` | `photo.png` | `photo.png(1)` | `photo.png(1).json` |
| `image(3).(2)(3).jpg` | `(3).`（最后一个） | `(3)` | `image(3).(2).jpg` | `image(3).(2).jpg(3)` | `image(3).(2).jpg(3).json` |
| `normal.jpg` | 无匹配 | — | — | `normal.jpg`（原样返回） | `normal.jpg.json` |

**为什么取 `lastOrNull` 而不是第一个匹配：**
避免文件名如 `image(3).(2)(3).jpg` 被错误处理。如果取第一个匹配 `(3).`，会得到错误结果。取最后一个匹配确保处理的是扩展名前的括号。

**设计原因：** Google Takeout 会将 `image(11).jpg` 的 JSON 侧车文件命名为 `image.jpg(11).json`（括号位置从扩展名前移到扩展名后）。

---

### 方法 4: `_removeExtra` — 移除已知的"编辑"后缀

**代码：**
```dart
String _removeExtra(String filename) {
  filename = unorm.nfc(filename);
  for (final extra in extras.extraFormats) {
    if (filename.contains(extra)) {
      return filename.replaceLast(extra, '');
    }
  }
  return filename;
}
```

**逻辑分支：**

| 步骤 | 操作 | 条件/结果 |
|------|------|-----------|
| 1 | NFC 规范化 | `unorm.nfc(filename)` — 将 NFD 编码统一为 NFC 编码（处理 macOS 文件系统差异） |
| 2 | 遍历 `extraFormats` 列表（12 种语言的"编辑"后缀） | 按列表顺序逐一检查 |
| 3 | 对每个 `extra`，检查 `filename.contains(extra)` | 包含 → 进入步骤 4；不包含 → 继续下一个 `extra` |
| 4 | 移除最后一次出现的 `extra` | `filename.replaceLast(extra, '')`，立即返回结果，结束循环 |
| 5 | 所有 `extra` 都不匹配 | 返回原始 `filename` |

**`extraFormats` 完整列表（12 项）：**

| 序号 | 语言 | 后缀 | 备注 |
|------|------|------|------|
| 1 | 英语/美式 | `-edited` | |
| 2 | 英语/美式 | `-effects` | |
| 3 | 英语/美式 | `-smile` | |
| 4 | 英语/美式 | `-mix` | |
| 5 | 波兰语 | `-edytowane` | |
| 6 | 德语 | `-bearbeitet` | |
| 7 | 荷兰语 | `-bewerkt` | |
| 8 | 日语 | `-編集済み` | |
| 9 | 意大利语 | `-modificato` | |
| 10 | 法语 | `-modifié` | 含重音字符 |
| 11 | 西班牙语 | `-ha editado` | 含空格 |
| 12 | 加泰罗尼亚语 | `-editat` | |

**查找路径示例：**

| 输入文件名 | 匹配的 extra | 输出 | 查找的 JSON 路径 |
|-----------|-------------|------|-----------------|
| `photo-edited.jpg` | `-edited` | `photo.jpg` | `photo.jpg.json` |
| `vacation-effects.png` | `-effects` | `vacation.png` | `vacation.png.json` |
| `urlaub-bearbeitet.jpg` | `-bearbeitet` | `urlaub.jpg` | `urlaub.jpg.json` |
| `photo-modifié.jpg` | `-modifié` | `photo.jpg` | `photo.jpg.json` |
| `normal.jpg` | 无匹配 | `normal.jpg`（原样返回） | `normal.jpg.json` |

**为什么使用 `replaceLast` 而不是 `replaceAll`：**
防止意外移除文件名中间出现的字符串。例如 `my-edited-photo-edited.jpg` 只移除最后一个 `-edited`。

---

### 方法 5: `_noExtension` — 移除扩展名

**代码：**
```dart
String _noExtension(String filename) =>
    p.basenameWithoutExtension(File(filename).path);
```

**逻辑：**
- 提取不含扩展名的文件名部分
- `path.basenameWithoutExtension()` 移除最后一个 `.` 及其后的内容

**查找路径示例：**

| 输入文件名 | 输出 | 查找的 JSON 路径 |
|-----------|------|-----------------|
| `20030616.jpg` | `20030616` | `20030616.json` |
| `photo.png` | `photo` | `photo.json` |
| `archive.tar.gz` | `archive.tar` | `archive.tar.json` |

**设计原因：** 原始文件上传时没有扩展名（如 `20030616`），Google 处理后添加了扩展名（变成 `20030616.jpg`），但 JSON 侧车文件仍使用无扩展名的原始名称（`20030616.json`）。

---

### 方法 6: `_removeExtraRegex` — 正则移除 extra 后缀（仅 tryhard 模式）

**代码：**
```dart
String _removeExtraRegex(String filename) {
  filename = unorm.nfc(filename);
  final matches = RegExp(r'(?<extra>-[A-Za-zÀ-ÖØ-öø-ÿ]+(\(\d\))?)\.\w+$')
      .allMatches(filename);
  if (matches.length == 1) {
    return filename.replaceAll(matches.first.namedGroup('extra')!, '');
  }
  return filename;
}
```

**逻辑分支：**

| 步骤 | 操作 | 条件/结果 |
|------|------|-----------|
| 1 | NFC 规范化 | `unorm.nfc(filename)` |
| 2 | 用正则 `(?<extra>-[A-Za-zÀ-ÖØ-öø-ÿ]+(\(\d\))?)\.\w+$` 匹配 | 匹配末尾的 `-字母串(可选数字).扩展名` 模式 |
| 3 | 检查 `matches.length` | **== 1** → 进入步骤 4；**!= 1**（0 个或多个）→ 返回原始文件名 |
| 4 | 提取命名组 `extra` | 获取匹配到的 extra 部分 |
| 5 | 从文件名中移除 `extra` | `filename.replaceAll(extra, '')`，返回结果 |

**正则表达式详解：**

```
(?<extra>-[A-Za-zÀ-ÖØ-öø-ÿ]+(\(\d\))?)\.\w+$
│       │                      │        │  │
│       │                      │        │  └─ $: 字符串末尾
│       │                      │        └──── \.\w+: 点号 + 扩展名（字母数字）
│       │                      └───────────── (\(\d\))?: 可选的 (数字) 部分
│       └─────────────────────────────────── [A-Za-zÀ-ÖØ-öø-ÿ]+: 一个或多个字母（含重音）
└─────────────────────────────────────────── (?<extra>...): 命名捕获组
```

**正则匹配示例：**

| 输入文件名 | 是否匹配 | 匹配的 extra | 输出 | 查找的 JSON 路径 |
|-----------|----------|-------------|------|-----------------|
| `something-edited(1).jpg` | 是 | `-edited(1)` | `something.jpg` | `something.jpg.json` |
| `photo-modifié.png` | 是 | `-modifié` | `photo.png` | `photo.png.json` |
| `image-bearbeitet(2).gif` | 是 | `-bearbeitet(2)` | `image.gif` | `image.gif.json` |
| `normal.jpg` | 否 | — | `normal.jpg`（原样返回） | `normal.jpg.json` |
| `a-b-c.jpg` | 否（匹配多个） | — | `a-b-c.jpg`（原样返回） | `a-b-c.jpg.json` |

**为什么要求 `matches.length == 1`：**
防止误匹配。如果文件名中有多个类似模式（如 `a-b-c.jpg`），不确定的情况下保持原样，避免错误移除。

**与 `_removeExtra` 的区别：**
- `_removeExtra`：只移除预定义的 12 种语言后缀，安全但覆盖有限
- `_removeExtraRegex`：匹配任意 `-字母串` 模式，覆盖更广但有误判风险，因此仅在 tryhard 模式使用

---

### 方法 7: `_removeDigit` — 移除数字括号（仅 tryhard 模式）

**代码：**
```dart
String _removeDigit(String filename) =>
    filename.replaceAll(RegExp(r'\(\d\)\.'), '.');
```

**逻辑：**
- 用正则 `\(\d\)\.` 匹配 `(单个数字).` 模式
- 替换为 `.`（移除括号和数字）
- 使用 `replaceAll`（全局替换，非仅最后一次）

**查找路径示例：**

| 输入文件名 | 正则匹配 | 输出 | 查找的 JSON 路径 |
|-----------|----------|------|-----------------|
| `photo(1).jpg` | `(1).` | `photo.jpg` | `photo.jpg.json` |
| `image(3).png` | `(3).` | `image.png` | `image.png.json` |
| `a(1)b(2).jpg` | `(2).` | `a(1)b.jpg` | `a(1)b.jpg.json` |
| `normal.jpg` | 无匹配 | `normal.jpg`（原样返回） | `normal.jpg.json` |

**为什么放在最后：**
注释说明 `// most files with '(digit)' have jsons`，意味着大多数带数字括号的文件都能通过前面的方法找到 JSON，所以这个方法作为最后的兜底策略。

**与 `_bracketSwap` 的区别：**
- `_bracketSwap`：`(11).jpg` → `.jpg(11)`（交换位置，保留括号）
- `_removeDigit`：`(1).jpg` → `.jpg`（直接移除括号）

---

## 五、完整查找流程示例

### 示例 1: 普通文件名

```
输入文件: /Takeout/Photos from 2020/photo.jpg
tryhard: false

方法 1 (无变换):  查找 photo.jpg.json         → 不存在
方法 2 (截断):    查找 photo.jpg.json          → 不存在（文件名未超 51 字符，无变化）
方法 3 (括号交换): 查找 photo.jpg.json         → 不存在（无括号，无变化）
方法 4 (移除extra): 查找 photo.jpg.json        → 不存在（无 extra 后缀，无变化）
方法 5 (无扩展名): 查找 photo.json             → 不存在
→ 返回 null
```

### 示例 2: 带编辑后缀的文件

```
输入文件: /Takeout/Photos from 2020/vacation-edited.jpg
tryhard: false

方法 1 (无变换):  查找 vacation-edited.jpg.json    → 不存在
方法 2 (截断):    查找 vacation-edited.jpg.json    → 不存在
方法 3 (括号交换): 查找 vacation-edited.jpg.json   → 不存在
方法 4 (移除extra): 查找 vacation.jpg.json         → 存在！返回该文件
→ 返回 File(/Takeout/Photos from 2020/vacation.jpg.json)
```

### 示例 3: 括号位置交换的文件

```
输入文件: /Takeout/Photos from 2020/image(11).jpg
tryhard: false

方法 1 (无变换):  查找 image(11).jpg.json    → 不存在
方法 2 (截断):    查找 image(11).jpg.json    → 不存在
方法 3 (括号交换): 查找 image.jpg(11).json   → 存在！返回该文件
→ 返回 File(/Takeout/Photos from 2020/image.jpg(11).json)
```

### 示例 4: 超长文件名（tryhard 模式）

```
输入文件: /Takeout/Photos from 2020/very_long_filename_that_exceeds_fifty_one_characters_limit-edited.jpg
tryhard: true

方法 1 (无变换):       查找 very_long_filename_that_exceeds_fifty_one_characters_limit-edited.jpg.json  → 不存在
方法 2 (截断):         查找 very_long_filename_that_exceeds_forty_six.json                              → 不存在
方法 3 (括号交换):     查找 very_long_filename_that_exceeds_fifty_one_characters_limit-edited.jpg.json  → 不存在
方法 4 (移除extra):    查找 very_long_filename_that_exceeds_fifty_one_characters_limit.jpg.json         → 不存在
方法 5 (无扩展名):     查找 very_long_filename_that_exceeds_fifty_one_characters_limit-edited.json      → 不存在
方法 6 (正则extra):    查找 very_long_filename_that_exceeds_fifty_one_characters_limit.jpg.json         → 不存在
方法 7 (移除数字):     查找 very_long_filename_that_exceeds_fifty_one_characters_limit-edited.jpg.json  → 不存在
→ 返回 null
```

### 示例 5: 带数字括号的编辑文件（tryhard 模式）

```
输入文件: /Takeout/Photos from 2020/photo-something(1).jpg
tryhard: true

方法 1 (无变换):       查找 photo-something(1).jpg.json    → 不存在
方法 2 (截断):         查找 photo-something(1).jpg.json    → 不存在
方法 3 (括号交换):     查找 photo-something.jpg(1).json    → 不存在
方法 4 (移除extra):    查找 photo-something(1).jpg.json    → 不存在（-something 不在 extraFormats 中）
方法 5 (无扩展名):     查找 photo-something(1).json        → 不存在
方法 6 (正则extra):    查找 photo.jpg.json                 → 存在！返回该文件
→ 返回 File(/Takeout/Photos from 2020/photo.jpg.json)
```

---

## 六、方法执行顺序与条件总结

| 顺序 | 方法名 | 执行条件 | 变换类型 | 安全性 |
|------|--------|----------|----------|--------|
| 1 | 恒等函数 | 始终执行 | 无变换 | 最高（原始匹配） |
| 2 | `_shortenName` | 始终执行 | 截断到 46 字符 | 高（Google 限制） |
| 3 | `_bracketSwap` | 始终执行 | `(11).jpg` → `.jpg(11)` | 高（已知模式） |
| 4 | `_removeExtra` | 始终执行 | 移除 12 种语言后缀 | 高（预定义列表） |
| 5 | `_noExtension` | 始终执行 | 移除扩展名 | 中（可能误判） |
| 6 | `_removeExtraRegex` | 仅 tryhard=true | 正则移除 `-字母串` | 低（可能误判） |
| 7 | `_removeDigit` | 仅 tryhard=true | 移除 `(数字).` | 低（可能误判） |

**安全性递减：** 前面的方法基于已知的 Google Takeout 行为模式，后面的方法基于更宽泛的正则匹配，误判风险递增。

---

## 七、关键依赖

| 依赖 | 用途 |
|------|------|
| `package:unorm_dart/unorm_dart.dart` | NFC 规范化，处理 macOS NFD 编码差异 |
| `package:path/path.dart` | 路径操作（dirname, basename, join, basenameWithoutExtension） |
| `package:gpth/extras.dart` | 提供 `extraFormats` 常量（12 种语言后缀列表） |
| `dart:convert` | JSON 解析（jsonDecode） |
| `dart:io` | 文件操作（File, Directory） |

# Copilot PR Review 修改 — 开发文档

## 一、修改概览

本次修改针对 Copilot PR Review 提出的 16 条意见，按优先级分为高/中/低三档，涉及错误处理、并发安全、跨平台兼容、依赖管理和 CI 完善。

---

## 二、高优先级修改（必须修复）

### 2.1 `internal/migrator/migrator.go` — 错误处理与并发安全

#### 问题 1：scanFiles 忽略 Walk 错误
**现状**：`filepath.Walk` 返回的错误被丢弃，权限错误等会导致静默漏文件。
**修改方案**：
```go
// 修改前
filepath.Walk(yf, func(path string, info os.FileInfo, err error) error {
    if err != nil || info.IsDir() { return nil }
    // ...
    return nil
})

// 修改后
if err := filepath.Walk(yf, func(path string, info os.FileInfo, walkErr error) error {
    if walkErr != nil { return walkErr }
    if info.IsDir() { return nil }
    // ...
    return nil
}); err != nil {
    return nil, fmt.Errorf("walk %s: %w", yf, err)
}
```

#### 问题 2：relPath 错误被丢弃
**现状**：`filepath.Rel` 失败时 RelPath 为空，影响日志和 error 目录路径。
**修改方案**：
```go
relPath, relErr := filepath.Rel(inputDir, path)
if relErr != nil {
    relPath = path // fallback 到绝对路径
}
```

#### 问题 3：进度条乱序
**现状**：并发 worker 发送进度可能乱序（如 20 先于 10 打印），进度条倒退。
**修改方案**：
```go
// 新增 progress renderer goroutine
progCh := make(chan int, workers)
go func() {
    last := 0
    for cur := range progCh {
        if cur > last {
            last = cur
            progress.PrintProgress(cur, total)
        }
    }
}()

// worker 中发送进度
progCh <- int(processed.Add(1))
```

#### 问题 4：cleanup 忽略 Rename 错误
**现状**：恢复原名失败时，文件留在 tmpPath，后续 hashing/moveToError 基于错误路径。
**修改方案**：
```go
cleanup := func() error {
    return os.Rename(tmpPath, dstPath)
}
// 调用处检查错误
if err := cleanupRename(); err != nil {
    // 移动 tmpPath 到 error，而非 dstPath
    moveToErrorByPath(tmpPath, entry.RelPath, outputDir, jsonResult)
}
```

#### 问题 5：moveToErrorByPath 忽略错误
**现状**：os.Rename 和 copyToPath 失败时静默跳过，文件可能留在输出目录。
**修改方案**：
```go
if err := os.Rename(srcPath, dstError); err != nil {
    // fallback: copy + remove
    if copyErr := copyToPath(srcPath, dstError); copyErr != nil {
        // log both errors
    } else {
        os.Remove(srcPath)
    }
}
```

### 2.2 `cmd/gtoh/cmd/root.go` — Execute 吞掉错误
**现状**：命令失败时 exit code 仍为 0，CI/shell 无法检测失败。
**修改方案**：
```go
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### 2.3 `internal/dedup/dedup.go` — 扩展名与解码器不匹配
**现状**：`imageExts` 包含 bmp/tiff/webp/heic，但只注册了 GIF/JPEG/PNG 解码器。
**修改方案**：移除不支持的扩展名，或添加 `golang.org/x/image/{bmp,tiff,webp}` 依赖。

---

## 三、中优先级修改（建议修复）

### 3.1 `cmd/gtoh/cmd/organize.go` — Windows 路径拼接
**现状**：`organizeDir + "/" + organizeMode` 在 Windows 下失效。
**修改方案**：使用 `filepath.Join(organizeDir, organizeMode)`。

### 3.2 `internal/organizer/organizer.go` — resolveDestPath 冲突
**现状**：只检查一次存在性，同一秒内多个冲突或历史残留文件会被覆盖。
**修改方案**：循环直到找到不存在的文件名（加计数器）。
```go
func resolveDestPath(destDir, name string) string {
    target := filepath.Join(destDir, name)
    if _, err := os.Stat(target); os.IsNotExist(err) {
        return target
    }
    ext := filepath.Ext(name)
    stem := strings.TrimSuffix(name, ext)
    for i := 1; ; i++ {
        candidate := filepath.Join(destDir, fmt.Sprintf("%s_%d%s", stem, i, ext))
        if _, err := os.Stat(candidate); os.IsNotExist(err) {
            return candidate
        }
    }
}
```

### 3.3 `internal/parser/exiftool.go` — 注释与代码不符
**现状**：注释说"both lat and lon non-zero"，代码是 `||`（任一非零）。
**修改方案**：更新注释匹配实际行为：`// GPS is valid if at least one of lat/lon is non-zero`。

### 3.4 `internal/parser/exiftool_test.go` — 忽略 WriteFile 错误
**现状**：测试文件写入失败时，测试可能误判。
**修改方案**：检查 `os.WriteFile` 返回值，失败时 `t.Fatal(err)`。

### 3.5 `internal/migrator/hasher.go` — 未使用的重复函数
**现状**：`SHA256File` 与 `copier.go` 的 `HashFile` 功能重复且未使用。
**修改方案**：删除 `hasher.go`。

### 3.6 `go.mod` — goimagehash 标记为 indirect
**现状**：`goimagehash` 被 `internal/dedup` 直接导入，不应标记为 `// indirect`。
**修改方案**：运行 `go mod tidy`。

---

## 四、低优先级修改（可选修复）

### 4.1 `Makefile` — 缺少 vet target
**修改方案**：添加 `vet: go vet ./...`，`lint: vet`。

### 4.2 `.github/workflows/ci.yml` — 缺少 go vet 步骤
**修改方案**：添加 `- run: go vet ./...` 步骤。

### 4.3 `README.md` — PR 描述与实际命令不符
**修改方案**：对齐 PR 描述与实际 CLI 命令集（fix-takeout, fix-img, organize, rename, clean-json, migrate, dedup）。

---

## 五、实施顺序

| 阶段 | 修改项 | 预计工作量 |
|------|--------|------------|
| 1 | 高优先级 1-5（migrator.go 错误处理与并发） | 1.5h |
| 2 | 高优先级 6-7（root.go, dedup.go） | 0.5h |
| 3 | 中优先级 1-6（organize.go, organizer.go, exiftool.go, hasher.go, go.mod） | 1h |
| 4 | 低优先级 1-3（Makefile, ci.yml, README.md） | 0.5h |

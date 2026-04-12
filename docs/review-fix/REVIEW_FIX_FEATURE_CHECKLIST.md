# Copilot PR Review 修改 — 功能验证清单

## 一、错误处理验证

- [ ] **Walk 错误传播**：模拟权限错误，`Run` 返回错误而非静默继续
- [ ] **Rel 错误 fallback**：模拟 `filepath.Rel` 失败，`RelPath` 使用绝对路径
- [ ] **cleanup Rename 错误**：模拟恢复原名失败，文件移到 error/ 而非丢失
- [ ] **moveToErrorByPath 错误**：模拟 Rename 失败，fallback 到 copy+remove
- [ ] **Execute 退出码**：命令失败时 exit code 为 1

## 二、并发安全验证

- [ ] **进度条单调性**：20 文件并发处理，进度条从 5% 到 100% 无倒退
- [ ] **stats 计数器准确**：并发处理后 Scanned = Processed + Skipped + Failed
- [ ] **logger 无乱码**：并发写入 log 文件无交错或丢失

## 三、跨平台兼容验证

- [ ] **Windows 路径**：`organize.go` 使用 `filepath.Join`，路径分隔符正确
- [ ] **路径冲突处理**：`resolveDestPath` 循环生成唯一文件名，不覆盖

## 四、依赖与配置验证

- [ ] **dedup 扩展名匹配**：仅处理支持的格式（jpg/jpeg/png/gif），bmp/tiff/webp 被跳过或正确解码
- [ ] **go.mod 准确性**：`goimagehash` 为 direct require，`go mod tidy` 无变更
- [ ] **hasher.go 清理**：无重复的 SHA256 函数

## 五、CI 与文档验证

- [ ] **CI 运行 vet**：workflow 包含 `go vet ./...` 步骤
- [ ] **Makefile vet target**：`make vet` 执行 `go vet`
- [ ] **README 命令对齐**：README 列出的命令与实际 `gtoh --help` 一致
- [ ] **注释与代码一致**：exiftool.go GPS 注释匹配 `||` 逻辑

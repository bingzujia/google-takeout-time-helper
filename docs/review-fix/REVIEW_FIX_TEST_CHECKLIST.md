# Copilot PR Review 修改 — 测试检查清单

## 一、单元测试

### 1.1 migrator 错误处理测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| Walk 权限错误 | 无读权限的目录 | `Run` 返回错误，不静默跳过 |
| Rel 路径失败 | 跨盘符路径（模拟） | `RelPath` 使用绝对路径，不 panic |
| cleanup Rename 失败 | 目标路径只读 | 移动 tmpPath 到 error/，记录错误 |
| moveToErrorByPath Rename 失败 | 目标已存在 | fallback 到 copy+remove，记录错误 |

### 1.2 并发安全测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 进度条顺序 | 20 文件，8 workers | 进度条单调递增，无倒退 |
| stats 准确性 | 100 文件并发 | Scanned = Processed + Skipped + Failed |
| logger 并发 | 50 文件并发写入 | log 文件无乱码或丢失行 |

### 1.3 dedup 扩展名测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| 支持格式 | jpg/png/gif 文件 | 正常处理 |
| 不支持格式 | bmp/tiff/webp 文件 | 跳过或正确解码（若添加依赖） |

### 1.4 其他单元测试

| 测试用例 | 输入 | 预期 |
|----------|------|------|
| os.WriteFile 错误 | 只读目录 | 测试失败并报告错误 |
| resolveDestPath 冲突 | 同名文件已存在 | 生成 `name_1.ext`, `name_2.ext` 等 |
| Execute 退出码 | 无效命令参数 | exit code 1 |

## 二、集成测试

### 2.1 完整流程测试

| 测试场景 | 预期 |
|----------|------|
| 正常迁移（20 文件） | 全部处理成功，进度条单调，log 完整 |
| 包含权限错误目录 | `Run` 返回错误，不静默继续 |
| 包含重命名冲突 | 循环生成唯一文件名，不覆盖 |

### 2.2 并发压力测试

| 规模 | 预期 |
|------|------|
| 100 文件，8 workers | 无 race condition，stats 准确 |
| 1000 文件，8 workers | 进度条平滑，无内存泄漏 |

## 三、CI 与工具链验证

| 检查项 | 命令 | 预期 |
|--------|------|------|
| 编译 | `go build ./...` | 无错误 |
| Vet | `go vet ./...` | 无警告 |
| 测试 | `go test -race ./...` | 全部通过，无 data race |
| 依赖 | `go mod tidy` | 无变更 |
| Makefile | `make vet` | 执行 `go vet` |

## 四、手动验证

| 验证项 | 操作 | 预期 |
|--------|------|------|
| 进度条单调性 | 运行 50 文件迁移 | 进度条从 0% 到 100% 无倒退 |
| 错误文件 quarantine | 故意制造一个 exiftool 失败的文件 | 文件移到 `output/error/`，log 记录错误 |
| Windows 路径兼容 | 在 WSL/Windows 下运行 organize | 路径分隔符正确，无 `/` 硬编码问题 |
| README 对齐 | 对比 `gtoh --help` 与 README | 命令列表一致 |

# g_photo_take_out_helper — 打包为 Linux 单文件二进制的二次开发手册

## 目录

1. [打包方案选型](#1-打包方案选型)
2. [打包前：统一入口重构](#2-打包前统一入口重构)
   - 2.1 [从多脚本改为单 CLI 工具](#21-从多脚本改为单-cli-工具)
   - 2.2 [目录结构规范](#22-目录结构规范)
   - 2.3 [统一入口脚本模板](#23-统一入口脚本模板)
3. [方案 A：shc 编译为原生二进制](#3-方案-ashc-编译为原生二进制)
   - 3.1 [安装 shc](#31-安装-shc)
   - 3.2 [编译步骤](#32-编译步骤)
   - 3.3 [处理外部依赖（jq 内嵌）](#33-处理外部依赖jq-内嵌)
   - 3.4 [shc 的限制与注意事项](#34-shc-的限制与注意事项)
4. [方案 B：makeself 自解压包（推荐）](#4-方案-bmakeself-自解压包推荐)
   - 4.1 [安装 makeself](#41-安装-makeself)
   - 4.2 [构建目录结构](#42-构建目录结构)
   - 4.3 [打包命令](#43-打包命令)
   - 4.4 [发布与安装流程](#44-发布与安装流程)
5. [方案 C：静态 Shell + bpkg/basher 包管理（开发者友好）](#5-方案-c静态-shell--bpkgbasher-包管理开发者友好)
6. [自动化构建：Makefile](#6-自动化构建makefile)
7. [依赖管理策略](#7-依赖管理策略)
   - 7.1 [运行时检测依赖](#71-运行时检测依赖)
   - 7.2 [内嵌静态 jq 二进制](#72-内嵌静态-jq-二进制)
8. [版本号管理](#8-版本号管理)
9. [二次开发工作流](#9-二次开发工作流)
   - 9.1 [源码修改 → 重新打包流程](#91-源码修改--重新打包流程)
   - 9.2 [解包已发布 bin 文件进行调试](#92-解包已发布-bin-文件进行调试)
   - 9.3 [新增子命令](#93-新增子命令)
   - 9.4 [修改已有子命令逻辑](#94-修改已有子命令逻辑)
10. [测试策略](#10-测试策略)
11. [发布检查清单](#11-发布检查清单)
12. [常见问题](#12-常见问题)

---

## 1. 打包方案选型

将多个 Bash 脚本打包为单一可执行文件，主要有以下三种方案：

| 方案 | 产物类型 | 优点 | 缺点 | 适用场景 |
|------|----------|------|------|----------|
| **shc** | 原生 ELF 二进制 | 运行最快、无需解压、可防止源码被直接读取 | 依赖 `bash` 版本、调试困难、跨架构需分别编译 | 商业分发、希望保护源码 |
| **makeself** | 自解压 Shell 脚本（`.run`） | 无需额外工具、跨发行版、可携带依赖、易调试 | 体积稍大、首次运行需解压 | **开源分发（推荐）** |
| **basher/bpkg** | Shell 包（源码形式） | 开发者友好、版本可控 | 需要包管理器 | 面向开发者的分发 |

**本手册重点讲解方案 A（shc）和方案 B（makeself）**，并以 makeself 为推荐方案。

---

## 2. 打包前：统一入口重构

打包前必须将多个独立脚本整合为**单一入口 + 子命令**的 CLI 架构，否则无法打包为一个 bin 文件。

### 2.1 从多脚本改为单 CLI 工具

**现状（多脚本）**
```
fix_takeout_photo_time_wsl.sh
rename_photos.sh
organize_photos.sh
organize_screenshots.sh
organize_wechat.sh
fix_img_timestamps.sh
delete_json_files.sh
```

**目标（单 CLI）**
```bash
gphotoh fix-time        # 对应 fix_takeout_photo_time_wsl.sh
gphotoh rename          # 对应 rename_photos.sh
gphotoh organize-camera # 对应 organize_photos.sh
gphotoh organize-shots  # 对应 organize_screenshots.sh
gphotoh organize-wechat # 对应 organize_wechat.sh
gphotoh fix-img-time    # 对应 fix_img_timestamps.sh
gphotoh delete-json     # 对应 delete_json_files.sh
gphotoh run-all         # 一键执行完整工作流
```

### 2.2 目录结构规范

重构后的源码目录结构：

```
g_photo_take_out_helper/
├── src/
│   ├── main.sh                    # 统一入口（dispatch router）
│   ├── lib/
│   │   ├── common.sh              # 颜色、日志、进度条等公共函数
│   │   └── version.sh             # 版本号常量
│   └── commands/
│       ├── fix_time.sh            # gphotoh fix-time
│       ├── rename.sh              # gphotoh rename
│       ├── organize_camera.sh     # gphotoh organize-camera
│       ├── organize_shots.sh      # gphotoh organize-shots
│       ├── organize_wechat.sh     # gphotoh organize-wechat
│       ├── fix_img_time.sh        # gphotoh fix-img-time
│       └── delete_json.sh         # gphotoh delete-json
├── dist/                          # 构建产物（git ignore）
├── build/                         # 临时构建目录（git ignore）
├── Makefile                       # 构建脚本
└── scripts/
    └── bundle.sh                  # 合并脚本（将所有 .sh 合并为单文件）
```

### 2.3 统一入口脚本模板

`src/main.sh` 的结构如下：

```bash
#!/bin/bash
# g_photo_take_out_helper — 统一入口
# 使用方法: gphotoh <command> [options]

set -euo pipefail

# ── 版本与路径 ────────────────────────────────────────────────────────────────
VERSION="1.0.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── 引入公共库 ────────────────────────────────────────────────────────────────
source "$SCRIPT_DIR/lib/common.sh"

# ── 帮助信息 ──────────────────────────────────────────────────────────────────
show_help() {
    cat <<EOF
gphotoh v${VERSION} — Google Takeout 照片整理工具

用法:
  gphotoh <command> [options]

命令:
  fix-time          从 JSON 元数据恢复照片时间戳（Google Takeout 专用）
  rename            按时间戳统一重命名为 IMG/VID 格式
  organize-camera   将相机照片归类到 camera/ 目录
  organize-shots    将截图归类到 screenshot/ 目录
  organize-wechat   将微信媒体归类到 wechat/ 目录
  fix-img-time      按 IMG/VID 文件名校正时间戳
  delete-json       删除所有 JSON 元数据文件
  run-all           按顺序执行完整工作流

全局选项:
  -h, --help        显示此帮助
  -v, --version     显示版本号
  -d, --dry-run     预览模式（对所有支持的命令生效）

示例:
  gphotoh fix-time --dry-run
  gphotoh organize-camera --current
  gphotoh run-all --dry-run
EOF
}

# ── 命令路由 ──────────────────────────────────────────────────────────────────
main() {
    local command="${1:-}"
    shift || true

    case "$command" in
        fix-time)         source "$SCRIPT_DIR/commands/fix_time.sh" "$@" ;;
        rename)           source "$SCRIPT_DIR/commands/rename.sh" "$@" ;;
        organize-camera)  source "$SCRIPT_DIR/commands/organize_camera.sh" "$@" ;;
        organize-shots)   source "$SCRIPT_DIR/commands/organize_shots.sh" "$@" ;;
        organize-wechat)  source "$SCRIPT_DIR/commands/organize_wechat.sh" "$@" ;;
        fix-img-time)     source "$SCRIPT_DIR/commands/fix_img_time.sh" "$@" ;;
        delete-json)      source "$SCRIPT_DIR/commands/delete_json.sh" "$@" ;;
        run-all)          run_all_workflow "$@" ;;
        -h|--help|help)   show_help; exit 0 ;;
        -v|--version)     echo "gphotoh v${VERSION}"; exit 0 ;;
        "")               show_help; exit 1 ;;
        *)
            echo "❌ 未知命令: $command"
            echo "运行 'gphotoh --help' 查看可用命令"
            exit 1
            ;;
    esac
}

# ── 完整工作流 ────────────────────────────────────────────────────────────────
run_all_workflow() {
    log_info "开始执行完整工作流..."
    source "$SCRIPT_DIR/commands/fix_time.sh" "$@"
    source "$SCRIPT_DIR/commands/rename.sh" "$@"
    source "$SCRIPT_DIR/commands/organize_camera.sh" "$@"
    source "$SCRIPT_DIR/commands/organize_shots.sh" "$@"
    source "$SCRIPT_DIR/commands/organize_wechat.sh" "$@"
    log_success "完整工作流执行完毕"
}

main "$@"
```

> **关键原则**：每个 `commands/*.sh` 文件只包含业务逻辑函数，**不在顶层执行任何操作**（不要在文件顶层调用 `main()` 或直接运行命令），让 `src/main.sh` 统一控制执行时机。

---

## 3. 方案 A：shc 编译为原生二进制

`shc`（Shell Script Compiler）将 Bash 脚本编译为 C 代码，再通过 `gcc` 编译为原生 ELF 可执行文件。

### 3.1 安装 shc

```bash
# Ubuntu / Debian / WSL
sudo apt update && sudo apt install -y shc

# 或从源码编译（获取最新版本）
git clone https://github.com/neurobin/shc.git
cd shc && autoreconf -i && ./configure && make && sudo make install
```

### 3.2 编译步骤

**第一步：合并所有脚本为单文件**

shc 只能处理单个脚本文件，因此需要先将所有 `source` 引用的文件内联（inline）进去：

```bash
# scripts/bundle.sh — 合并脚本
#!/bin/bash
# 将 src/ 下所有脚本合并为单文件，inline 所有 source 引用

OUTPUT="build/gphotoh_bundled.sh"
mkdir -p build

# 写入 shebang 和头部
echo '#!/bin/bash' > "$OUTPUT"
echo '# g_photo_take_out_helper v'"$VERSION"' (bundled)' >> "$OUTPUT"

# 内联公共库
echo '# === lib/common.sh ===' >> "$OUTPUT"
grep -v '^#!/' src/lib/common.sh >> "$OUTPUT"

echo '# === lib/version.sh ===' >> "$OUTPUT"
grep -v '^#!/' src/lib/version.sh >> "$OUTPUT"

# 内联各命令模块
for cmd_file in src/commands/*.sh; do
    echo "# === $(basename $cmd_file) ===" >> "$OUTPUT"
    grep -v '^#!/' "$cmd_file" >> "$OUTPUT"
done

# 内联主入口（去掉 source 语句，因为已 inline）
echo '# === main ===' >> "$OUTPUT"
grep -v '^source ' src/main.sh | grep -v '^#!/' >> "$OUTPUT"

chmod +x "$OUTPUT"
echo "✅ 合并完成: $OUTPUT"
```

**第二步：用 shc 编译**

```bash
# 编译（-f 指定输入，-o 指定输出二进制名）
shc -f build/gphotoh_bundled.sh -o dist/gphotoh

# 常用选项:
# -r   运行时不检查脚本是否被修改（允许在其他机器运行）
# -T   允许跟踪（保留调试能力）
# -e   设置过期日期（格式: dd/mm/yyyy）
shc -r -f build/gphotoh_bundled.sh -o dist/gphotoh
```

**第三步：验证**

```bash
./dist/gphotoh --version
./dist/gphotoh --help
./dist/gphotoh fix-time --dry-run
```

**产物说明**

shc 会生成两个文件：
- `dist/gphotoh` — 可执行二进制
- `build/gphotoh_bundled.sh.x.c` — 中间 C 代码（可删除）

### 3.3 处理外部依赖（jq 内嵌）

shc 编译后的二进制在运行时仍然依赖系统中安装的 `jq`。若要真正无依赖分发，需要内嵌静态 jq：

```bash
# 下载静态编译的 jq 二进制
JQ_VERSION="1.7.1"
curl -Lo build/jq_static \
    "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-amd64"
chmod +x build/jq_static

# 在 bundle 脚本中，将 jq 二进制以 base64 内嵌到脚本头部
JQ_B64=$(base64 -w0 build/jq_static)
cat >> build/gphotoh_bundled.sh <<EOF

# ── 内嵌 jq 二进制（运行时解压到临时目录）────────────────────────────────────
_setup_embedded_jq() {
    local tmp_dir="\$(mktemp -d)"
    echo "${JQ_B64}" | base64 -d > "\$tmp_dir/jq"
    chmod +x "\$tmp_dir/jq"
    export PATH="\$tmp_dir:\$PATH"
    trap "rm -rf '\$tmp_dir'" EXIT
}
_setup_embedded_jq
EOF
```

> ⚠️ 内嵌 jq 会显著增大二进制体积（约 +2MB）。仅在需要完全离线分发时使用。

### 3.4 shc 的限制与注意事项

| 限制 | 说明 |
|------|------|
| 架构绑定 | 在 x86_64 上编译的二进制无法直接运行在 ARM64。需要分架构编译并发布。 |
| Bash 版本 | 运行机器的 Bash 版本需 ≥ 编译时使用的版本（建议目标 Bash ≥ 4.0）。 |
| 防逆向能力有限 | shc 产物可被 `strings`、`strace` 等工具分析，并非真正加密。 |
| 调试困难 | 崩溃时只有 C 层的错误，需保留 `.sh` 源码版本用于开发调试。 |
| `source` 不支持 | 编译后不能再 `source` 引用外部文件，必须 bundle 为单文件后再编译。 |

---

## 4. 方案 B：makeself 自解压包（推荐）

makeself 将目录打包为一个可自解压的 Shell 脚本文件（`.run`），运行时自动解压到临时目录并执行指定命令。**这是最成熟、最兼容的 Shell 项目分发方案。**

### 4.1 安装 makeself

```bash
# Ubuntu / Debian
sudo apt install -y makeself

# 或直接下载单文件版本（无需安装）
curl -Lo /usr/local/bin/makeself \
    https://github.com/megastep/makeself/releases/latest/download/makeself.run
chmod +x /usr/local/bin/makeself
```

### 4.2 构建目录结构

makeself 需要一个**打包目录**，将所有运行时所需文件放入其中：

```bash
# scripts/prepare_dist.sh
#!/bin/bash
set -euo pipefail

VERSION="${VERSION:-1.0.0}"
DIST_DIR="build/dist_pkg"

# 清理并重建打包目录
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR/bin" "$DIST_DIR/lib" "$DIST_DIR/commands"

# 复制脚本文件
cp src/main.sh        "$DIST_DIR/bin/gphotoh"
cp src/lib/*.sh       "$DIST_DIR/lib/"
cp src/commands/*.sh  "$DIST_DIR/commands/"

# （可选）内嵌静态 jq
if [[ -f "build/jq_static" ]]; then
    cp build/jq_static "$DIST_DIR/bin/jq"
fi

# 写入版本文件
echo "$VERSION" > "$DIST_DIR/VERSION"

# 写入安装脚本（makeself 解压后运行的入口）
cat > "$DIST_DIR/install.sh" <<'EOF'
#!/bin/bash
# makeself 解压后自动执行此脚本

INSTALL_DIR="${HOME}/.local/bin"
SCRIPT_INSTALL_DIR="${HOME}/.local/lib/gphotoh"

mkdir -p "$INSTALL_DIR" "$SCRIPT_INSTALL_DIR"

# 复制文件
cp -r commands lib bin/gphotoh "$SCRIPT_INSTALL_DIR/"
[[ -f bin/jq ]] && cp bin/jq "$INSTALL_DIR/jq"

# 创建启动包装器
cat > "$INSTALL_DIR/gphotoh" <<WRAPPER
#!/bin/bash
export GPHOTOH_LIB="${SCRIPT_INSTALL_DIR}"
exec bash "${SCRIPT_INSTALL_DIR}/gphotoh" "\$@"
WRAPPER
chmod +x "$INSTALL_DIR/gphotoh"

echo "✅ 安装完成！"
echo "   二进制位置: $INSTALL_DIR/gphotoh"
echo "   确保 $INSTALL_DIR 在您的 PATH 中"
EOF
chmod +x "$DIST_DIR/install.sh"

echo "✅ 打包目录准备完毕: $DIST_DIR"
```

### 4.3 打包命令

```bash
VERSION="1.0.0"

makeself \
    --nomd5 \
    --nocrc \
    --tar-extra "--exclude='.git'" \
    build/dist_pkg \
    "dist/gphotoh-${VERSION}-linux-x86_64.run" \
    "g_photo_take_out_helper v${VERSION}" \
    ./install.sh
```

**参数说明**

| 参数 | 说明 |
|------|------|
| `build/dist_pkg` | 要打包的目录 |
| `dist/gphotoh-*.run` | 输出的 `.run` 文件路径 |
| `"g_photo_take_out_helper ..."` | 解压时显示的标题 |
| `./install.sh` | 解压后自动执行的脚本 |
| `--nomd5 --nocrc` | 跳过校验（可选，减少启动时间）|

**不安装，直接运行（portable 模式）**

如果不想安装到系统，可以在 `install.sh` 里改为直接运行：

```bash
# install.sh 改为 run.sh
cat > "$DIST_DIR/run.sh" <<'EOF'
#!/bin/bash
# 直接在解压目录运行，不安装到系统
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "$SCRIPT_DIR/bin/gphotoh" "$@"
EOF

# 打包命令改为：
makeself build/dist_pkg "dist/gphotoh.run" "gphotoh" ./run.sh
```

用户直接运行：
```bash
./gphotoh.run fix-time --dry-run
```

### 4.4 发布与安装流程

**用户安装**
```bash
# 下载
curl -Lo gphotoh.run https://github.com/your-repo/releases/latest/download/gphotoh.run

# 赋予执行权限并安装
chmod +x gphotoh.run
./gphotoh.run

# 使用
gphotoh --help
gphotoh fix-time --dry-run
```

**用户卸载**
```bash
rm -rf ~/.local/lib/gphotoh
rm ~/.local/bin/gphotoh
```

---

## 5. 方案 C：静态 Shell + bpkg/basher 包管理（开发者友好）

适合面向技术用户的分发，提供源码级的包管理体验。

```bash
# 用户通过 basher 安装（类似 npm install）
basher install bingzujia/g_photo_take_out_helper

# 或通过 bpkg
bpkg install bingzujia/g_photo_take_out_helper
```

**需要在仓库根目录添加 `package.sh`**（basher 的包描述文件）：

```bash
# package.sh
PACKAGE_NAME="gphotoh"
PACKAGE_VERSION="1.0.0"
PACKAGE_DESCRIPTION="Google Takeout 照片整理工具"
PACKAGE_DEPENDENCIES="jq"
BINS="src/main.sh:gphotoh"
```

---

## 6. 自动化构建：Makefile

在项目根目录创建 `Makefile`，统一管理所有构建步骤：

```makefile
# Makefile for g_photo_take_out_helper

VERSION     ?= 1.0.0
DIST_DIR    := dist
BUILD_DIR   := build
BIN_NAME    := gphotoh
ARCH        := $(shell uname -m)

.PHONY: all clean bundle shc makeself test install

# 默认目标：输出帮助
all:
	@echo "可用目标:"
	@echo "  make bundle    — 将所有脚本合并为单文件"
	@echo "  make shc       — 用 shc 编译为原生二进制"
	@echo "  make makeself  — 用 makeself 打包为 .run 文件"
	@echo "  make test      — 运行冒烟测试"
	@echo "  make clean     — 清理构建产物"
	@echo "  make install   — 安装到 ~/.local/bin"

# 合并脚本
bundle: | $(BUILD_DIR)
	@echo "🔧 合并脚本..."
	bash scripts/bundle.sh
	@echo "✅ 合并完成: $(BUILD_DIR)/$(BIN_NAME)_bundled.sh"

# shc 编译为二进制
shc: bundle | $(DIST_DIR)
	@command -v shc >/dev/null || { echo "❌ 请先安装 shc: sudo apt install shc"; exit 1; }
	@echo "⚙️  编译中..."
	shc -r -f $(BUILD_DIR)/$(BIN_NAME)_bundled.sh -o $(DIST_DIR)/$(BIN_NAME)-$(VERSION)-linux-$(ARCH)
	@rm -f $(BUILD_DIR)/$(BIN_NAME)_bundled.sh.x.c
	@echo "✅ 二进制: $(DIST_DIR)/$(BIN_NAME)-$(VERSION)-linux-$(ARCH)"

# makeself 自解压包
makeself: | $(DIST_DIR)
	@command -v makeself >/dev/null || { echo "❌ 请先安装 makeself: sudo apt install makeself"; exit 1; }
	@echo "📦 打包中..."
	bash scripts/prepare_dist.sh
	makeself --nomd5 --nocrc \
		$(BUILD_DIR)/dist_pkg \
		$(DIST_DIR)/$(BIN_NAME)-$(VERSION)-linux-$(ARCH).run \
		"g_photo_take_out_helper v$(VERSION)" \
		./install.sh
	@echo "✅ 包文件: $(DIST_DIR)/$(BIN_NAME)-$(VERSION)-linux-$(ARCH).run"

# 冒烟测试
test:
	@echo "🧪 运行冒烟测试..."
	bash tests/smoke_test.sh

# 安装到用户目录
install: makeself
	$(DIST_DIR)/$(BIN_NAME)-$(VERSION)-linux-$(ARCH).run

# 清理
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(DIST_DIR):
	mkdir -p $(DIST_DIR)
```

**使用方式**

```bash
# 构建 makeself 包（推荐）
make makeself VERSION=1.2.0

# 构建 shc 二进制
make shc VERSION=1.2.0

# 清理
make clean
```

---

## 7. 依赖管理策略

### 7.1 运行时检测依赖

在 `src/lib/common.sh` 中集中管理依赖检测：

```bash
# src/lib/common.sh

# 检查必要依赖
check_dependencies() {
    local missing=()
    local required=("find" "touch" "date" "stat")
    
    for cmd in "${required[@]}"; do
        command -v "$cmd" &>/dev/null || missing+=("$cmd")
    done
    
    # jq 只在 fix-time 命令中需要
    # 在命令入口处单独检查，避免其他命令报错
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "缺少必要工具: ${missing[*]}"
        log_error "请安装后重试"
        exit 1
    fi
}

# 检查 jq（仅在需要时调用）
require_jq() {
    if ! command -v jq &>/dev/null; then
        # 优先使用内嵌 jq（makeself 分发时会内嵌）
        if [[ -f "${GPHOTOH_LIB}/bin/jq" ]]; then
            export PATH="${GPHOTOH_LIB}/bin:$PATH"
        else
            log_error "未找到 jq。请运行: sudo apt install jq"
            exit 1
        fi
    fi
}
```

### 7.2 内嵌静态 jq 二进制

**下载静态 jq**

```bash
# Makefile 中添加
fetch-jq:
	@mkdir -p $(BUILD_DIR)
	@JQ_VER="1.7.1"; ARCH=$(shell uname -m); \
	ARCH_NAME=$$([ "$$ARCH" = "x86_64" ] && echo "amd64" || echo "arm64"); \
	curl -Lo $(BUILD_DIR)/jq_static \
	    "https://github.com/jqlang/jq/releases/download/jq-$${JQ_VER}/jq-linux-$${ARCH_NAME}"; \
	chmod +x $(BUILD_DIR)/jq_static
	@echo "✅ 静态 jq 下载完成"
```

**安全检查（验证下载完整性）**

```bash
# 验证 jq 可执行
$(BUILD_DIR)/jq_static --version || { echo "❌ jq 下载验证失败"; exit 1; }
```

---

## 8. 版本号管理

在 `src/lib/version.sh` 中统一管理版本信息：

```bash
# src/lib/version.sh
GPHOTOH_VERSION="1.0.0"
GPHOTOH_BUILD_DATE="$(date +%Y-%m-%d)"
GPHOTOH_GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
```

**在 bundle 时将版本信息硬编码进去**（避免运行时依赖 git）：

```bash
# scripts/bundle.sh 中处理版本
VERSION="${VERSION:-1.0.0}"
GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
BUILD_DATE="$(date +%Y-%m-%d)"

sed -i \
    -e "s/GPHOTOH_VERSION=.*/GPHOTOH_VERSION=\"${VERSION}\"/" \
    -e "s/GPHOTOH_BUILD_DATE=.*/GPHOTOH_BUILD_DATE=\"${BUILD_DATE}\"/" \
    -e "s/GPHOTOH_GIT_COMMIT=.*/GPHOTOH_GIT_COMMIT=\"${GIT_COMMIT}\"/" \
    "$OUTPUT"
```

---

## 9. 二次开发工作流

### 9.1 源码修改 → 重新打包流程

```
1. 在 src/ 目录修改源码（永远不要修改 build/ 或 dist/ 中的文件）
        ↓
2. 本地测试（使用 --dry-run）
   bash src/main.sh fix-time --dry-run
        ↓
3. 运行测试套件
   make test
        ↓
4. 打包
   make makeself VERSION=x.y.z
        ↓
5. 验证产物
   ./dist/gphotoh-x.y.z-linux-x86_64.run
        ↓
6. 提交代码，发布 GitHub Release（附带 .run 文件）
```

**永远基于源码开发，从不修改产物文件。**

### 9.2 解包已发布 bin 文件进行调试

**makeself 包解包**

```bash
# 解包但不安装（查看内容）
./gphotoh.run --noexec --target /tmp/gphotoh_extracted

# 查看提取出的文件
ls /tmp/gphotoh_extracted/

# 直接运行（不安装）
bash /tmp/gphotoh_extracted/bin/gphotoh --help
```

**shc 二进制调试**

shc 二进制无法直接解包，但可以：

```bash
# 查看二进制中嵌入的字符串（辅助调试）
strings ./gphotoh | grep -E '(ERROR|WARNING|bash|jq)'

# 使用 strace 追踪系统调用（观察文件操作行为）
strace -e trace=file ./gphotoh fix-time --dry-run 2>&1 | head -50

# 始终保留对应版本的源码用于调试
git checkout v1.0.0  # 切换到对应版本的源码
```

### 9.3 新增子命令

以新增 `gphotoh dedup`（去除重复图片）命令为例：

**Step 1：创建命令模块** `src/commands/dedup.sh`

```bash
#!/bin/bash
# 命令: gphotoh dedup — 检测并删除重复图片

# 注意：顶层不执行任何操作，所有逻辑封装在函数中
# 由 src/main.sh source 后调用

# ── 仅在被直接 source 调用时执行 ─────────────────────────────────────────────
_dedup_main() {
    local dry_run=false
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -d|--dry-run) dry_run=true; shift ;;
            -h|--help)    _dedup_usage; return 0 ;;
            *) log_error "未知参数: $1"; return 1 ;;
        esac
    done
    
    log_info "开始检测重复图片..."
    # ... 去重逻辑 ...
}

_dedup_usage() {
    echo "用法: gphotoh dedup [--dry-run]"
}

# 执行入口
_dedup_main "$@"
```

**Step 2：在 `src/main.sh` 的路由表中注册**

```bash
case "$command" in
    # ... 已有命令 ...
    dedup)  source "$SCRIPT_DIR/commands/dedup.sh" "$@" ;;   # ← 新增
    # ...
esac
```

**Step 3：更新帮助信息**

```bash
# show_help() 中添加
echo "  dedup             检测并删除重复图片"
```

**Step 4：打包测试**

```bash
make makeself VERSION=1.1.0
./dist/gphotoh-1.1.0-linux-x86_64.run  # 安装
gphotoh dedup --dry-run                 # 测试
```

### 9.4 修改已有子命令逻辑

以修改 `fix-time` 命令新增 `--timezone` 参数为例：

```bash
# 修改 src/commands/fix_time.sh

# 在参数解析部分添加：
-t|--timezone)
    TIMEZONE="$2"
    shift 2
    ;;

# 在时间戳转换处应用时区：
readable_time=$(TZ="$TIMEZONE" date -d "@$timestamp" '+%Y-%m-%d %H:%M:%S')
```

修改后流程：
1. 本地测试：`bash src/main.sh fix-time --timezone "Asia/Shanghai" --dry-run`
2. 重新打包：`make makeself VERSION=1.0.1`
3. 验证：安装并运行新版本

---

## 10. 测试策略

创建 `tests/smoke_test.sh` 进行冒烟测试：

```bash
#!/bin/bash
# tests/smoke_test.sh — 冒烟测试

set -euo pipefail
PASS=0; FAIL=0
BIN="${1:-bash src/main.sh}"

run_test() {
    local name="$1"; shift
    if eval "$BIN $@" > /dev/null 2>&1; then
        echo "✅ $name"
        ((PASS++))
    else
        echo "❌ $name"
        ((FAIL++))
    fi
}

echo "🧪 开始冒烟测试..."

# 基础命令测试
run_test "help"         "--help"
run_test "version"      "--version"
run_test "fix-time-help" "fix-time --help"
run_test "rename-help"   "rename --help"

# dry-run 测试（在空目录中运行，不应报错）
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
run_test "fix-time dry-run"  "fix-time --dry-run"
run_test "rename dry-run"    "rename --dry-run"
run_test "delete-json dry-run" "delete-json --dry-run"
cd - > /dev/null
rm -rf "$TMPDIR"

# 未知命令应返回非零退出码
if ! eval "$BIN unknown-command" > /dev/null 2>&1; then
    echo "✅ unknown-command exits non-zero"
    ((PASS++))
else
    echo "❌ unknown-command should exit non-zero"
    ((FAIL++))
fi

echo ""
echo "测试结果: $PASS 通过, $FAIL 失败"
[[ $FAIL -eq 0 ]]
```

在 Makefile 中注册：

```makefile
test:
	bash tests/smoke_test.sh "bash src/main.sh"
test-bin: makeself
	bash tests/smoke_test.sh "./dist/gphotoh-$(VERSION)-linux-$(ARCH).run --"
```

---

## 11. 发布检查清单

在每次发布新版本前，按此清单逐项确认：

```
源码质量
  [ ] 所有新功能有对应的 --dry-run 支持
  [ ] src/lib/version.sh 中的版本号已更新
  [ ] CHANGELOG.md 已记录本次变更

构建验证
  [ ] make clean && make makeself 构建无报错
  [ ] make test 所有测试通过
  [ ] ./dist/*.run --help 输出正确版本号

功能验证
  [ ] 在 ext4 文件系统测试（正常路径）
  [ ] 在 NTFS 挂载（WSL）测试（时间戳失败应给出友好提示）
  [ ] --dry-run 模式不产生任何文件修改

分发准备
  [ ] 构建 x86_64 版本
  [ ] （如需）构建 ARM64 版本（在对应机器或交叉编译）
  [ ] 计算 SHA256 校验和并附在 Release Notes 中
  [ ] 上传到 GitHub Releases
```

**计算校验和**

```bash
sha256sum dist/gphotoh-*.run > dist/SHA256SUMS
cat dist/SHA256SUMS
```

---

## 12. 常见问题

### Q1：shc 编译后在其他机器运行报 `bash: /proc/...`  错误

**原因**：shc 在解密阶段依赖当前机器的 `/proc/self/...` 路径（反逆向机制）。不同内核版本或 chroot 环境可能不兼容。

**解决**：使用 `-r` 参数编译（relax 安全检查）：

```bash
shc -r -f build/gphotoh_bundled.sh -o dist/gphotoh
```

### Q2：makeself 包运行时报 `Cannot change to directory`

**原因**：解压目录权限问题，或 `/tmp` 空间不足。

**解决**：

```bash
# 指定自定义解压目录
TMPDIR=~/tmp ./gphotoh.run

# 或修改 makeself 打包时指定解压目录
makeself --target /opt/gphotoh build/dist_pkg ...
```

### Q3：二进制分发后用户无法运行，提示 `jq not found`

**原因**：未内嵌 jq，且目标机器未安装。

**解决方案（选一）**：
- 方案 1：`make fetch-jq && make makeself`（内嵌静态 jq）
- 方案 2：在 `install.sh` 中自动安装 jq：

```bash
if ! command -v jq &>/dev/null; then
    sudo apt-get install -y jq 2>/dev/null || \
    sudo yum install -y jq 2>/dev/null || \
    { echo "❌ 请手动安装 jq"; exit 1; }
fi
```

### Q4：如何支持 macOS 用户

macOS 的 `date` 和 `stat` 命令参数与 GNU 不兼容。在 `src/lib/common.sh` 中添加平台检测：

```bash
if [[ "$(uname)" == "Darwin" ]]; then
    # macOS：使用 BSD 版本参数
    get_unix_timestamp() { date -j -f "%Y-%m-%d %H:%M:%S" "$1" +%s; }
    get_file_mtime()     { stat -f "%m" "$1"; }
else
    # Linux：使用 GNU 版本参数  
    get_unix_timestamp() { date -d "$1" +%s; }
    get_file_mtime()     { stat -c "%Y" "$1"; }
fi
```

makeself 在 macOS 上同样可用（`.run` 文件本质是 Shell 脚本）。

### Q5：如何在 CI/CD（GitHub Actions）中自动构建

创建 `.github/workflows/release.yml`：

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install tools
        run: sudo apt-get install -y makeself shc
      
      - name: Get version
        run: echo "VERSION=${GITHUB_REF_NAME#v}" >> $GITHUB_ENV
      
      - name: Build makeself package
        run: make makeself VERSION=${{ env.VERSION }}
      
      - name: Build shc binary
        run: make shc VERSION=${{ env.VERSION }}
      
      - name: Compute checksums
        run: sha256sum dist/* > dist/SHA256SUMS
      
      - name: Upload to GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/*
```

推送 tag 即自动发布：

```bash
git tag v1.2.0
git push origin v1.2.0
```

### Q6：如何让 bin 文件支持自更新

在 `src/commands/update.sh` 中实现：

```bash
_update_main() {
    local latest
    latest=$(curl -s https://api.github.com/repos/bingzujia/g_photo_take_out_helper/releases/latest \
        | grep '"tag_name"' | cut -d'"' -f4)
    
    echo "当前版本: v${GPHOTOH_VERSION}"
    echo "最新版本: $latest"
    
    if [[ "v${GPHOTOH_VERSION}" == "$latest" ]]; then
        echo "✅ 已是最新版本"
        return 0
    fi
    
    local url="https://github.com/bingzujia/g_photo_take_out_helper/releases/download/${latest}/gphotoh-linux-x86_64.run"
    echo "📥 下载 $latest ..."
    curl -Lo /tmp/gphotoh_update.run "$url"
    chmod +x /tmp/gphotoh_update.run
    /tmp/gphotoh_update.run  # 运行新版本安装脚本
    rm -f /tmp/gphotoh_update.run
    echo "✅ 更新完成，请重新运行 gphotoh"
}
```

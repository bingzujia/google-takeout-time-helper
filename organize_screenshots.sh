#!/bin/bash

# 整理截图脚本 - 将包含screenshot的文件移动到screenshot目录
# 作者: GitHub Copilot
# 日期: 2025-08-14

# 定义颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 获取当前脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"

log_info "开始整理截图..."
log_info "基础目录: $BASE_DIR"

# 创建screenshot目录
SCREENSHOT_DIR="$SCRIPT_DIR/screenshot"
if [ ! -d "$SCREENSHOT_DIR" ]; then
    mkdir -p "$SCREENSHOT_DIR"
    log_success "创建screenshot目录: $SCREENSHOT_DIR"
else
    log_info "screenshot目录已存在: $SCREENSHOT_DIR"
fi

# 支持的文件扩展名（不区分大小写）
declare -a EXTENSIONS=(
    "jpg" "jpeg" "png" "gif" "bmp" "tiff" "tif" "heic" "heif"
    "webp" "avif"
    "JPG" "JPEG" "PNG" "GIF" "BMP" "TIFF" "TIF" "HEIC" "HEIF"
    "WEBP" "AVIF"
)

# 计数器
total_moved=0
total_skipped=0

# 检查文件是否为截图文件
is_screenshot_file() {
    local filename="$1"
    local basename=$(basename "$filename")
    
    # 检查文件扩展名
    local extension="${basename##*.}"
    local ext_match=false
    
    for ext in "${EXTENSIONS[@]}"; do
        if [[ "$extension" == "$ext" ]]; then
            ext_match=true
            break
        fi
    done
    
    if [ "$ext_match" = false ]; then
        return 1
    fi
    
    # 检查文件名是否包含screenshot（不区分大小写）
    if [[ "${basename,,}" == *"screenshot"* ]]; then
        return 0
    fi
    
    return 1
}

# 移动文件到screenshot目录
move_to_screenshot() {
    local source_file="$1"
    local filename=$(basename "$source_file")
    local destination="$SCREENSHOT_DIR/$filename"
    
    # 检查目标文件是否已存在
    if [ -f "$destination" ]; then
        # 生成新的文件名（添加时间戳）
        local name_without_ext="${filename%.*}"
        local extension="${filename##*.}"
        local timestamp=$(date +"%Y%m%d_%H%M%S")
        destination="$SCREENSHOT_DIR/${name_without_ext}_${timestamp}.${extension}"
        log_warning "目标文件已存在，重命名为: $(basename "$destination")"
    fi
    
    # 移动文件
    if mv "$source_file" "$destination"; then
        log_success "移动: $filename -> screenshot/"
        ((total_moved++))
    else
        log_error "移动失败: $filename"
    fi
}

# 处理当前目录
process_current_directory() {
    log_info "处理当前目录: $SCRIPT_DIR"
    
    for file in "$SCRIPT_DIR"/*; do
        if [ -f "$file" ]; then
            if is_screenshot_file "$file"; then
                # 避免移动脚本自身
                if [[ $(basename "$file") != "organize_screenshots.sh" ]]; then
                    move_to_screenshot "$file"
                fi
            else
                ((total_skipped++))
            fi
        fi
    done
}

# 处理Photos开头的目录
process_photos_directories() {
    log_info "搜索Photos开头的目录..."
    
    # 在基础目录中查找Photos开头的目录
    find "$BASE_DIR" -maxdepth 2 -type d -name "Photos*" | while read -r photos_dir; do
        if [ -d "$photos_dir" ]; then
            log_info "处理目录: $photos_dir"
            
            # 遍历该目录下的所有文件
            find "$photos_dir" -type f | while read -r file; do
                if is_screenshot_file "$file"; then
                    move_to_screenshot "$file"
                else
                    ((total_skipped++))
                fi
            done
        fi
    done
}

# 显示使用说明
show_usage() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示此帮助信息"
    echo "  -c, --current  仅处理当前目录"
    echo "  -a, --all      处理所有Photos目录（默认）"
    echo "  -d, --dry-run  模拟运行，不实际移动文件"
    echo "  -l, --list     列出所有包含screenshot的文件"
    echo ""
    echo "支持的扩展名:"
    printf "  "
    printf "%s " "${EXTENSIONS[@]}"
    echo ""
    echo ""
    echo "匹配规则:"
    echo "  文件名包含'screenshot'（不区分大小写）"
    echo "  例如: Screenshot_20231010_123456.png"
    echo "       screenshot (1).jpg"
    echo "       My_Screenshot_Image.png"
}

# 列出所有截图文件
list_screenshots() {
    log_info "搜索所有包含screenshot的文件..."
    
    # 当前目录
    for file in "$SCRIPT_DIR"/*; do
        if [ -f "$file" ] && is_screenshot_file "$file"; then
            echo "当前目录: $(basename "$file")"
        fi
    done
    
    # Photos目录
    find "$BASE_DIR" -maxdepth 2 -type d -name "Photos*" | while read -r photos_dir; do
        if [ -d "$photos_dir" ]; then
            find "$photos_dir" -type f | while read -r file; do
                if is_screenshot_file "$file"; then
                    echo "$(basename "$photos_dir"): $(basename "$file")"
                fi
            done
        fi
    done
}

# 模拟运行模式
dry_run=false
current_only=false
list_only=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -c|--current)
            current_only=true
            shift
            ;;
        -a|--all)
            current_only=false
            shift
            ;;
        -d|--dry-run)
            dry_run=true
            shift
            ;;
        -l|--list)
            list_only=true
            shift
            ;;
        *)
            log_error "未知选项: $1"
            show_usage
            exit 1
            ;;
    esac
done

# 如果只是列出文件
if [ "$list_only" = true ]; then
    list_screenshots
    exit 0
fi

# 如果是模拟运行
if [ "$dry_run" = true ]; then
    log_warning "模拟运行模式 - 不会实际移动文件"
    # 重定义移动函数为只显示信息
    move_to_screenshot() {
        local source_file="$1"
        local filename=$(basename "$source_file")
        log_info "将移动: $filename -> screenshot/"
        ((total_moved++))
    }
fi

# 主执行逻辑
main() {
    log_info "开始执行截图整理任务..."
    
    if [ "$current_only" = true ]; then
        process_current_directory
    else
        process_current_directory
        process_photos_directories
    fi
    
    # 显示统计信息
    echo ""
    log_info "整理完成！"
    log_success "成功移动文件: $total_moved"
    log_info "跳过文件: $total_skipped"
    
    if [ "$dry_run" = true ]; then
        log_warning "这是模拟运行，未实际移动文件"
    fi
}

# 执行主函数
main

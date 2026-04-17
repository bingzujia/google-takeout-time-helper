#!/bin/bash

# 整理图片脚本 - 将手机拍摄的照片移动到camera目录
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

log_info "开始整理图片..."
log_info "基础目录: $BASE_DIR"

# 创建camera目录
CAMERA_DIR="$SCRIPT_DIR/camera"
if [ ! -d "$CAMERA_DIR" ]; then
    mkdir -p "$CAMERA_DIR"
    log_success "创建camera目录: $CAMERA_DIR"
else
    log_info "camera目录已存在: $CAMERA_DIR"
fi

# 定义手机拍摄文件的模式
# 支持的前缀模式
declare -a PATTERNS=(
    "WP_*"           # Windows Phone
    "IMG_*"          # 通用图片前缀
    "VID_*"          # 视频文件
    "P_*"            # 某些手机的照片前缀
    "PXL_*"            # 某些手机的照片前缀
    "DSC_*"          # 数码相机常用前缀
    "IMG*"           # IMG开头（无下划线）
    "[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]_[0-9][0-9][0-9][0-9][0-9][0-9]*" # YYYYMMDD_HHmmss格式
)

# 支持的文件扩展名（不区分大小写）
declare -a EXTENSIONS=(
    "jpg" "jpeg" "png" "gif" "bmp" "tiff" "tif" "heic" "heif"
    "mp4" "mov" "avi" "mkv" "wmv" "flv" "3gp"
    "JPG" "JPEG" "PNG" "GIF" "BMP" "TIFF" "TIF" "HEIC" "HEIF"
    "MP4" "MOV" "AVI" "MKV" "WMV" "FLV" "3GP"
)

# 计数器
total_moved=0
total_skipped=0

# 检查文件是否匹配手机拍摄模式
is_camera_file() {
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
    
    # 检查文件名模式
    for pattern in "${PATTERNS[@]}"; do
        if [[ "$basename" == $pattern ]]; then
            return 0
        fi
    done
    
    return 1
}

# 移动文件到camera目录
move_to_camera() {
    local source_file="$1"
    local filename=$(basename "$source_file")
    local destination="$CAMERA_DIR/$filename"
    
    # 检查目标文件是否已存在
    if [ -f "$destination" ]; then
        # 生成新的文件名（添加时间戳）
        local name_without_ext="${filename%.*}"
        local extension="${filename##*.}"
        local timestamp=$(date +"%Y%m%d_%H%M%S")
        destination="$CAMERA_DIR/${name_without_ext}_${timestamp}.${extension}"
        log_warning "目标文件已存在，重命名为: $(basename "$destination")"
    fi
    
    # 移动文件
    if mv "$source_file" "$destination"; then
        log_success "移动: $filename -> camera/"
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
            if is_camera_file "$file"; then
                # 避免移动脚本自身
                if [[ $(basename "$file") != "organize_photos.sh" ]]; then
                    move_to_camera "$file"
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
                if is_camera_file "$file"; then
                    move_to_camera "$file"
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
    echo ""
    echo "支持的文件模式:"
    printf "  前缀: "
    printf "%s " "${PATTERNS[@]}"
    echo ""
    printf "  扩展名: "
    printf "%s " "${EXTENSIONS[@]}"
    echo ""
}

# 模拟运行模式
dry_run=false
current_only=false

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
        *)
            log_error "未知选项: $1"
            show_usage
            exit 1
            ;;
    esac
done

# 如果是模拟运行
if [ "$dry_run" = true ]; then
    log_warning "模拟运行模式 - 不会实际移动文件"
    # 重定义移动函数为只显示信息
    move_to_camera() {
        local source_file="$1"
        local filename=$(basename "$source_file")
        log_info "将移动: $filename -> camera/"
        ((total_moved++))
    }
fi

# 主执行逻辑
main() {
    log_info "开始执行图片整理任务..."
    
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

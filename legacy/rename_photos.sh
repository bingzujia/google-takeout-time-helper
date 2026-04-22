#!/bin/bash

# 照片文件重命名脚本 - 根据创建时间重命名为IMG格式
# 作者: GitHub Copilot
# 日期: 2025-08-14
# 格式: IMGYYYYMMDDHHMMSS (例如: IMG20250727124825)

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

log_info "开始重命名照片文件..."
log_info "工作目录: $SCRIPT_DIR"

# 支持的图片文件扩展名（不区分大小写）
declare -a IMAGE_EXTENSIONS=(
    "jpg" "jpeg" "png" "gif" "bmp" "tiff" "tif" "heic" "heif"
    "webp" "avif" "raw" "cr2" "nef" "arw" "dng"
    "JPG" "JPEG" "PNG" "GIF" "BMP" "TIFF" "TIF" "HEIC" "HEIF"
    "WEBP" "AVIF" "RAW" "CR2" "NEF" "ARW" "DNG"
)

# 支持的视频文件扩展名（不区分大小写）
declare -a VIDEO_EXTENSIONS=(
    "mp4" "mov" "avi" "mkv" "wmv" "flv" "3gp" "m4v" "webm"
    "mpg" "mpeg" "asf" "rm" "rmvb" "vob" "ts" "mts" "m2ts"
    "MP4" "MOV" "AVI" "MKV" "WMV" "FLV" "3GP" "M4V" "WEBM"
    "MPG" "MPEG" "ASF" "RM" "RMVB" "VOB" "TS" "MTS" "M2TS"
)

# 计数器
total_renamed=0
total_skipped=0
total_errors=0

# 检查文件是否为图片文件
is_image_file() {
    local filename="$1"
    local basename=$(basename "$filename")
    
    # 检查文件扩展名
    local extension="${basename##*.}"
    
    for ext in "${IMAGE_EXTENSIONS[@]}"; do
        if [[ "$extension" == "$ext" ]]; then
            return 0
        fi
    done
    
    return 1
}

# 检查文件是否为视频文件
is_video_file() {
    local filename="$1"
    local basename=$(basename "$filename")
    
    # 检查文件扩展名
    local extension="${basename##*.}"
    
    for ext in "${VIDEO_EXTENSIONS[@]}"; do
        if [[ "$extension" == "$ext" ]]; then
            return 0
        fi
    done
    
    return 1
}

# 检查文件是否为媒体文件（图片或视频）
is_media_file() {
    local filename="$1"
    
    if is_image_file "$filename" || is_video_file "$filename"; then
        return 0
    fi
    
    return 1
}

# 获取文件的创建时间（使用修改时间作为创建时间）
get_file_creation_time() {
    local file="$1"
    
    # 在Linux上，使用stat命令获取文件的修改时间
    # 格式: YYYYMMDDHHMMSS
    if command -v stat >/dev/null 2>&1; then
        # Linux/Unix系统
        stat -c "%Y" "$file" 2>/dev/null | xargs -I {} date -d "@{}" "+%Y%m%d%H%M%S" 2>/dev/null
    else
        # 备用方案：使用ls命令
        ls -l --time-style=+%Y%m%d%H%M%S "$file" 2>/dev/null | awk '{print $6}'
    fi
}

# 生成新的文件名
generate_new_filename() {
    local original_file="$1"
    local creation_time="$2"
    local extension="${original_file##*.}"
    
    # 根据文件类型选择前缀
    local prefix=""
    if is_image_file "$original_file"; then
        prefix="IMG"
    elif is_video_file "$original_file"; then
        prefix="VID"
    else
        # 默认使用IMG前缀
        prefix="IMG"
    fi
    
    # 基础文件名格式: IMGYYYYMMDDHHMMSS 或 VIDYYYYMMDDHHMMSS
    local base_name="${prefix}${creation_time}"
    local new_filename="${base_name}.${extension}"
    
    # 检查文件名冲突并在秒数上递增
    local counter=0
    local target_path="$SCRIPT_DIR/$new_filename"
    
    while [ -f "$target_path" ] && [ "$target_path" != "$original_file" ]; do
        ((counter++))
        
        # 解析时间戳各部分: YYYYMMDDHHMMSS
        local year="${creation_time:0:4}"
        local month="${creation_time:4:2}"
        local day="${creation_time:6:2}"
        local hour="${creation_time:8:2}"
        local minute="${creation_time:10:2}"
        local second="${creation_time:12:2}"
        
        # 在秒数上递增
        local new_second=$((10#$second + counter))
        
        # 处理秒数溢出（超过59秒）
        local new_minute=$((10#$minute))
        local new_hour=$((10#$hour))
        local new_day=$((10#$day))
        local new_month=$((10#$month))
        local new_year=$((10#$year))
        
        if [ $new_second -gt 59 ]; then
            new_minute=$((new_minute + new_second / 60))
            new_second=$((new_second % 60))
        fi
        
        if [ $new_minute -gt 59 ]; then
            new_hour=$((new_hour + new_minute / 60))
            new_minute=$((new_minute % 60))
        fi
        
        if [ $new_hour -gt 23 ]; then
            new_day=$((new_day + new_hour / 24))
            new_hour=$((new_hour % 24))
        fi
        
        # 简化处理：如果天数超过当月最大天数，只增加天数（不考虑月份天数差异）
        if [ $new_day -gt 31 ]; then
            new_month=$((new_month + new_day / 32))
            new_day=$((new_day % 32))
            if [ $new_day -eq 0 ]; then
                new_day=1
            fi
        fi
        
        if [ $new_month -gt 12 ]; then
            new_year=$((new_year + new_month / 13))
            new_month=$((new_month % 13))
            if [ $new_month -eq 0 ]; then
                new_month=1
            fi
        fi
        
        # 格式化新的时间戳（补零）
        local modified_time=$(printf "%04d%02d%02d%02d%02d%02d" $new_year $new_month $new_day $new_hour $new_minute $new_second)
        
        new_filename="${prefix}${modified_time}.${extension}"
        target_path="$SCRIPT_DIR/$new_filename"
        
        # 防止无限循环
        if [ $counter -gt 999 ]; then
            log_error "无法为文件生成唯一名称: $(basename "$original_file")"
            return 1
        fi
    done
    
    echo "$new_filename"
    return 0
}

# 重命名单个文件
rename_file() {
    local file="$1"
    local basename=$(basename "$file")
    
    # 跳过脚本文件
    if [[ "$basename" == *.sh ]]; then
        return 0
    fi
    
    # 检查是否为图片文件
    if ! is_media_file "$file"; then
        log_warning "跳过非媒体文件: $basename"
        ((total_skipped++))
        return 0
    fi
    
    # 获取创建时间
    local creation_time=$(get_file_creation_time "$file")
    if [ -z "$creation_time" ] || [ ${#creation_time} -ne 14 ]; then
        log_error "无法获取文件创建时间: $basename"
        ((total_errors++))
        return 1
    fi
    
    # 生成新文件名
    local new_filename=$(generate_new_filename "$file" "$creation_time")
    if [ $? -ne 0 ] || [ -z "$new_filename" ]; then
        log_error "无法生成新文件名: $basename"
        ((total_errors++))
        return 1
    fi
    
    # 如果文件名已经是目标格式，跳过
    local expected_prefix=""
    if is_image_file "$file"; then
        expected_prefix="IMG"
    elif is_video_file "$file"; then
        expected_prefix="VID"
    fi
    
    if [[ "$basename" =~ ^${expected_prefix}[0-9]{14}\..* ]]; then
        log_info "文件名已是目标格式: $basename"
        ((total_skipped++))
        return 0
    fi
    
    # 执行重命名
    local new_path="$SCRIPT_DIR/$new_filename"
    if mv "$file" "$new_path"; then
        log_success "重命名: $basename -> $new_filename"
        ((total_renamed++))
        
        # 保持原始的访问和修改时间
        if command -v touch >/dev/null 2>&1; then
            touch -r "$new_path" "$new_path" 2>/dev/null || true
        fi
    else
        log_error "重命名失败: $basename"
        ((total_errors++))
        return 1
    fi
}

# 处理当前目录的所有文件
process_current_directory() {
    log_info "开始处理当前目录中的图片文件..."
    
    # 创建临时数组存储文件列表，避免在遍历过程中修改目录内容
    local files=()
    while IFS= read -r -d '' file; do
        files+=("$file")
    done < <(find "$SCRIPT_DIR" -maxdepth 1 -type f -print0)
    
    # 处理每个文件
    for file in "${files[@]}"; do
        if [ -f "$file" ]; then
            rename_file "$file"
        fi
    done
}

# 显示使用说明
show_usage() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示此帮助信息"
    echo "  -d, --dry-run  模拟运行，不实际重命名文件"
    echo "  -l, --list     列出所有图片文件及其创建时间"
    echo ""
    echo "支持的图片格式:"
    printf "  "
    printf "%s " "${IMAGE_EXTENSIONS[@]}"
    echo ""
    echo "支持的视频格式:"
    printf "  "
    printf "%s " "${VIDEO_EXTENSIONS[@]}"
    echo ""
    echo ""
    echo "重命名规则:"
    echo "  图片格式: IMGYYYYMMDDHHMMSS.扩展名"
    echo "  视频格式: VIDYYYYMMDDHHMMSS.扩展名"
    echo "  例如: IMG20250727124825.jpg"
    echo "       VID20230702081129.mp4"
    echo "  说明: 根据文件的创建时间生成新文件名"
    echo "  冲突处理: 同名时在秒数上递增（自动处理分钟、小时等进位）"
    echo ""
    echo "注意事项:"
    echo "  - 不会修改文件的创建/修改时间"
    echo "  - 仅处理当前目录中的文件"
    echo "  - 跳过脚本文件(.sh)"
    echo "  - 图片文件使用IMG前缀，视频文件使用VID前缀"
    echo "  - 已经是目标格式的文件会被跳过"
}

# 列出所有图片文件及其创建时间
list_image_files() {
    log_info "列出当前目录中的所有媒体文件及创建时间..."
    echo ""
    printf "%-40s %-8s %-20s %-20s\n" "当前文件名" "类型" "创建时间" "新文件名"
    printf "%-40s %-8s %-20s %-20s\n" "----------------------------------------" "--------" "--------------------" "--------------------"
    
    for file in "$SCRIPT_DIR"/*; do
        if [ -f "$file" ]; then
            local basename=$(basename "$file")
            
            # 跳过脚本文件
            if [[ "$basename" == *.sh ]]; then
                continue
            fi
            
            local file_type=""
            if is_image_file "$file"; then
                file_type="图片"
            elif is_video_file "$file"; then
                file_type="视频"
            else
                continue
            fi
            
            local creation_time=$(get_file_creation_time "$file")
            if [ -n "$creation_time" ] && [ ${#creation_time} -eq 14 ]; then
                local new_filename=$(generate_new_filename "$file" "$creation_time")
                local formatted_time="${creation_time:0:4}-${creation_time:4:2}-${creation_time:6:2} ${creation_time:8:2}:${creation_time:10:2}:${creation_time:12:2}"
                printf "%-40s %-8s %-20s %-20s\n" "$basename" "$file_type" "$formatted_time" "$new_filename"
            else
                printf "%-40s %-8s %-20s %-20s\n" "$basename" "$file_type" "获取失败" "N/A"
            fi
        fi
    done
}

# 模拟运行模式
dry_run=false
list_only=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
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
    list_image_files
    exit 0
fi

# 如果是模拟运行
if [ "$dry_run" = true ]; then
    log_warning "模拟运行模式 - 不会实际重命名文件"
    # 重定义重命名函数为只显示信息
    rename_file() {
        local file="$1"
        local basename=$(basename "$file")
        
        # 跳过脚本文件
        if [[ "$basename" == *.sh ]]; then
            return 0
        fi
        
        if ! is_media_file "$file"; then
            log_warning "跳过非媒体文件: $basename"
            ((total_skipped++))
            return 0
        fi
        
        local creation_time=$(get_file_creation_time "$file")
        if [ -z "$creation_time" ] || [ ${#creation_time} -ne 14 ]; then
            log_error "无法获取文件创建时间: $basename"
            ((total_errors++))
            return 1
        fi
        
        local new_filename=$(generate_new_filename "$file" "$creation_time")
        if [ $? -ne 0 ] || [ -z "$new_filename" ]; then
            log_error "无法生成新文件名: $basename"
            ((total_errors++))
            return 1
        fi
        
        if [ "$basename" == "$new_filename" ]; then
            log_info "文件名已是目标格式: $basename"
            ((total_skipped++))
        else
            local file_type_desc=""
            if is_image_file "$file"; then
                file_type_desc="图片"
            elif is_video_file "$file"; then
                file_type_desc="视频"
            else
                file_type_desc="媒体"
            fi
            log_info "将重命名${file_type_desc}: $basename -> $new_filename"
            ((total_renamed++))
        fi
    }
fi

# 主执行逻辑
main() {
    log_info "开始执行照片重命名任务..."
    
    # 检查当前目录是否有图片文件
    local image_count=0
    for file in "$SCRIPT_DIR"/*; do
        if [ -f "$file" ] && [[ $(basename "$file") != *.sh ]] && is_media_file "$file"; then
            ((image_count++))
        fi
    done
    
    if [ $image_count -eq 0 ]; then
        log_warning "当前目录中没有找到媒体文件"
        exit 0
    fi
    
    log_info "发现 $image_count 个媒体文件"
    
    process_current_directory
    
    # 显示统计信息
    echo ""
    log_info "重命名任务完成！"
    log_success "成功重命名: $total_renamed 个文件"
    log_info "跳过文件: $total_skipped 个"
    
    if [ $total_errors -gt 0 ]; then
        log_error "处理失败: $total_errors 个文件"
    fi
    
    if [ "$dry_run" = true ]; then
        log_warning "这是模拟运行，未实际重命名文件"
    fi
}

# 执行主函数
main

#!/bin/bash

# IMG文件时间戳校正脚本 (WSL Ubuntu 专用版本)
# 处理形如 IMG20250409084814.MOV、IMG_20250727_141938.MOV 等文件
# 从文件名中提取时间信息并校正文件的创建时间和修改时间
# 
# 支持的文件名格式:
# - IMG20250409084814.MOV (IMGyyyyMMddHHmmss.ext)
# - IMG_20250727_141938.MOV (IMG_yyyyMMdd_HHmmss.ext)
# - VID20250409084814.MP4 (VIDyyyyMMddHHmmss.ext)
# - VID_20250727_141938.MP4 (VID_yyyyMMdd_HHmmss.ext)
#
# 使用方法:
# 1. 在 WSL Ubuntu 中，cd 到目标目录
# 2. 运行此脚本: bash fix_img_timestamps.sh
# 
# 注意: 需要安装 touch 命令支持 -d 参数

# 检查是否在 WSL 或 Linux 环境中运行
if [[ -f /proc/version ]] && grep -qi microsoft /proc/version; then
    echo "✓ 检测到 WSL 环境"
elif [[ -f /proc/version ]] && grep -qi linux /proc/version; then
    echo "✓ 检测到 Linux 环境"
else
    echo "⚠️  警告: 此脚本专为 WSL Ubuntu 或 Linux 环境设计"
fi

echo ""
echo "📸 IMG/VID 文件时间戳校正工具"
echo "📁 当前工作目录: $(pwd)"
echo "🔍 递归扫描当前目录及其子目录..."

# 检查必要工具
if ! command -v touch &> /dev/null; then
    echo "❌ 错误: 未找到 touch 命令"
    exit 1
fi

# 测试 touch 命令是否支持 -d 参数
if ! touch -d "2025-01-01 12:00:00" /tmp/test_touch_$$ 2>/dev/null; then
    echo "❌ 错误: touch 命令不支持 -d 参数"
    rm -f /tmp/test_touch_$$
    exit 1
fi
rm -f /tmp/test_touch_$$

echo "✓ 工具检查完成"
echo ""

# 函数：从文件名提取时间戳
extract_timestamp() {
    local filename="$1"
    local basename=$(basename "$filename")
    
    # 移除文件扩展名
    local name_without_ext="${basename%.*}"
    
    # 模式1: IMG20250409084814 (IMGyyyyMMddHHmmss)
    if [[ $name_without_ext =~ ^(IMG|VID)([0-9]{4})([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})$ ]]; then
        local year="${BASH_REMATCH[2]}"
        local month="${BASH_REMATCH[3]}"
        local day="${BASH_REMATCH[4]}"
        local hour="${BASH_REMATCH[5]}"
        local minute="${BASH_REMATCH[6]}"
        local second="${BASH_REMATCH[7]}"
        
        echo "${year}-${month}-${day} ${hour}:${minute}:${second}"
        return 0
    fi
    
    # 模式2: IMG_20250727_141938 (IMG_yyyyMMdd_HHmmss)
    if [[ $name_without_ext =~ ^(IMG|VID)_([0-9]{4})([0-9]{2})([0-9]{2})_([0-9]{2})([0-9]{2})([0-9]{2})$ ]]; then
        local year="${BASH_REMATCH[2]}"
        local month="${BASH_REMATCH[3]}"
        local day="${BASH_REMATCH[4]}"
        local hour="${BASH_REMATCH[5]}"
        local minute="${BASH_REMATCH[6]}"
        local second="${BASH_REMATCH[7]}"
        
        echo "${year}-${month}-${day} ${hour}:${minute}:${second}"
        return 0
    fi
    
    # 未匹配到任何模式
    return 1
}

# 函数：验证日期时间是否有效
validate_datetime() {
    local datetime="$1"
    
    # 使用 date 命令验证日期时间格式
    if date -d "$datetime" &>/dev/null; then
        return 0
    else
        return 1
    fi
}

# 统计符合条件的文件
echo "📊 正在统计符合条件的文件..."
total_files=0
processed_files=0
skipped_files=0
error_files=0

# 查找所有可能的文件（常见的图像和视频扩展名）
file_extensions="jpg jpeg png gif bmp tiff mov mp4 avi mkv wmv flv webm m4v 3gp"
find_pattern=""

for ext in $file_extensions; do
    if [[ -z "$find_pattern" ]]; then
        find_pattern="-iname \"*.${ext}\""
    else
        find_pattern="$find_pattern -o -iname \"*.${ext}\""
    fi
done

# 创建临时文件列表
temp_file_list="/tmp/img_files_$$"
eval "find . -type f \( $find_pattern \) -print0" > "$temp_file_list" 2>/dev/null

# 统计总文件数
while IFS= read -r -d '' file; do
    basename_file=$(basename "$file")
    if [[ $basename_file =~ ^(IMG|VID) ]]; then
        ((total_files++))
    fi
done < "$temp_file_list"

if [[ $total_files -eq 0 ]]; then
    echo "✅ 未找到符合条件的 IMG/VID 文件"
    rm -f "$temp_file_list"
    echo "🏁 脚本执行完毕"
    exit 0
fi

echo "📋 找到 $total_files 个符合条件的文件"
echo ""

# 显示前几个文件作为预览
echo "📋 文件预览（前10个）:"
count=0
while IFS= read -r -d '' file; do
    basename_file=$(basename "$file")
    if [[ $basename_file =~ ^(IMG|VID) ]]; then
        if [[ $count -lt 10 ]]; then
            timestamp=$(extract_timestamp "$file")
            if [[ $? -eq 0 ]]; then
                echo "   ✓ $basename_file → $timestamp"
            else
                echo "   ⚠ $basename_file (无法解析时间戳)"
            fi
        elif [[ $count -eq 10 ]]; then
            echo "   ... 还有 $((total_files - 10)) 个文件"
            break
        fi
        ((count++))
    fi
done < "$temp_file_list"

echo ""
echo "🚀 开始处理文件..."

# 进度条函数
show_progress() {
    local current=$1
    local total=$2
    local width=40
    local percentage=$((current * 100 / total))
    local completed=$((current * width / total))
    local remaining=$((width - completed))
    
    printf "\r🔄 ["
    printf "%*s" $completed | tr ' ' '█'
    printf "%*s" $remaining | tr ' ' '░'
    printf "] %d%% (%d/%d)" $percentage $current $total
}

# 处理所有文件
current=0
current_dir=""

while IFS= read -r -d '' file; do
    basename_file=$(basename "$file")
    
    # 只处理 IMG 或 VID 开头的文件
    if [[ ! $basename_file =~ ^(IMG|VID) ]]; then
        continue
    fi
    
    ((current++))
    show_progress $current $total_files
    
    # 获取文件所在的目录
    file_dir=$(dirname "$file")
    
    # 显示当前处理的目录（如果变更）
    if [[ "$file_dir" != "$current_dir" ]]; then
        current_dir="$file_dir"
        echo -e "\n📁 处理目录: $current_dir"
    fi
    
    # 检查文件是否存在
    if [[ ! -f "$file" ]]; then
        echo -e "\n⚠️  文件不存在: $basename_file"
        ((skipped_files++))
        continue
    fi
    
    # 提取时间戳
    timestamp=$(extract_timestamp "$file")
    if [[ $? -ne 0 ]]; then
        echo -e "\n⚠️  无法解析时间戳: $basename_file"
        ((skipped_files++))
        continue
    fi
    
    # 验证时间戳
    if ! validate_datetime "$timestamp"; then
        echo -e "\n❌ 无效的时间戳: $basename_file ($timestamp)"
        ((error_files++))
        continue
    fi
    
    # 获取当前文件时间戳
    current_mtime=$(stat -c %Y "$file" 2>/dev/null)
    target_timestamp=$(date -d "$timestamp" +%s 2>/dev/null)
    
    # 检查是否需要更新（允许1秒误差）
    if [[ -n "$current_mtime" ]] && [[ -n "$target_timestamp" ]]; then
        time_diff=$((target_timestamp - current_mtime))
        if [[ $time_diff -ge -1 ]] && [[ $time_diff -le 1 ]]; then
            # 时间戳已经正确，跳过
            ((skipped_files++))
            continue
        fi
    fi
    
    # 应用时间戳
    if touch -d "$timestamp" "$file" 2>/dev/null; then
        # echo -e "\n✅ 已更新: $basename_file → $timestamp"
        ((processed_files++))
    else
        echo -e "\n❌ 更新失败: $basename_file"
        ((error_files++))
    fi
    
done < "$temp_file_list"

# 清理临时文件
rm -f "$temp_file_list"

# 清除进度条并显示最终结果
echo ""
echo ""
echo "🎉 处理完成!"
echo "📊 统计结果:"
echo "   总计: $total_files 个文件"
echo "   ✅ 成功处理: $processed_files 个文件"
echo "   ⚠️  跳过: $skipped_files 个文件 (时间戳已正确或无法解析)"
echo "   ❌ 处理失败: $error_files 个文件"

if [[ $processed_files -gt 0 ]]; then
    echo ""
    echo "✨ 成功校正了 $processed_files 个文件的时间戳"
fi

if [[ $error_files -gt 0 ]]; then
    echo ""
    echo "⚠️  有 $error_files 个文件处理失败，可能的原因:"
    echo "   • 文件权限问题"
    echo "   • 无效的时间戳格式"
    echo "   • 文件系统不支持时间戳修改"
fi

if [[ $skipped_files -gt 0 ]]; then
    echo ""
    echo "💡 跳过了 $skipped_files 个文件，可能的原因:"
    echo "   • 文件名格式不匹配"
    echo "   • 时间戳已经正确"
    echo "   • 无法解析文件名中的时间信息"
fi

echo ""
echo "🏁 脚本执行完毕"

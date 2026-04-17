#!/bin/bash

# 删除 JSON 文件脚本 (WSL Ubuntu 专用版本)
# 递归遍历当前目录及其所有子目录，删除所有 .json 文件
# 
# 使用方法:
# 1. 在 WSL Ubuntu 中，cd 到目标目录
# 2. 运行此脚本: bash delete_json_files.sh
# 
# 注意: 此操作不可逆，请谨慎使用！建议先备份重要数据

# 检查是否在 WSL 或 Linux 环境中运行
if [[ -f /proc/version ]] && grep -qi microsoft /proc/version; then
    echo "✓ 检测到 WSL 环境"
elif [[ -f /proc/version ]] && grep -qi linux /proc/version; then
    echo "✓ 检测到 Linux 环境"
else
    echo "⚠️  警告: 此脚本专为 WSL Ubuntu 或 Linux 环境设计"
fi

echo ""
echo "🗑️  JSON 文件删除工具"
echo "📁 当前工作目录: $(pwd)"
echo "🔍 递归扫描当前目录及其子目录..."

# 统计 JSON 文件数量
echo "📊 正在统计 JSON 文件数量..."
total_files=0
while IFS= read -r -d '' json_file; do
    ((total_files++))
done < <(find . -name "*.json" -type f -print0 2>/dev/null)

if [[ $total_files -eq 0 ]]; then
    echo "✅ 未找到任何 JSON 文件"
    echo "🏁 脚本执行完毕"
    exit 0
fi

echo "⚠️  找到 $total_files 个 JSON 文件"
echo ""

# 显示将要删除的文件列表（仅显示前20个作为预览）
echo "📋 将要删除的文件列表（预览前20个）:"
count=0
while IFS= read -r -d '' json_file; do
    ((count++))
done < <(find . -name "*.json" -type f -print0 2>/dev/null)

echo ""
echo "🚀 开始删除 JSON 文件..."

# 初始化计数器
current=0
deleted=0
failed=0
current_dir=""

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

# 删除所有 JSON 文件（递归搜索）
while IFS= read -r -d '' json_file; do
    [[ ! -f "$json_file" ]] && continue
    
    # 更新进度
    ((current++))
    show_progress $current $total_files
    
    # 获取文件所在的目录
    file_dir=$(dirname "$json_file")
    file_name=$(basename "$json_file")
    
    # 显示当前处理的目录（如果变更）
    if [[ "$file_dir" != "$current_dir" ]]; then
        current_dir="$file_dir"
        echo -e "\n📁 处理目录: $current_dir"
    fi
    
    # 尝试删除文件
    if rm "$json_file" 2>/dev/null; then
        # echo -e "\n✅ 已删除: $file_name"
        ((deleted++))
    else
        # 获取详细的错误信息
        error_msg=$(rm "$json_file" 2>&1)
        echo -e "\n❌ 删除失败: $file_name"
        echo "   错误信息: $error_msg"
        
        # 检查文件权限
        if [[ -f "$json_file" ]]; then
            ls_info=$(ls -la "$json_file" 2>/dev/null)
            echo "   文件信息: $ls_info"
        fi
        
        ((failed++))
    fi
    
done < <(find . -name "*.json" -type f -print0 2>/dev/null)

# 清除进度条并显示最终结果
echo ""
echo ""
echo "🎉 删除操作完成!"
echo "📊 统计结果:"
echo "   总计: $total_files 个文件"
echo "   ✅ 成功删除: $deleted 个文件"
echo "   ❌ 删除失败: $failed 个文件"

if [[ $deleted -gt 0 ]]; then
    echo ""
    echo "✨ 成功删除了 $deleted 个 JSON 文件"
    echo "💡 提示: 可以使用 find . -name '*.json' 命令验证是否还有剩余的 JSON 文件"
fi

if [[ $failed -gt 0 ]]; then
    echo ""
    echo "⚠️  有 $failed 个文件删除失败，可能的原因:"
    echo "   • 文件权限问题"
    echo "   • 文件被其他程序占用"
    echo "   • 文件系统只读"
    echo "   • 文件路径包含特殊字符"
fi

echo ""
echo "🏁 脚本执行完毕"

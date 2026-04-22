#!/bin/bash

# Google Takeout 照片时间戳修复脚本 (WSL Ubuntu 专用版本)
# 根据 JSON 文件中的时间戳修改对应照片文件的创建时间
# 此版本会递归处理当前目录及其所有子目录中的文件
# 
# 使用方法:
# 1. 在Windows中，将Google Takeout解压的照片文件夹复制到WSL可访问的位置
# 2. 在WSL Ubuntu中，cd到照片文件夹的根目录
# 3. 运行此脚本: bash fix_photo_timestamps_wsl.sh
# 
# 注意: 此脚本会修改文件的修改时间和访问时间，但不会修改文件内容
#
# 参数:
# --dry-run: 预览模式，仅显示将要修改的文件，不实际修改时间戳

# 解析命令行参数
DRY_RUN=0
for arg in "$@"; do
    if [[ "$arg" == "--dry-run" ]]; then
        DRY_RUN=1
    fi
done

# 检查是否在 WSL 或 Linux 环境中运行
if [[ -f /proc/version ]] && grep -qi microsoft /proc/version; then
    echo "✓ 检测到 WSL 环境"
elif [[ -f /proc/version ]] && grep -qi linux /proc/version; then
    echo "✓ 检测到 Linux 环境"
else
    echo "⚠️  警告: 此脚本专为 WSL Ubuntu 或 Linux 环境设计"
    echo "当前环境可能不支持某些功能"
fi

# 检查 jq 是否安装
if ! command -v jq &> /dev/null; then
    echo "❌ 错误: 请先安装 jq"
    echo "在 Ubuntu/WSL 中运行: sudo apt update && sudo apt install jq"
    exit 1
fi

# 检查 find 命令是否可用
if ! command -v find &> /dev/null; then
    echo "❌ 错误: find 命令不可用"
    exit 1
fi

echo ""
if [[ "$DRY_RUN" == "1" ]]; then
    echo "🧪 Dry-run 模式: 仅显示将要修改的文件，不实际修改时间戳"
else
    echo "🚀 开始处理照片时间戳..."
fi
echo "📁 当前工作目录: $(pwd)"
echo "🔍 仅处理 'Photos from*' 开头的文件夹..."

# 统计所有 'Photos from*' 开头文件夹下的 *.json 文件数量
echo "📊 正在统计 JSON 文件数量..."
total_files=0
json_dirs=()
while IFS= read -r -d '' dir; do
    json_dirs+=("$dir")
done < <(find . -type d -name "Photos from*" -print0 2>/dev/null)

for dir in "${json_dirs[@]}"; do
    while IFS= read -r -d '' json_file; do
        ((total_files++))
    done < <(find "$dir" -name "*.json" -type f -print0 2>/dev/null)
done

if [[ $total_files -eq 0 ]]; then
    echo "❌ 未找到 JSON 文件"
    echo "请确保:"
    echo "  1. 当前目录包含 Google Takeout 导出的照片文件"
    echo "  2. 仅处理 'Photos from*' 开头的文件夹"
    exit 1
fi

echo "✅ 找到 $total_files 个需要处理的文件"
echo ""

# 初始化计数器
current=0
processed=0
skipped=0
current_dir=""
total_dirs=${#json_dirs[@]}

echo "📂 将处理 $total_dirs 个目录"
echo ""


# 进度条函数
show_progress() {
    local current=$1
    local total=$2
    local width=40
    local percentage=$((current * 100 / total))
    local completed=$((current * width / total))
    local remaining=$((width - completed))
    
    printf "\r🔄 ["
    printf "%*s" $completed | tr ' ' '+'
    printf "%*s" $remaining | tr ' ' '-'
    printf "] %d%% (%d/%d)" $percentage $current $total
}

# 仅处理所有 'Photos from*' 开头目录下的 JSON 文件
for dir in "${json_dirs[@]}"; do
    while IFS= read -r -d '' json_file; do
        [[ ! -f "$json_file" ]] && continue
    
        # 更新进度
        ((current++))
        show_progress $current $total_files
    
        # 获取JSON文件所在的目录
        json_dir=$(dirname "$json_file")
        json_basename=$(basename "$json_file")
        
        # 显示当前处理的目录（如果变更）
        if [[ "$json_dir" != "$current_dir" ]]; then
            current_dir="$json_dir"
            echo -e "\n📁 处理目录: $current_dir"
        fi
        
        # 解析 JSON 文件名结构
        base_name=""
        number=""
        
        # 优化 JSON 文件名解析，支持多种常见格式
        # 1. 带编号且有补充后缀：basename.ext.suffix(数字).json
        #    例如：IMG_20240913_162956.jpg.supplemental-metadata(1).json
        if [[ "$json_basename" =~ ^(.+)\.([^.]+)\.([^.]+)\(([0-9]+)\)\.json$ ]]; then
            number="${BASH_REMATCH[4]}"
            base_name="${BASH_REMATCH[1]}(${number}).${BASH_REMATCH[2]}"
        # 3. 带编号：basename(数字).json
        #    例如：IMG_20240913_162956(1).json 或 Screenshot_2023-05-01-22-08-49-61_e39d2c7de191(1).json
        elif [[ "$json_basename" =~ ^(.+)\(([0-9]+)\)\.json$ ]]; then
            base_name="${BASH_REMATCH[1]}(${BASH_REMATCH[2]})"
            number="${BASH_REMATCH[2]}"
        # 4. basename.ext.suffix.json（如 supplemental、supp、s 等）
        #    例如：IMG_20240913_162956.jpg.supplemental-metadata.json
        elif [[ "$json_basename" =~ ^(.+)\.([^.]+)\.([^.]+)\.json$ ]]; then
            base_name="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
        # 5. basename..json → basename
        #    例如：IMG_20240913_162956..json
        elif [[ "$json_basename" =~ ^(.+)\.\.json$ ]]; then
            base_name="${BASH_REMATCH[1]}"
        # 6. basename.json → basename
        #    例如：IMG_20240913_162956.json
        elif [[ "$json_basename" =~ ^(.+)\.json$ ]]; then
            base_name="${BASH_REMATCH[1]}"
        else
            base_name="${json_basename%.json}"
        fi
        
        # 查找对应的照片文件
        photo_file=""
        
        # 生成可能的文件名候选列表
        candidates=()
        
        # 判断 base_name 是否带后缀名（如 .jpg/.png/.heic 等）
        if [[ "$base_name" =~ ^(.+)\.([^.]+)$ ]]; then
            filename="${BASH_REMATCH[1]}"
            extension="${BASH_REMATCH[2]}"
            # 优先处理带后缀名的精确匹配
            if [[ -n "$number" ]]; then
                # 带编号且带后缀名
                candidate="$json_dir/${filename}(${number}).${extension}"
                if [[ -f "$candidate" ]]; then
                    candidates+=("$candidate")
                else
                    # 带修饰符
                    for suffix in "-已修改" "-编辑" "-修改" "-edited" "-modified"; do
                        candidate="$json_dir/${filename}${suffix}(${number}).${extension}"
                        if [[ -f "$candidate" ]]; then
                            candidates+=("$candidate")
                        fi
                    done
                fi
            else
                # 普通带后缀名文件
                candidate="$json_dir/${filename}.${extension}"
                if [[ -f "$candidate" ]]; then
                    candidates+=("$candidate")
                else
                    # 带修饰符
                    for suffix in "-已修改" "-编辑" "-修改" "-edited" "-modified"; do
                        candidate="$json_dir/${filename}${suffix}.${extension}"
                        if [[ -f "$candidate" ]]; then
                            candidates+=("$candidate")
                        fi
                    done
                fi
                # 使用 find 查找所有以 filename 开头，以 .extension 结尾的文件
                # while IFS= read -r -d '' candidate; do
                #     candidates+=("$candidate")
                # done < <(find "$json_dir" -maxdepth 1 -name "${filename}*.${extension}" -type f -print0 2>/dev/null)
            fi
        else
            # 不带后缀名的情况
            if [[ -n "$number" ]]; then
                # 带编号无后缀名
                candidate="$json_dir/${base_name}(${number})"
                if [[ -f "$candidate" ]]; then
                    candidates+=("$candidate")
                else
                    for suffix in "-已修改" "-编辑" "-修改" "-edited" "-modified"; do
                        candidate="$json_dir/${base_name}${suffix}(${number})"
                        if [[ -f "$candidate" ]]; then
                            candidates+=("$candidate")
                        fi
                    done
                fi
            else
                # 普通无后缀名文件
                candidate="$json_dir/$base_name"
                if [[ -f "$candidate" ]]; then
                    candidates+=("$candidate")
                else 
                    for suffix in "-已修改" "-编辑" "-修改" "-edited" "-modified"; do
                        candidate="$json_dir/${base_name}${suffix}"
                        if [[ -f "$candidate" ]]; then
                            candidates+=("$candidate")
                        fi
                    done
                fi
            fi
        fi
        
        # 收集所有存在的候选文件
        photo_files=()
        for candidate in "${candidates[@]}"; do
            if [[ -f "$candidate" ]]; then
                photo_files+=("$candidate")
            fi
        done
        
        # 如果没有找到任何候选文件，尝试模糊匹配（仅唯一结果才采用）
        if [[ ${#photo_files[@]} -eq 0 ]]; then
            tmp_candidates=()
            while IFS= read -r -d '' candidate; do
                [[ "$candidate" == *.json ]] && continue
                tmp_candidates+=("$candidate")
            done < <(find "$json_dir" -maxdepth 1 -type f -name "${base_name}*" ! -name "*.json" -print0 2>/dev/null)
            if [[ ${#tmp_candidates[@]} -eq 1 ]]; then
                photo_files+=("${tmp_candidates[0]}")
            elif [[ ${#tmp_candidates[@]} -eq 0 && -n "$number" ]]; then
                # 存在 base_name 是 Screenshot_2023-06-22-01-01-22-72_a2db1b9502c9(1) 
                # candidate 是 Screenshot_2023-06-22-01-01-22-72_a2db1b9502c98(1) 
                # 这种情况就需要去这样匹配 Screenshot_2023-06-22-01-01-22-72_a2db1b9502c9*(1)
                # 拆分数字后缀
                if [[ "$base_name" =~ ^(.+)\(([0-9]+)\)$ ]]; then
                    prefix="${BASH_REMATCH[1]}"
                    suffix="${BASH_REMATCH[2]}"
                    # 用通配符匹配，需要转义括号以正确处理
                    while IFS= read -r -d '' candidate; do
                        [[ "$candidate" == *.json ]] && continue
                        tmp_candidates+=("$candidate")
                    done < <(find "$json_dir" -maxdepth 1 -type f -name "${prefix}*\(${suffix}\)*" ! -name "*.json" -type f -print0 2>/dev/null)
                    if [[ ${#tmp_candidates[@]} -eq 1 ]]; then
                        photo_files+=("${tmp_candidates[0]}")
                    fi
                fi
            elif [[ ${#tmp_candidates[@]} -eq 0 ]]; then
                echo -e "\n⚠️  跳过: $json_file (找不到对应的照片文件 $base_name) "                
            else
                # 多个候选文件，尝试精确匹配
                for candidate in "${tmp_candidates[@]}"; do
                    candidate_name=$(basename "$candidate")
                    
                    if [[ -n "$number" ]]; then
                        # 如果 JSON 文件有编号，优先匹配带相同编号的照片
                        if [[ "$base_name" == "$candidate_name" ]]; then
                            photo_files+=("$candidate")
                            break
                        elif [[ "$candidate_name" == "$base_name"* && "$candidate_name" =~ \(${number}\) ]]; then
                            photo_files+=("$candidate")
                            break
                        fi
                    else
                        # 无编号情况的匹配逻辑
                        if [[ "$base_name" == "$candidate_name" ]]; then
                            photo_files+=("$candidate")
                            break
                        elif [[ ! ("$candidate_name" =~ \([0-9]\)) && "$candidate_name" == "$base_name"* ]]; then
                            photo_files+=("$candidate")
                        fi
                    fi
                done
            fi
        fi
        
        # 检查是否找到照片文件
        if [[ ${#photo_files[@]} -eq 0 ]]; then
            echo -e "\n⚠️  跳过: $json_file (找不到对应 $base_name)"
            echo "   尝试过的候选文件:"
            for candidate in "${candidates[@]}"; do
            echo "   - $candidate"
            done
            ((skipped++))
            continue
        elif [[ ${#photo_files[@]} -gt 1 ]]; then
            echo -e "\n⚠️  找到多个匹配文件"
            echo "   所有匹配文件:"
            for photo_file in "${photo_files[@]}"; do
            echo "   - $photo_file"
            done
            continue
        fi
        # 使用第一个找到的照片文件
        
        
        # 提取时间戳
        timestamp=$(jq -r '.photoTakenTime.timestamp // empty' "$json_file" 2>/dev/null)
        
        # 优先从文件名中提取时间戳
        photo_name=$(basename "${photo_files[0]}")
        timestamp=""
        # 支持多种常见格式
        # 1. IMG_20230302_112040
        if [[ "$photo_name" =~ ([0-9]{8})_([0-9]{6}) ]]; then
            date="${BASH_REMATCH[1]}"
            time="${BASH_REMATCH[2]}"
            timestamp=$(date -d "${date:0:4}-${date:4:2}-${date:6:2} ${time:0:2}:${time:2:2}:${time:4:2}" +%s 2>/dev/null)
        # 2. IMG20230123102606
        elif [[ "$photo_name" =~ ([0-9]{8})([0-9]{6}) ]]; then
            date="${BASH_REMATCH[1]}"
            time="${BASH_REMATCH[2]}"
            timestamp=$(date -d "${date:0:4}-${date:4:2}-${date:6:2} ${time:0:2}:${time:2:2}:${time:4:2}" +%s 2>/dev/null)
        # 3. WP_20131010_074
        elif [[ "$photo_name" =~ ([0-9]{8})_([0-9]{3,6}) ]]; then
            date="${BASH_REMATCH[1]}"
            time="${BASH_REMATCH[2]}"
            time=$(printf "%06d" "$time")
            timestamp=$(date -d "${date:0:4}-${date:4:2}-${date:6:2} ${time:0:2}:${time:2:2}:${time:4:2}" +%s 2>/dev/null)
        # 4. 20151120_120004
        elif [[ "$photo_name" =~ ([0-9]{8})_([0-9]{6}) ]]; then
            date="${BASH_REMATCH[1]}"
            time="${BASH_REMATCH[2]}"
            timestamp=$(date -d "${date:0:4}-${date:4:2}-${date:6:2} ${time:0:2}:${time:2:2}:${time:4:2}" +%s 2>/dev/null)
        # 5. 20151120_120004~2
        elif [[ "$photo_name" =~ ([0-9]{8})_([0-9]{6})~[0-9]+ ]]; then
            date="${BASH_REMATCH[1]}"
            time="${BASH_REMATCH[2]}"
            timestamp=$(date -d "${date:0:4}-${date:4:2}-${date:6:2} ${time:0:2}:${time:2:2}:${time:4:2}" +%s 2>/dev/null)
        # 6. Screenshot_2016-02-28-13-06-34
        elif [[ "$photo_name" =~ ([0-9]{4})-([0-9]{2})-([0-9]{2})-([0-9]{2})-([0-9]{2})-([0-9]{2}) ]]; then
            timestamp=$(date -d "${BASH_REMATCH[1]}-${BASH_REMATCH[2]}-${BASH_REMATCH[3]} ${BASH_REMATCH[4]}:${BASH_REMATCH[5]}:${BASH_REMATCH[6]}" +%s 2>/dev/null)
        # 7. Screenshot_20210803-084525
        elif [[ "$photo_name" =~ ([0-9]{8})-([0-9]{6}) ]]; then
            date="${BASH_REMATCH[1]}"
            time="${BASH_REMATCH[2]}"
            timestamp=$(date -d "${date:0:4}-${date:4:2}-${date:6:2} ${time:0:2}:${time:2:2}:${time:4:2}" +%s 2>/dev/null)
        # 8. mmexport1491013330299
        elif [[ "$photo_name" =~ mmexport([0-9]{13}) ]]; then
            timestamp="${BASH_REMATCH[1]:0:10}"
        # 9. mmexport1491013330299-已修改 或 mmexport1491013330299-编辑 或 mmexport1491013330299-任意中文字符
        elif [[ "$photo_name" =~ mmexport([0-9]{13})-([[:alpha:]]+|[[:punct:]]+|[[:digit:]]+|[一-龥]+) ]]; then
            timestamp="${BASH_REMATCH[1]:0:10}"
        # 10. mmexport1491013330299(数字)
        elif [[ "$photo_name" =~ mmexport([0-9]{13})\([0-9]+\) ]]; then
            timestamp="${BASH_REMATCH[1]:0:10}"
        fi

        # 如果文件名没有时间戳，则尝试从 JSON 获取
        if [[ -z "$timestamp" || "$timestamp" == "null" ]]; then
            timestamp=$(jq -r '.photoTakenTime.timestamp // empty' "$json_file" 2>/dev/null)
        fi

        # 检查时间戳是否有效
        if [[ -z "$timestamp" || "$timestamp" == "null" ]]; then
            echo -e "\n⚠️  跳过: $json_file (无时间戳)"
            ((skipped++))
            continue
        fi
            
        # 对所有找到的照片文件进行时间戳修改
        files_success=0
        files_failed=0
        # for photo_file in "${photo_files[@]}"; do
        photo_file=${photo_files[0]}
        # 获取可读的时间格式
        readable_time=$(date -d "@$timestamp" '+%Y-%m-%d %H:%M:%S' 2>/dev/null)
        
        if [[ "$DRY_RUN" == "1" ]]; then
            # Dry-run 模式，只显示将要修改的文件，不实际修改
            # echo -e "\n🧪 将修改: $(basename "$photo_file") -> $readable_time"
            ((files_success++))
            ((processed++))
        else
            # 实际修改时间戳
            if touch -d "@$timestamp" "$photo_file" 2>/dev/null; then
                # 只在详细模式下显示成功信息
                # echo -e "\n✅ $(basename "$photo_file") -> $readable_time"
                ((files_success++))
                ((processed++))
            else
                # 获取详细的错误信息
                error_msg=$(touch -d "@$timestamp" "$photo_file" 2>&1)
                echo -e "\n❌ 失败: $(basename "$photo_file")"
                echo "   错误信息: $error_msg"
                
                # 检查文件权限
                if [[ -f "$photo_file" ]]; then
                    ls_info=$(ls -la "$photo_file" 2>/dev/null)
                    echo "   文件信息: $ls_info"
                fi
                
                # 检查文件类型
                file_type=$(file "$photo_file" 2>/dev/null || echo "无法检测文件类型")
                echo "   文件类型: $file_type"
                
                # 检查文件系统是否支持时间戳修改
                fs_type=$(df -T "$photo_file" 2>/dev/null | tail -1 | awk '{print $2}')
                if [[ -n "$fs_type" ]]; then
                    echo "   文件系统: $fs_type"
                    if [[ "$fs_type" == "ntfs" ]] || [[ "$fs_type" == "vfat" ]]; then
                        echo "   注意: $fs_type 文件系统可能不完全支持Linux时间戳操作"
                    fi
                fi
                
                ((files_failed++))
                ((skipped++))
            fi
        fi
        # done
    
    done < <(find "$dir" -name "*.json" -type f -print0 2>/dev/null)
done

# 清除进度条并显示最终结果
echo ""
echo ""
echo "🎉 处理完成!"
echo "📊 统计结果:"
echo "   总计: $total_files 个文件"
echo "   ✅ 成功: $processed 个文件"
echo "   ⚠️  跳过: $skipped 个文件"

if [[ $processed -gt 0 ]]; then
    echo ""
    if [[ "$DRY_RUN" == "1" ]]; then
        echo "✨ 将修改 $processed 个文件的时间戳"
        echo "💡 提示: 重新运行脚本并去掉 --dry-run 参数以实际修改文件"
    else
        echo "✨ 成功修改了 $processed 个文件的时间戳"
        echo "💡 提示: 可以使用 ls -la 命令查看文件的新时间戳"
    fi
fi

if [[ $skipped -gt 0 ]]; then
    echo ""
    echo "⚠️  有 $skipped 个文件被跳过，可能的原因:"
    echo "   • 找不到对应的照片文件"
    echo "   • JSON 文件中没有时间戳信息"
    echo "   • 文件权限问题"
    echo "   • 文件系统不支持时间戳修改"
fi

echo ""
echo "🏁 脚本执行完毕"

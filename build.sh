#!/usr/bin/env bash

# 遇到错误立即退出
set -e

package_update_zip() {
    local os=$1
    local arch=$2
    local output_dir=$3

    local version
    version="$(get_app_version)" || version="unknown"

    local exe_name="${APP_NAME}"
    if [ "$os" = "windows" ]; then
        exe_name="${APP_NAME}.exe"
    fi

    local src_exe="dist/${output_dir}/${exe_name}"
    if [ ! -f "$src_exe" ]; then
        echo -e "${RED}❌ 未找到编译产物: ${src_exe}${NC}"
        return 1
    fi

    if ! command -v zip >/dev/null 2>&1; then
        echo -e "${RED}❌ 未找到 zip 命令，无法生成更新包（请安装 zip 工具）${NC}"
        return 1
    fi

    mkdir -p "dist/packages"
    local pkg_name="${APP_NAME}-${os}-${arch}-v${version}.zip"
    local pkg_path="dist/packages/${pkg_name}"

    local tmp_dir
    tmp_dir="$(mktemp -d)"
    cp "$src_exe" "${tmp_dir}/${exe_name}"

    (
        cd "$tmp_dir"
        zip -q -9 -r "$SCRIPT_DIR/${pkg_path}" "${exe_name}"
    )
    rm -rf "$tmp_dir"

    echo -e "${GREEN}✅ 更新包已生成: ${pkg_path}${NC}"

    local sha256
    sha256="$(get_sha256_hex "$pkg_path")" || sha256=""
    if [ -n "$sha256" ]; then
        echo "${sha256}  ${pkg_name}" > "${pkg_path}.sha256"
        echo -e "${GREEN}✅ SHA256 已生成: ${pkg_path}.sha256${NC}\n"
    else
        echo -e "${YELLOW}⚠️  未找到 shasum/sha256sum，跳过生成 SHA256 文件${NC}\n"
    fi
}

build_all_update_packages() {
    echo -e "${BLUE}=====================================${NC}"
    echo -e "${YELLOW}📦 开始构建所有架构更新包...${NC}"
    echo -e "${BLUE}=====================================${NC}"

    build_backend "windows" "amd64" "windows_amd64" "Windows 64位"
    package_update_zip "windows" "amd64" "windows_amd64"

    build_backend "linux" "arm64" "linux_arm64" "Linux ARM64"
    package_update_zip "linux" "arm64" "linux_arm64"

    build_backend "linux" "amd64" "linux_amd64" "Linux 64位"
    package_update_zip "linux" "amd64" "linux_amd64"

    build_backend "darwin" "arm64" "darwin_arm64" "macOS (Apple Silicon)"
    package_update_zip "darwin" "arm64" "darwin_arm64"

    build_backend "darwin" "amd64" "darwin_amd64" "macOS (Intel)"
    package_update_zip "darwin" "amd64" "darwin_amd64"

    echo -e "${GREEN}🎉 所有架构更新包已生成，产物已保存至 ./dist/packages 目录下！${NC}\n"
}

# ==========================================
# 切换到脚本所在目录
# ==========================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "$SCRIPT_DIR"

# ==========================================
# 项目配置
# ==========================================
APP_NAME="NetworkAuth"
STATUS_FILE="constants/status.go"

# ==========================================
# 颜色定义
# ==========================================
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# ==========================================
# 功能函数
# ==========================================

get_app_version() {
    local status_file="${STATUS_FILE}"
    if [ ! -f "$status_file" ]; then
        echo ""
        return 1
    fi
    local line
    line="$(awk -F'"' '/AppVersion[[:space:]]*=[[:space:]]*"/ {print $2; exit 0}' "$status_file")"
    if [ -z "$line" ]; then
        echo ""
        return 1
    fi
    echo "$line"
    return 0
}

bump_patch_version() {
    local current
    current="$(get_app_version)" || return 1

    local v="${current#v}"
    v="${v#V}"

    local major minor patch
    IFS='.' read -r major minor patch <<EOF
$v
EOF
    if [ -z "$major" ] || [ -z "$minor" ] || [ -z "$patch" ]; then
        echo -e "${RED}❌ 无法解析当前版本号: ${current}${NC}"
        return 1
    fi
    if ! [[ "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ && "$patch" =~ ^[0-9]+$ ]]; then
        echo -e "${RED}❌ 版本号必须为 x.y.z 的数字格式: ${current}${NC}"
        return 1
    fi

    local next_patch=$((patch + 1))
    local next_version="${major}.${minor}.${next_patch}"

    local status_file="${STATUS_FILE}"
    local tmp_file
    tmp_file="$(mktemp)"
    awk -v ver="$next_version" '
        BEGIN { updated=0 }
        {
            if ($0 ~ /AppVersion[[:space:]]*=[[:space:]]*"/ && updated==0) {
                sub(/AppVersion[[:space:]]*=[[:space:]]*"[^"]*"/, sprintf("AppVersion = %c%s%c", 34, ver, 34))
                updated=1
            }
            print
        }
        END { if (updated==0) exit 2 }
    ' "$status_file" > "$tmp_file"
    local rc=$?
    if [ $rc -ne 0 ]; then
        rm -f "$tmp_file"
        echo -e "${RED}❌ 自动迭代版本失败：未找到 AppVersion 定义${NC}"
        return 1
    fi
    mv "$tmp_file" "$status_file"

    echo -e "${GREEN}✅ 版本号已迭代: ${current} -> ${next_version}${NC}"
    return 0
}

get_sha256_hex() {
    local file_path=$1
    if [ -z "$file_path" ] || [ ! -f "$file_path" ]; then
        echo ""
        return 1
    fi
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$file_path" | awk '{print $1}'
        return 0
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file_path" | awk '{print $1}'
        return 0
    fi
    echo ""
    return 1
}

# 编译并拷贝前端
build_frontend() {
    echo -e "${BLUE}=====================================${NC}"
    echo -e "${YELLOW}🚀 开始编译前端项目...${NC}"
    echo -e "${BLUE}=====================================${NC}"
    cd frontend
    # 如果需要安装依赖可以取消注释下面这行
    # pnpm install
    pnpm run build
    cd ..
    echo -e "${GREEN}✅ 前端编译完成！${NC}\n"

    echo -e "${BLUE}=====================================${NC}"
    echo -e "${YELLOW}📂 正在将前端代码拷贝到后端 public 目录...${NC}"
    echo -e "${BLUE}=====================================${NC}"
    # 清理旧的静态文件
    rm -rf public/dist
    # 确保 public 目录存在
    mkdir -p public
    # 将前端新编译的 dist 目录整个复制到 public 下 (使用 -R 增强跨平台兼容性)
    cp -R frontend/dist public/
    echo -e "${GREEN}✅ 前端代码拷贝完成！${NC}\n"
}

# 编译后端指定架构
build_backend() {
    local os=$1
    local arch=$2
    local output_dir=$3
    local desc=$4

    # 确定可执行文件名称
    local exe_name="${APP_NAME}"
    if [ "${os}" = "windows" ]; then
        exe_name="${APP_NAME}.exe"
    fi

    # 创建对应架构的输出目录
    mkdir -p "dist/${output_dir}"
    
    echo -e "${YELLOW}👉 正在编译 ${desc}...${NC}"
    # 使用用户要求的精确命令格式（通过 -o 指定到子目录，但文件名保持原样）
    CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build -trimpath --ldflags="-s -w" -o "dist/${output_dir}/${exe_name}"
    echo -e "${GREEN}✅ 编译成功: dist/${output_dir}/${exe_name}${NC}\n"
}

# 编译所有后端架构
build_all_backend() {
    echo -e "${BLUE}=====================================${NC}"
    echo -e "${YELLOW}⚙️  开始编译所有架构 Go 后端项目...${NC}"
    echo -e "${BLUE}=====================================${NC}"
    build_backend "windows" "amd64" "windows_amd64" "Windows 64位"
    build_backend "linux" "arm64" "linux_arm64" "Linux ARM64"
    build_backend "linux" "amd64" "linux_amd64" "Linux 64位"
    build_backend "darwin" "arm64" "darwin_arm64" "macOS (Apple Silicon)"
    build_backend "darwin" "amd64" "darwin_amd64" "macOS (Intel)"
    echo -e "${GREEN}🎉 所有架构编译完成，产物已保存至 ./dist 目录下！${NC}\n"
}

# ==========================================
# 辅助函数
# ==========================================
pause_and_return() {
    echo -e "\n${YELLOW}按回车键返回主菜单...${NC}"
    # 使用标准的 read -r 提升在 zsh 和各种不同 shell 中的跨平台兼容性
    read -r
}

# ==========================================
# 菜单界面
# ==========================================
show_menu() {
    clear
    echo -e "${BLUE}=====================================${NC}"
    echo -e "${GREEN}           项目构建脚本菜单           ${NC}"
    echo -e "${BLUE}=====================================${NC}"
    echo -e "1. 🔁 自动构建 + 生成更新包"
    echo -e "2. 🚀 全部构建 (前端 + 后端)"
    echo -e "3. 📦 编译所有后端架构"
    echo -e "4. 🌐 仅编译前端并拷贝"
    echo -e "-------------------------------------"
    echo -e "0. ❌ 退出"
    echo -e "${BLUE}=====================================${NC}"
    echo -n -e "${YELLOW}请输入选项数字并按回车: ${NC}"
}

# ==========================================
# 主循环
# ==========================================
while true; do
    show_menu
    read -r choice
    echo ""
    case $choice in
        1)
            bump_patch_version
            build_frontend
            build_all_update_packages
            pause_and_return
            ;;
        2)
            build_frontend
            build_all_backend
            pause_and_return
            ;;
        3)
            build_all_backend
            pause_and_return
            ;;
        4)
            build_frontend
            pause_and_return
            ;;
        0)
            echo -e "${GREEN}👋 退出脚本。${NC}"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 无效选项，请重新输入！${NC}"
            sleep 1.5
            ;;
    esac
done

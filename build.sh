#!/usr/bin/env bash

# 遇到错误立即退出
set -e

# ==========================================
# 切换到脚本所在目录
# ==========================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "$SCRIPT_DIR"

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
    local exe_name="NetworkAuth"
    if [ "$os" = "windows" ]; then
        exe_name="NetworkAuth.exe"
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
    echo -e "${GREEN}    ApiServe 项目构建脚本菜单        ${NC}"
    echo -e "${BLUE}=====================================${NC}"
    echo -e "1. 🚀 一键全部构建 (前端 + 所有架构后端)"
    echo -e "2. 📦 仅编译所有后端架构"
    echo -e "3. 🌐 仅编译前端并拷贝"
    echo -e "-------------------------------------"
    echo -e "4. 🪟  编译后端: Windows 64位"
    echo -e "5. 🐧 编译后端: Linux ARM64"
    echo -e "6. 🐧 编译后端: Linux 64位"
    echo -e "7. 🍎 编译后端: macOS (Apple Silicon)"
    echo -e "8. 🍎 编译后端: macOS (Intel)"
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
    read choice
    echo ""
    case $choice in
        1)
            build_frontend
            build_all_backend
            pause_and_return
            ;;
        2)
            build_all_backend
            pause_and_return
            ;;
        3)
            build_frontend
            pause_and_return
            ;;
        4)
            build_backend "windows" "amd64" "windows_amd64" "Windows 64位"
            pause_and_return
            ;;
        5)
            build_backend "linux" "arm64" "linux_arm64" "Linux ARM64"
            pause_and_return
            ;;
        6)
            build_backend "linux" "amd64" "linux_amd64" "Linux 64位"
            pause_and_return
            ;;
        7)
            build_backend "darwin" "arm64" "darwin_arm64" "macOS (Apple Silicon)"
            pause_and_return
            ;;
        8)
            build_backend "darwin" "amd64" "darwin_amd64" "macOS (Intel)"
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

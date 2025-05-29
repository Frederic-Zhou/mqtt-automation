#!/data/data/com.termux/files/usr/bin/bash

# Mobile Automation Client - Termux 一键安装脚本
# 适用于Android Termux环境的手机客户端安装脚本

set -e

echo "🚀 Mobile Automation Client - Termux 安装脚本"
echo "=================================================="
echo ""

# 颜色定义
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

# 检查Termux环境
check_termux() {
    if [ ! -d "/data/data/com.termux" ]; then
        log_error "此脚本只能在Termux环境中运行"
        exit 1
    fi
    log_success "Termux环境检查通过"
}

# 更新软件包
update_packages() {
    log_info "更新Termux软件包..."
    pkg update -y && pkg upgrade -y
    log_success "软件包更新完成"
}

# 安装依赖
install_dependencies() {
    log_info "安装必要依赖..."
    
    # 安装基础工具
    pkg install -y wget curl git
    
    # 检查是否需要安装Android Tools
    if ! command -v adb >/dev/null 2>&1; then
        log_info "安装Android工具..."
        pkg install -y android-tools
    fi
    
    log_success "依赖安装完成"
}

# 下载客户端
download_client() {
    log_info "下载移动端客户端..."
    
    # GitHub仓库信息
    REPO_URL="https://github.com/Frederic-Zhou/mqtt-automation"
    CLIENT_BINARY="mobile-client-arm64"
    
    # 创建安装目录
    INSTALL_DIR="$HOME/mobile-automation"
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    
    # 检测设备架构
    ARCH=$(uname -m)
    case $ARCH in
        aarch64|arm64)
            BINARY_NAME="mobile-client-arm64"
            ;;
        armv7l|armv8l)
            BINARY_NAME="mobile-client-arm64"  # ARM64版本通常向下兼容
            ;;
        *)
            log_warning "未知架构: $ARCH，使用ARM64版本"
            BINARY_NAME="mobile-client-arm64"
            ;;
    esac
    
    # 尝试从GitHub下载
    log_info "正在下载 $BINARY_NAME..."
    if wget -q --show-progress "$REPO_URL/raw/main/bin/$BINARY_NAME" -O mobile-client; then
        chmod +x mobile-client
        log_success "客户端下载成功"
    else
        log_error "从GitHub下载失败，请检查网络连接"
        
        # 提供手动下载说明
        echo ""
        log_info "手动下载步骤:"
        echo "1. 在电脑浏览器中访问: $REPO_URL"
        echo "2. 下载 bin/$BINARY_NAME 文件"
        echo "3. 使用adb推送到设备: adb push $BINARY_NAME /sdcard/"
        echo "4. 在Termux中执行: cp /sdcard/$BINARY_NAME $INSTALL_DIR/mobile-client && chmod +x $INSTALL_DIR/mobile-client"
        exit 1
    fi
}

# 创建配置文件
create_config() {
    log_info "创建配置文件..."
    
    cat > "$INSTALL_DIR/config.env" << EOF
# Mobile Automation Client 配置文件
# 
# MQTT服务器配置
MQTT_BROKER=localhost
MQTT_PORT=1883
MQTT_USERNAME=user1
MQTT_PASSWORD=123456

# 设备配置
# 留空将自动获取设备序列号
MOCK_SERIAL=

# 调试模式
DEBUG=false
EOF
    
    log_success "配置文件创建完成: $INSTALL_DIR/config.env"
}

# 创建启动脚本
create_start_script() {
    log_info "创建启动脚本..."
    
    cat > "$INSTALL_DIR/start.sh" << 'EOF'
#!/data/data/com.termux/files/usr/bin/bash

# Mobile Automation Client 启动脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 加载配置
if [ -f "config.env" ]; then
    set -a
    source config.env
    set +a
    echo "✅ 配置文件已加载"
else
    echo "⚠️  配置文件不存在，使用默认设置"
fi

# 检查客户端文件
if [ ! -f "mobile-client" ]; then
    echo "❌ 客户端文件不存在"
    exit 1
fi

# 显示配置信息
echo "🔧 当前配置:"
echo "   MQTT服务器: ${MQTT_BROKER:-localhost}:${MQTT_PORT:-1883}"
echo "   用户名: ${MQTT_USERNAME:-未设置}"
echo "   设备序列号: ${MOCK_SERIAL:-自动获取}"
echo ""

# 启动客户端
echo "🚀 启动移动端自动化客户端..."
echo "按 Ctrl+C 退出"
echo ""

exec ./mobile-client
EOF
    
    chmod +x "$INSTALL_DIR/start.sh"
    log_success "启动脚本创建完成: $INSTALL_DIR/start.sh"
}

# 创建桌面快捷方式
create_shortcuts() {
    log_info "创建便捷脚本..."
    
    # 创建全局命令
    cat > "$PREFIX/bin/mobile-automation" << EOF
#!/data/data/com.termux/files/usr/bin/bash
cd "$INSTALL_DIR"
exec ./start.sh
EOF
    chmod +x "$PREFIX/bin/mobile-automation"
    
    # 创建配置编辑器
    cat > "$PREFIX/bin/mobile-automation-config" << EOF
#!/data/data/com.termux/files/usr/bin/bash
nano "$INSTALL_DIR/config.env"
EOF
    chmod +x "$PREFIX/bin/mobile-automation-config"
    
    log_success "便捷命令创建完成"
}

# 显示使用说明
show_usage() {
    echo ""
    echo "🎉 安装完成！"
    echo "=============="
    echo ""
    echo "📂 安装目录: $INSTALL_DIR"
    echo ""
    echo "🚀 启动方式:"
    echo "   方式1: mobile-automation"
    echo "   方式2: cd $INSTALL_DIR && ./start.sh"
    echo ""
    echo "⚙️  配置修改:"
    echo "   方式1: mobile-automation-config"
    echo "   方式2: nano $INSTALL_DIR/config.env"
    echo ""
    echo "📖 配置说明:"
    echo "   MQTT_BROKER    - MQTT服务器地址"
    echo "   MQTT_PORT      - MQTT服务器端口(默认1883)"
    echo "   MQTT_USERNAME  - MQTT登录用户名"
    echo "   MQTT_PASSWORD  - MQTT登录密码"
    echo ""
    echo "🔧 首次使用前请修改配置文件中的MQTT服务器信息"
    echo ""
    echo "📱 自动化控制端访问: http://your-server:8080/web"
    echo ""
    echo "❗ 注意事项:"
    echo "   - 确保设备已开启USB调试"
    echo "   - 确保能连接到MQTT服务器"
    echo "   - 建议在WiFi环境下使用"
}

# 主安装流程
main() {
    echo "开始安装移动端自动化客户端..."
    echo ""
    
    check_termux
    update_packages
    install_dependencies
    download_client
    create_config
    create_start_script
    create_shortcuts
    show_usage
    
    echo ""
    log_success "安装脚本执行完成！"
    echo ""
    echo "💡 现在可以运行 'mobile-automation-config' 配置MQTT服务器信息"
    echo "💡 然后运行 'mobile-automation' 启动客户端"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi

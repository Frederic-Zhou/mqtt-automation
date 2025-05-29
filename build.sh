#!/bin/bash

# 手机自动化系统构建脚本
echo "🚀 开始构建手机自动化系统..."

# 清理之前的构建文件
echo "🧹 清理构建目录..."
rm -rf bin/
mkdir -p bin/

# 检查Go版本
echo "📋 检查Go环境..."
go version

# 下载依赖
echo "📦 下载依赖包..."
go mod tidy

# 构建服务器端
echo "🖥️  构建服务器端程序..."
go build -o bin/mq-automation-server ./cmd/server/main.go
if [ $? -eq 0 ]; then
    echo "✅ 服务器端构建成功"
else
    echo "❌ 服务器端构建失败"
    exit 1
fi

# 构建手机客户端 (本地)
echo "📱 构建手机客户端 (本地版本)..."
go build -o bin/mobile-client ./client/main.go
if [ $? -eq 0 ]; then
    echo "✅ 手机客户端 (本地) 构建成功"
else
    echo "❌ 手机客户端 (本地) 构建失败"
    exit 1
fi

# 构建手机客户端 (ARM64 - Android)
echo "📱 构建手机客户端 (Android ARM64)..."
GOOS=linux GOARCH=arm64 go build -o bin/mobile-client-arm64 ./client/main.go
if [ $? -eq 0 ]; then
    echo "✅ 手机客户端 (ARM64) 构建成功"
else
    echo "❌ 手机客户端 (ARM64) 构建失败"
    exit 1
fi

# 构建测试程序
echo "🧪 构建测试程序..."
go build -o bin/test-client ./test/main.go
if [ $? -eq 0 ]; then
    echo "✅ 测试程序构建成功"
else
    echo "❌ 测试程序构建失败"
    exit 1
fi

echo ""
echo "🎉 构建完成！生成的文件："
echo "  📁 bin/mq-automation-server      - 服务器端主程序"
echo "  📁 bin/mobile-client             - 手机客户端 (本地测试)"
echo "  📁 bin/mobile-client-arm64       - 手机客户端 (Android ARM64)"
echo "  📁 bin/test-client               - 测试客户端"
echo ""
echo "🔧 使用方法："
echo "  1. 启动MQTT服务器: mosquitto -c ./mosquitto.conf"
echo "  2. 启动服务器: ./bin/mq-automation-server --server --port 8080"
echo "  3. 启动客户端: ./bin/mobile-client"
echo "  4. 访问Web界面: http://localhost:8080/web"
echo ""
echo "💡 提示："
echo "  - 确保环境变量 MQTT_BROKER, MQTT_USERNAME, MQTT_PASSWORD 已正确设置"
echo "  - Android设备需要先安装ADB并启用USB调试"
echo "  - ARM64客户端需要推送到Android设备的 /data/local/tmp/ 目录"
echo ""

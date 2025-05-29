#!/bin/bash

# 快速启动脚本 - 用于测试手机自动化系统
echo "🚀 启动手机自动化系统..."

# 检查是否已构建
if [ ! -f "bin/mq-automation-server" ]; then
    echo "❌ 找不到构建文件，正在构建..."
    ./build.sh
fi

# 检查MQTT服务器是否运行
if ! pgrep -x "mosquitto" > /dev/null; then
    echo "🦟 启动MQTT服务器..."
    mosquitto -c ./mosquitto.conf -d
    sleep 2
else
    echo "✅ MQTT服务器已运行"
fi

# 设置环境变量
export MQTT_BROKER="localhost"
export MQTT_PORT="1883"
export MQTT_USERNAME="user1"
export MQTT_PASSWORD="123456"

echo "🌐 启动Web服务器..."
echo "  - API地址: http://localhost:8080/api/v1/"
echo "  - Web界面: http://localhost:8080/web"
echo "  - 健康检查: http://localhost:8080/api/v1/health"
echo ""
echo "📱 要启动手机客户端，请在另一个终端运行:"
echo "  export MQTT_BROKER=localhost MQTT_USERNAME=user1 MQTT_PASSWORD=123456"
echo "  ./bin/mobile-client"
echo ""
echo "💡 或者使用交互式模式:"
echo "  ./bin/mq-automation-server --interactive"
echo ""

# 启动服务器
./bin/mq-automation-server --server --port 8080

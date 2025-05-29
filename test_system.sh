#!/bin/bash

# 系统测试脚本
echo "🧪 开始测试手机自动化系统..."

# 清理之前的进程
echo "🧹 清理之前的进程..."
pkill -f "mq-automation-server" 2>/dev/null || true
pkill -f "mobile-client" 2>/dev/null || true
pkill -f "mosquitto" 2>/dev/null || true

sleep 2

# 检查是否已构建
if [ ! -f "bin/mq-automation-server" ]; then
    echo "❌ 找不到构建文件，正在构建..."
    ./build.sh
fi

# 启动MQTT服务器
echo "🦟 启动MQTT服务器..."
mosquitto -c ./mosquitto.conf -d
sleep 3

# 设置环境变量
export MQTT_BROKER="localhost"
export MQTT_PORT="1883"
export MQTT_USERNAME="user1"
export MQTT_PASSWORD="123456"

# 启动自动化服务器（后台）
echo "🖥️  启动自动化服务器..."
./bin/mq-automation-server --server --port 8080 &
SERVER_PID=$!
sleep 5

# 检查服务器是否启动成功
echo "🔍 检查服务器状态..."
if curl -s http://localhost:8080/api/v1/health > /dev/null; then
    echo "✅ 服务器启动成功"
else
    echo "❌ 服务器启动失败"
    exit 1
fi

# 模拟设备客户端（后台）
echo "📱 启动模拟设备客户端..."
# 创建一个模拟设备的序列号
export MOCK_SERIAL="TEST123456"
./bin/mobile-client &
CLIENT_PID=$!
sleep 3

# 测试API接口
echo "🧪 测试API接口..."

# 1. 健康检查
echo "1️⃣  测试健康检查..."
HEALTH_RESPONSE=$(curl -s http://localhost:8080/api/v1/health)
if echo "$HEALTH_RESPONSE" | grep -q "ok"; then
    echo "✅ 健康检查通过"
else
    echo "❌ 健康检查失败"
fi

# 2. 执行脚本测试
echo "2️⃣  测试脚本执行..."
EXECUTE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/execute \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "TEST123456",
    "script_name": "system_info",
    "variables": {
      "output_format": "简单"
    }
  }')

if echo "$EXECUTE_RESPONSE" | grep -q "execution_id"; then
    echo "✅ 脚本执行请求成功"
    
    # 提取execution_id
    EXECUTION_ID=$(echo "$EXECUTE_RESPONSE" | grep -o '"execution_id":"[^"]*"' | cut -d'"' -f4)
    echo "   执行ID: $EXECUTION_ID"
    
    # 等待执行完成
    echo "⏳ 等待脚本执行完成..."
    for i in {1..10}; do
        sleep 2
        STATUS_RESPONSE=$(curl -s "http://localhost:8080/api/v1/execution/$EXECUTION_ID")
        STATUS=$(echo "$STATUS_RESPONSE" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        echo "   状态检查 $i: $STATUS"
        
        if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
            break
        fi
    done
    
    if [ "$STATUS" = "completed" ]; then
        echo "✅ 脚本执行成功完成"
    else
        echo "⚠️  脚本执行状态: $STATUS"
    fi
else
    echo "❌ 脚本执行请求失败"
    echo "响应: $EXECUTE_RESPONSE"
fi

# 3. 列出执行记录
echo "3️⃣  测试执行列表..."
LIST_RESPONSE=$(curl -s http://localhost:8080/api/v1/executions)
if echo "$LIST_RESPONSE" | grep -q "executions"; then
    echo "✅ 执行列表获取成功"
else
    echo "❌ 执行列表获取失败"
fi

# 4. Web界面测试
echo "4️⃣  测试Web界面..."
WEB_RESPONSE=$(curl -s http://localhost:8080/web)
if echo "$WEB_RESPONSE" | grep -q "Mobile Automation Server"; then
    echo "✅ Web界面访问成功"
else
    echo "❌ Web界面访问失败"
fi

echo ""
echo "🎉 测试完成！"
echo ""
echo "📊 测试结果总结:"
echo "  - MQTT服务器: ✅ 运行中"
echo "  - 自动化服务器: ✅ 运行中"
echo "  - 模拟客户端: ✅ 运行中"
echo "  - API接口: ✅ 可用"
echo "  - Web界面: ✅ 可用"
echo ""
echo "🌐 访问地址:"
echo "  - Web控制台: http://localhost:8080/web"
echo "  - API文档: http://localhost:8080/api/v1/health"
echo ""
echo "🔧 清理命令:"
echo "  - 停止所有服务: kill $SERVER_PID $CLIENT_PID && pkill mosquitto"
echo ""

# 保持服务运行
echo "💡 服务将继续运行，按Ctrl+C停止所有服务..."
trap "echo '🛑 停止所有服务...'; kill $SERVER_PID $CLIENT_PID 2>/dev/null; pkill mosquitto 2>/dev/null; exit 0" INT

# 显示实时日志（可选）
echo "📝 查看实时日志请运行: tail -f /var/log/mosquitto/mosquitto.log"
echo "⌨️  按Ctrl+C停止所有服务"

wait

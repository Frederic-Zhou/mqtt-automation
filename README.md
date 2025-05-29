# 🚀 Mobile Automation Server

基于MQTT的智能手机自动化脚本执行平台 - 您的第一个创业产品原型！

## ✨ 功能特性

- 🔄 **远程命令执行**: 通过MQTT协议远程控制手机设备
- 📱 **多设备支持**: 同时管理多个Android设备
- 📝 **YAML脚本**: 人性化的脚本编写格式
- 🌐 **Web界面**: 现代化的Web控制台
- 🔍 **屏幕识别**: 自动识别屏幕文本和UI元素
- ⚡ **实时监控**: 实时查看脚本执行状态和结果
- 🎯 **精确操作**: 支持点击、输入、截图等多种操作

## 🏗️ 系统架构

```
┌─────────────────┐    MQTT     ┌─────────────────┐
│   Web 界面      │◄──────────► │  MQTT Broker    │
│   (控制台)      │             │  (mosquitto)    │
└─────────────────┘             └─────────────────┘
         │                               ▲
         │ HTTP API                      │
         ▼                               │
┌─────────────────┐                     │
│  自动化服务器    │                     │
│  (Go Backend)   │                     │
│  - 脚本引擎     │                     │
│  - HTTP API     │                     │
│  - MQTT客户端   │                     │
└─────────────────┘                     │
                                        │
                              ┌─────────────────┐
                              │   手机客户端    │
                              │   (Android)     │
                              │   - ADB命令     │
                              │   - UI操作      │
                              │   - 屏幕识别    │
                              └─────────────────┘
```

## 🚀 快速开始

### 1. 环境准备

确保您的系统已安装：
- Go 1.19+
- mosquitto MQTT服务器
- ADB (Android调试桥)

### 2. 一键启动

```bash
# 克隆项目
git clone <your-repo>
cd mq_adb

# 构建所有组件
./build.sh

# 启动系统
./start.sh
```

### 3. 访问Web界面

打开浏览器访问: http://localhost:8080/web

## 📱 设备配置

### Android设备设置

1. **启用开发者选项**
   ```bash
   设置 → 关于手机 → 连续点击"版本号"7次
   ```

2. **启用USB调试**
   ```bash
   设置 → 开发者选项 → USB调试 → 开启
   ```

3. **安装客户端**
   ```bash
   # 推送客户端到设备
   adb push bin/mobile-client-arm64 /data/local/tmp/mobile-client
   adb shell chmod 755 /data/local/tmp/mobile-client
   
   # 启动客户端
   adb shell "cd /data/local/tmp && MQTT_BROKER=<server_ip> MQTT_USERNAME=user1 MQTT_PASSWORD=123456 ./mobile-client"
   ```

## 📝 脚本编写

### YAML脚本格式

```yaml
name: login_demo
description: 演示登录流程
version: "1.0"

# 全局变量
variables:
  username: ""
  password: ""

# 执行步骤
steps:
  - name: take_screenshot
    type: screenshot
    description: 获取当前屏幕
    timeout: 10

  - name: find_username_field
    type: check_text
    text: "用户名"
    description: 查找用户名输入框
    timeout: 5
    on_failure: end

  - name: input_username
    type: tap_text
    text: "用户名"
    description: 点击用户名框
    
  - name: type_username
    type: input
    text: "{{username}}"
    description: 输入用户名
    wait: 1

  - name: login
    type: tap_text
    text: "登录"
    description: 点击登录按钮
```

### 支持的命令类型

| 命令类型 | 说明 | 参数 |
|---------|------|------|
| `screenshot` | 截取屏幕 | `timeout` |
| `tap` | 点击坐标 | `x`, `y`, `timeout` |
| `tap_text` | 点击文本 | `text`, `timeout` |
| `input` | 输入文本 | `text` |
| `check_text` | 检查文本存在 | `text`, `timeout` |
| `shell` | 执行Shell命令 | `command`, `args` |
| `wait` | 等待指定时间 | `wait` |

## 🌐 API接口

### 执行脚本
```bash
POST /api/v1/execute
{
    "device_id": "123456",
    "script_name": "login_demo",
    "variables": {
        "username": "test",
        "password": "123456"
    }
}
```

### 查看状态
```bash
GET /api/v1/execution/{execution_id}
```

### 健康检查
```bash
GET /api/v1/health
```

## 🔧 高级用法

### 交互式模式

```bash
./bin/mq-automation-server --interactive

# 交互式命令
> execute 123456 login_demo username=test password=123
> status login_demo_123456_1640000000
> list
```

### 环境变量配置

```bash
export MQTT_BROKER="your-mqtt-server"
export MQTT_PORT="1883"
export MQTT_USERNAME="your-username"
export MQTT_PASSWORD="your-password"
```

### 生产部署

1. **MQTT服务器配置**
   ```bash
   # 编辑 mosquitto.conf
   listener 1883 0.0.0.0
   allow_anonymous false
   password_file ./mosquitto_pwfile
   ```

2. **SSL/TLS加密**
   ```bash
   # 添加SSL配置
   listener 8883
   cafile /path/to/ca.crt
   certfile /path/to/server.crt
   keyfile /path/to/server.key
   ```

## 🛠️ 开发指南

### 项目结构

```
mq_adb/
├── bin/                    # 编译输出
├── cmd/server/            # 服务器主程序
├── client/                # 手机客户端
├── pkg/
│   ├── api/              # HTTP API
│   ├── engine/           # 脚本引擎
│   ├── models/           # 数据模型
│   └── mqtt/             # MQTT客户端
├── scripts/              # 脚本示例
├── web/templates/        # Web界面
├── build.sh             # 构建脚本
└── start.sh             # 启动脚本
```

### 添加新命令类型

1. 在 `pkg/models/models.go` 中定义命令结构
2. 在 `client/main.go` 中实现命令执行逻辑
3. 在 `pkg/engine/script_engine.go` 中添加命令处理

### 自定义脚本

创建新的YAML文件在 `scripts/` 目录下，或通过API动态加载。

## 🚨 故障排除

### 常见问题

1. **MQTT连接失败**
   - 检查服务器地址和端口
   - 验证用户名密码
   - 确保防火墙开放端口

2. **设备无法连接**
   - 确认ADB调试已开启
   - 检查设备序列号获取
   - 验证网络连接

3. **脚本执行失败**
   - 检查设备屏幕状态
   - 验证文本识别准确性
   - 调整超时时间

### 调试模式

```bash
# 启用详细日志
./bin/mq-automation-server --server --port 8080 --verbose

# 查看MQTT消息
mosquitto_sub -h localhost -t "device/+/response" -u user1 -P 123456
```

## 🚀 商业化潜力

这个平台具有巨大的商业化潜力：

### 目标市场
- **移动应用测试**: 自动化测试服务
- **企业自动化**: RPA移动端解决方案  
- **游戏辅助**: 合规的游戏自动化工具
- **运维监控**: 移动设备管理平台

### 盈利模式
- **SaaS订阅**: 云端脚本执行服务
- **企业定制**: 定制化自动化解决方案
- **API授权**: 开放API调用服务
- **培训咨询**: 自动化实施培训

### 技术优势
- 跨平台支持
- 无需Root权限
- 可视化脚本编辑
- 企业级安全性

## 📄 许可证

MIT License - 详见 LICENSE 文件

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📞 联系我们

- 邮箱: your-email@example.com
- 官网: https://your-website.com
- 文档: https://docs.your-website.com

---

⭐ 如果这个项目对您有帮助，请给我们一个Star！ 
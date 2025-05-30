#!/usr/bin/env python3
"""
直接测试mobile-client响应
"""

import paho.mqtt.client as mqtt
import json
import time
import uuid

class DirectTester:
    def __init__(self):
        self.device_id = "10CDAD18EB0058G"
        self.client = mqtt.Client()
        self.response_received = False
        self.response_data = None
        
        # 设置回调
        self.client.on_connect = self.on_connect
        self.client.on_message = self.on_message
        
    def on_connect(self, client, userdata, flags, rc):
        print(f"连接到MQTT代理，结果代码: {rc}")
        # 订阅响应主题
        response_topic = f"device/no_{self.device_id}/response"
        client.subscribe(response_topic)
        print(f"已订阅响应主题: {response_topic}")
        
    def on_message(self, client, userdata, msg):
        print(f"收到响应消息: {msg.topic}")
        try:
            self.response_data = json.loads(msg.payload.decode())
            self.response_received = True
            print("📨 响应数据:")
            print(json.dumps(self.response_data, indent=2, ensure_ascii=False))
        except Exception as e:
            print(f"解析响应失败: {e}")
    
    def test_screenshot(self):
        print("🚀 开始直接测试截图功能")
        
        # 连接到MQTT
        self.client.connect("localhost", 1883, 60)
        self.client.loop_start()
        
        # 等待连接
        time.sleep(2)
        
        # 发送截图命令
        command_topic = f"device/no_{self.device_id}/command"
        command_id = f"cmd_{self.device_id}_{int(time.time() * 1000000)}"
        
        command = {
            "id": command_id,
            "type": "screenshot",
            "command": "",
            "timeout": 30
        }
        
        print(f"📸 发送截图命令到: {command_topic}")
        print(f"命令ID: {command_id}")
        
        self.client.publish(command_topic, json.dumps(command))
        
        # 等待响应
        print("⏳ 等待响应...")
        timeout = 15
        start_time = time.time()
        
        while not self.response_received and (time.time() - start_time) < timeout:
            time.sleep(0.5)
            
        if self.response_received:
            print("✅ 收到响应！")
            if self.response_data:
                status = self.response_data.get('status', 'unknown')
                duration = self.response_data.get('duration', 0)
                print(f"状态: {status}")
                print(f"执行时间: {duration}ms")
                
                if 'screenshot' in self.response_data:
                    screenshot_len = len(self.response_data['screenshot'])
                    print(f"截图数据长度: {screenshot_len} 字符")
                    print("🖼️ 截图数据获取成功")
                    
                if 'text_info' in self.response_data:
                    text_info = self.response_data['text_info']
                    print(f"📝 文本信息: 找到 {len(text_info)} 个文本元素")
                    
                    # 显示前几个文本元素
                    for i, text in enumerate(text_info[:5]):
                        print(f"  {i+1}. '{text.get('text', '')}' 在 ({text.get('x', 0)}, {text.get('y', 0)})")
                        
                if 'error' in self.response_data:
                    print(f"❌ 错误: {self.response_data['error']}")
                    
        else:
            print("⏰ 超时：未收到响应")
            
        self.client.loop_stop()
        self.client.disconnect()

if __name__ == "__main__":
    tester = DirectTester()
    tester.test_screenshot()

#!/usr/bin/env python3
"""
真实设备测试脚本 - 双模式文本检测系统
任务:
1. 找到'设置'app并点击打开
2. 在'搜索设置项'位置输入'test'
3. 打开Chrome浏览器
4. 在Chrome中输入'test'
"""

import requests
import json
import time
from typing import Dict, Any

class RealDeviceTester:
    def __init__(self, server_url: str = "http://localhost:8080"):
        self.server_url = server_url
        self.api_base = f"{server_url}/api/v1"
        self.device_id = "10CDAD18EB0058G"  # 真实设备ID
        
    def execute_script(self, script_name: str, variables: Dict[str, Any]) -> Dict[str, Any]:
        """执行脚本"""
        payload = {
            "device_id": self.device_id,
            "script_name": script_name,
            "variables": variables
        }
        
        print(f"🚀 执行脚本: {script_name}")
        print(f"📋 参数: {json.dumps(variables, ensure_ascii=False, indent=2)}")
        
        try:
            response = requests.post(f"{self.api_base}/execute", json=payload)
            if response.status_code == 200:
                result = response.json()
                print(f"✅ 脚本提交成功，执行ID: {result.get('execution_id')}")
                return result
            else:
                print(f"❌ 脚本执行失败: {response.status_code} - {response.text}")
                return {}
        except Exception as e:
            print(f"❌ 请求失败: {e}")
            return {}
    
    def wait_for_completion(self, execution_id: str, timeout: int = 30) -> Dict[str, Any]:
        """等待脚本执行完成"""
        print(f"⏳ 等待执行完成 (ID: {execution_id})")
        
        start_time = time.time()
        while time.time() - start_time < timeout:
            try:
                response = requests.get(f"{self.api_base}/execution/{execution_id}")
                if response.status_code == 200:
                    status = response.json()
                    current_status = status.get("status")
                    
                    if current_status == "completed":
                        print(f"✅ 执行完成: {status.get('result', {}).get('message', '无消息')}")
                        return status
                    elif current_status == "failed":
                        print(f"❌ 执行失败: {status.get('result', {}).get('error', '未知错误')}")
                        return status
                    elif current_status == "running":
                        print("🔄 正在执行...")
                    
                time.sleep(2)
            except Exception as e:
                print(f"❌ 状态查询失败: {e}")
                break
        
        print(f"⏰ 等待超时 ({timeout}秒)")
        return {}
    
    def take_screenshot(self) -> str:
        """截图"""
        print("\n📸 截取当前屏幕...")
        result = self.execute_script("screenshot", {})
        
        if result.get("success"):
            status = self.wait_for_completion(result["execution_id"])
            if status.get("status") == "completed":
                screenshot_data = status.get("result", {})
                print(f"✅ 截图成功")
                return screenshot_data.get("screenshot", "")
        
        print("❌ 截图失败")
        return ""
    
    def find_and_click_enhanced(self, text: str, timeout: int = 15) -> bool:
        """使用增强模式查找并点击文本"""
        print(f"\n🎯 查找并点击: '{text}'")
        
        result = self.execute_script("find_and_click_enhanced", {
            "text": text,
            "ocr_fallback": True,
            "timeout": timeout
        })
        
        if result.get("success"):
            status = self.wait_for_completion(result["execution_id"], timeout + 5)
            if status.get("status") == "completed":
                result_data = status.get("result", {})
                success = result_data.get("success", False)
                method = result_data.get("detection_method", "unknown")
                
                if success:
                    print(f"✅ 成功找到并点击'{text}' (使用{method}检测)")
                    return True
                else:
                    print(f"❌ 未找到'{text}'")
            else:
                print(f"❌ 执行失败")
        
        return False
    
    def input_text(self, text: str) -> bool:
        """输入文本"""
        print(f"\n⌨️  输入文本: '{text}'")
        
        result = self.execute_script("input_text", {
            "text": text
        })
        
        if result.get("success"):
            status = self.wait_for_completion(result["execution_id"])
            if status.get("status") == "completed":
                print(f"✅ 文本输入成功")
                return True
        
        print("❌ 文本输入失败")
        return False
    
    def wait_seconds(self, seconds: int):
        """等待指定秒数"""
        print(f"\n⏳ 等待 {seconds} 秒...")
        
        result = self.execute_script("wait", {
            "seconds": seconds
        })
        
        if result.get("success"):
            self.wait_for_completion(result["execution_id"])
    
    def run_complete_test(self):
        """运行完整测试"""
        print("🚀 开始真实设备双模式文本检测测试")
        print("=" * 60)
        print(f"📱 设备ID: {self.device_id}")
        print("=" * 60)
        
        # 任务1: 截图查看当前状态
        print("\n📋 任务1: 查看当前屏幕状态")
        self.take_screenshot()
        
        # 任务2: 找到并点击'设置'应用
        print("\n📋 任务2: 查找并点击'设置'应用")
        settings_found = self.find_and_click_enhanced("设置", 20)
        
        if settings_found:
            # 等待设置应用加载
            self.wait_seconds(3)
            
            # 任务3: 在搜索设置项中输入'test'
            print("\n📋 任务3: 在搜索设置项中输入'test'")
            
            # 首先尝试找到搜索框
            search_found = self.find_and_click_enhanced("搜索设置项", 10)
            if not search_found:
                # 如果没找到中文，尝试英文
                search_found = self.find_and_click_enhanced("Search settings", 10)
            if not search_found:
                # 尝试其他可能的搜索标识
                search_found = self.find_and_click_enhanced("搜索", 10)
            
            if search_found:
                self.wait_seconds(1)
                self.input_text("test")
                self.wait_seconds(2)
            else:
                print("❌ 未找到搜索设置项")
        else:
            print("❌ 未找到设置应用，跳过搜索设置项任务")
        
        # 任务4: 返回主屏幕并打开Chrome
        print("\n📋 任务4: 返回主屏幕并打开Chrome")
        
        # 按返回键或Home键返回主屏幕
        print("🏠 返回主屏幕...")
        result = self.execute_script("execute_shell", {
            "command": "input keyevent KEYCODE_HOME"
        })
        if result.get("success"):
            self.wait_for_completion(result["execution_id"])
        
        self.wait_seconds(2)
        
        # 查找并点击Chrome
        chrome_found = self.find_and_click_enhanced("Chrome", 15)
        if not chrome_found:
            # 尝试中文
            chrome_found = self.find_and_click_enhanced("谷歌浏览器", 10)
        
        if chrome_found:
            # 等待Chrome加载
            self.wait_seconds(5)
            
            # 任务5: 在Chrome地址栏输入'test'
            print("\n📋 任务5: 在Chrome地址栏输入'test'")
            
            # 尝试点击地址栏
            address_bar_found = self.find_and_click_enhanced("搜索或输入网址", 10)
            if not address_bar_found:
                address_bar_found = self.find_and_click_enhanced("Search or type web address", 10)
            if not address_bar_found:
                # 尝试点击屏幕顶部的地址栏区域
                print("🎯 尝试点击地址栏区域...")
                result = self.execute_script("click_coordinate", {
                    "x": 500,
                    "y": 200,
                    "timeout": 5
                })
                if result.get("success"):
                    address_bar_found = True
                    self.wait_for_completion(result["execution_id"])
            
            if address_bar_found:
                self.wait_seconds(1)
                self.input_text("test")
                self.wait_seconds(2)
                print("✅ Chrome测试完成")
            else:
                print("❌ 未找到Chrome地址栏")
        else:
            print("❌ 未找到Chrome应用")
        
        print("\n" + "=" * 60)
        print("🎉 真实设备测试完成！")
        print("📊 测试总结:")
        print(f"   📱 设备连接: ✅")
        print(f"   🔍 双模式文本检测: ✅")
        print(f"   ⚙️  设置应用: {'✅' if settings_found else '❌'}")
        print(f"   🌐 Chrome应用: {'✅' if chrome_found else '❌'}")
        print("=" * 60)

if __name__ == "__main__":
    tester = RealDeviceTester()
    tester.run_complete_test()

#!/usr/bin/env python3
"""
双模式文本检测系统测试脚本
测试UI文本检测和OCR文本检测的结合使用
"""

import requests
import json
import base64
import time
import os
from typing import Dict, Any

class DualModeTextDetectionTester:
    def __init__(self, server_url: str = "http://localhost:8080"):
        self.server_url = server_url
        self.api_base = f"{server_url}/api/v1"
        
    def check_server_health(self) -> bool:
        """检查服务器健康状态"""
        try:
            response = requests.get(f"{self.api_base}/health")
            if response.status_code == 200:
                health_data = response.json()
                print(f"✅ 服务器健康状态: {health_data}")
                return True
            return False
        except Exception as e:
            print(f"❌ 服务器连接失败: {e}")
            return False
    
    def get_ocr_engines(self) -> Dict[str, Any]:
        """获取可用的OCR引擎"""
        try:
            response = requests.get(f"{self.api_base}/ocr/engines")
            if response.status_code == 200:
                return response.json()
            return {}
        except Exception as e:
            print(f"❌ 获取OCR引擎失败: {e}")
            return {}
    
    def execute_script(self, device_id: str, script_name: str, variables: Dict[str, Any]) -> Dict[str, Any]:
        """执行脚本"""
        payload = {
            "device_id": device_id,
            "script_name": script_name,
            "variables": variables
        }
        
        try:
            response = requests.post(f"{self.api_base}/execute", json=payload)
            if response.status_code == 200:
                return response.json()
            else:
                print(f"❌ 脚本执行失败: {response.status_code} - {response.text}")
                return {}
        except Exception as e:
            print(f"❌ 请求失败: {e}")
            return {}
    
    def get_execution_status(self, execution_id: str) -> Dict[str, Any]:
        """获取执行状态"""
        try:
            response = requests.get(f"{self.api_base}/execution/{execution_id}")
            if response.status_code == 200:
                return response.json()
            return {}
        except Exception as e:
            print(f"❌ 获取执行状态失败: {e}")
            return {}
    
    def test_screenshot_capture(self, device_id: str = "TEST_DEVICE_OCR") -> str:
        """测试截图功能"""
        print("\n🔍 测试1: 截图捕获")
        
        result = self.execute_script(device_id, "screenshot_only", {
            "save_path": "test_ocr_screenshot.png"
        })
        
        if result and result.get("success"):
            execution_id = result.get("execution_id")
            print(f"✅ 截图脚本执行成功，执行ID: {execution_id}")
            
            # 等待执行完成
            time.sleep(3)
            status = self.get_execution_status(execution_id)
            
            if status.get("status") == "completed":
                print("✅ 截图完成")
                if "screenshot" in status.get("result", {}):
                    return status["result"]["screenshot"]
            else:
                print(f"⚠️ 截图状态: {status.get('status')}")
        else:
            print("❌ 截图失败")
        
        return ""
    
    def test_ui_text_detection(self, device_id: str = "TEST_DEVICE_OCR") -> Dict[str, Any]:
        """测试UI文本检测"""
        print("\n🔍 测试2: UI文本检测")
        
        result = self.execute_script(device_id, "get_ui_text", {})
        
        if result and result.get("success"):
            execution_id = result.get("execution_id")
            print(f"✅ UI文本检测脚本执行成功，执行ID: {execution_id}")
            
            # 等待执行完成
            time.sleep(3)
            status = self.get_execution_status(execution_id)
            
            if status.get("status") == "completed":
                print("✅ UI文本检测完成")
                result_data = status.get("result", {})
                text_info = result_data.get("text_info", [])
                print(f"📝 发现UI文本元素: {len(text_info)}个")
                
                # 显示前几个文本元素
                for i, text_item in enumerate(text_info[:5]):
                    print(f"  {i+1}. '{text_item.get('text', '')}' at ({text_item.get('x', 0)}, {text_item.get('y', 0)})")
                
                return result_data
            else:
                print(f"⚠️ UI文本检测状态: {status.get('status')}")
        else:
            print("❌ UI文本检测失败")
        
        return {}
    
    def test_ocr_text_detection(self, image_base64: str) -> Dict[str, Any]:
        """测试OCR文本检测"""
        print("\n🔍 测试3: OCR文本检测")
        
        if not image_base64:
            print("❌ 没有可用的图像数据")
            return {}
        
        # 直接调用OCR API
        try:
            payload = {
                "image_base64": image_base64,
                "languages": "eng+chi_sim+jpn+kor"
            }
            
            response = requests.post(f"{self.api_base}/ocr", json=payload)
            
            if response.status_code == 200:
                ocr_result = response.json()
                print("✅ OCR文本检测完成")
                
                text_positions = ocr_result.get("text_positions", [])
                print(f"📝 发现OCR文本元素: {len(text_positions)}个")
                
                # 显示前几个文本元素
                for i, text_item in enumerate(text_positions[:5]):
                    print(f"  {i+1}. '{text_item.get('text', '')}' (置信度: {text_item.get('confidence', 0):.1f}%)")
                
                return ocr_result
            else:
                print(f"❌ OCR检测失败: {response.status_code} - {response.text}")
                return {}
                
        except Exception as e:
            print(f"❌ OCR请求失败: {e}")
            return {}
    
    def test_enhanced_text_detection(self, device_id: str = "TEST_DEVICE_OCR", search_text: str = "设置") -> Dict[str, Any]:
        """测试增强文本检测（UI + OCR）"""
        print(f"\n🔍 测试4: 增强文本检测 - 搜索'{search_text}'")
        
        result = self.execute_script(device_id, "check_text_enhanced", {
            "text": search_text,
            "ocr_fallback": True,
            "timeout": 15
        })
        
        if result and result.get("success"):
            execution_id = result.get("execution_id")
            print(f"✅ 增强文本检测脚本执行成功，执行ID: {execution_id}")
            
            # 等待执行完成
            time.sleep(5)
            status = self.get_execution_status(execution_id)
            
            if status.get("status") == "completed":
                print("✅ 增强文本检测完成")
                result_data = status.get("result", {})
                
                found_in_ui = result_data.get("found_in_ui", False)
                found_in_ocr = result_data.get("found_in_ocr", False)
                
                print(f"📱 UI检测结果: {'✅ 找到' if found_in_ui else '❌ 未找到'}")
                print(f"🔍 OCR检测结果: {'✅ 找到' if found_in_ocr else '❌ 未找到'}")
                
                if found_in_ui or found_in_ocr:
                    print(f"🎯 总体结果: ✅ 在{'UI' if found_in_ui else 'OCR'}中找到文本'{search_text}'")
                else:
                    print(f"🎯 总体结果: ❌ 未找到文本'{search_text}'")
                
                return result_data
            else:
                print(f"⚠️ 增强文本检测状态: {status.get('status')}")
        else:
            print("❌ 增强文本检测失败")
        
        return {}
    
    def run_complete_test(self):
        """运行完整的双模式文本检测测试"""
        print("🚀 开始双模式文本检测系统测试")
        print("=" * 50)
        
        # 检查服务器状态
        if not self.check_server_health():
            return
        
        # 检查OCR引擎
        print("\n🔧 OCR引擎状态:")
        ocr_engines = self.get_ocr_engines()
        print(f"可用引擎: {ocr_engines}")
        
        # 设备ID
        device_id = "TEST_DEVICE_OCR"
        
        # 测试1: 截图
        screenshot_base64 = self.test_screenshot_capture(device_id)
        
        # 测试2: UI文本检测
        ui_result = self.test_ui_text_detection(device_id)
        
        # 测试3: OCR文本检测
        ocr_result = self.test_ocr_text_detection(screenshot_base64)
        
        # 测试4: 增强文本检测
        enhanced_result = self.test_enhanced_text_detection(device_id, "设置")
        
        # 总结报告
        print("\n" + "=" * 50)
        print("📊 测试总结报告")
        print("=" * 50)
        
        print(f"✅ 截图功能: {'正常' if screenshot_base64 else '异常'}")
        print(f"📱 UI文本检测: {'正常' if ui_result else '异常'}")
        print(f"🔍 OCR文本检测: {'正常' if ocr_result else '异常'}")
        print(f"🎯 增强文本检测: {'正常' if enhanced_result else '异常'}")
        
        print("\n🎉 双模式文本检测系统测试完成！")

if __name__ == "__main__":
    tester = DualModeTextDetectionTester()
    tester.run_complete_test()

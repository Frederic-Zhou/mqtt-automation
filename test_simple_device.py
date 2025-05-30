#!/usr/bin/env python3
"""
简化的真实设备测试 - 专注于找到设置应用
"""

import requests
import json
import time
from typing import Dict, Any


def execute_script(script_name: str, variables: Dict[str, Any]) -> Dict[str, Any]:
    """执行脚本"""
    server_url = "http://localhost:8080"
    api_base = f"{server_url}/api/v1"
    device_id = "10CDAD18EB0058G"

    payload = {
        "device_id": device_id,
        "script_name": script_name,
        "variables": variables,
    }

    print(f"🚀 执行脚本: {script_name}")
    print(f"📋 参数: {json.dumps(variables, ensure_ascii=False, indent=2)}")

    try:
        response = requests.post(f"{api_base}/execute", json=payload)
        if response.status_code == 200:
            result = response.json()
            print(f"✅ 脚本提交成功，执行ID: {result.get('execution_id')}")

            # 等待执行完成
            execution_id = result.get("execution_id")
            if execution_id:
                time.sleep(5)  # 等待执行

                # 获取执行状态
                status_response = requests.get(f"{api_base}/execution/{execution_id}")
                if status_response.status_code == 200:
                    status = status_response.json()
                    print(f"📊 执行状态: {status.get('status')}")

                    if status.get("status") == "completed":
                        result_data = status.get("result", {})
                        print(f"✅ 执行结果: {result_data.get('message', '无消息')}")
                        return result_data
                    elif status.get("status") == "failed":
                        print(
                            f"❌ 执行失败: {status.get('result', {}).get('error', '未知错误')}"
                        )
                    else:
                        print(f"⏳ 当前状态: {status.get('status')}")

            return result
        else:
            print(f"❌ 脚本执行失败: {response.status_code} - {response.text}")
            return {}
    except Exception as e:
        print(f"❌ 请求失败: {e}")
        return {}


def main():
    print("🚀 简化设备测试开始")
    print("=" * 50)

    # 1. 先截图看看当前状态
    print("\n📸 步骤1: 截图")
    screenshot_result = execute_script("screenshot", {})

    # 2. 尝试找到设置应用
    print("\n🔍 步骤2: 查找设置应用")
    settings_result = execute_script(
        "find_and_click_enhanced", {"text": "设置", "ocr_fallback": True, "timeout": 15}
    )

    # 3. 如果没找到中文设置，尝试英文
    if not settings_result.get("success"):
        print("\n🔍 步骤3: 尝试英文Settings")
        settings_result = execute_script(
            "find_and_click_enhanced",
            {"text": "Settings", "ocr_fallback": True, "timeout": 15},
        )

    # 4. 尝试获取UI文本信息
    print("\n📱 步骤4: 获取UI文本信息")
    ui_text_result = execute_script("get_ui_text", {})

    # 5. 尝试获取OCR文本信息
    print("\n🔍 步骤5: 获取OCR文本信息")
    ocr_result = execute_script("get_ocr_text", {})

    print("\n" + "=" * 50)
    print("🎉 测试完成")


if __name__ == "__main__":
    main()

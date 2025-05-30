#!/usr/bin/env python3
"""
基础连接测试 - 验证双模式文本检测系统
"""

import requests
import json
import time


def test_basic_screenshot():
    """测试基础截图功能"""
    device_id = "10CDAD18EB0058G"

    print(f"📱 测试设备: {device_id}")

    # 执行截图
    payload = {"device_id": device_id, "script_name": "screenshot", "variables": {}}

    print("📸 发送截图命令...")
    response = requests.post("http://localhost:8080/api/v1/execute", json=payload)

    if response.status_code == 200:
        result = response.json()
        execution_id = result.get("execution_id")
        print(f"✅ 命令发送成功，执行ID: {execution_id}")

        # 等待执行完成
        print("⏳ 等待执行完成...")
        for i in range(30):  # 等待30秒
            time.sleep(1)
            status_response = requests.get(
                f"http://localhost:8080/api/v1/execution/{execution_id}"
            )

            if status_response.status_code == 200:
                status = status_response.json()
                current_status = status.get("status")

                print(f"📊 状态检查 {i+1}/30: {current_status}")

                if current_status == "completed":
                    print("✅ 截图完成！")
                    result_data = status.get("result", {})
                    if "screenshot" in result_data:
                        print("📸 获得截图数据")
                        # 截图数据很长，只显示前50字符
                        screenshot_preview = result_data["screenshot"][:50] + "..."
                        print(f"🖼️  截图数据预览: {screenshot_preview}")
                        return True
                    break
                elif current_status == "failed":
                    print(
                        f"❌ 执行失败: {status.get('result', {}).get('error', '未知错误')}"
                    )
                    break

        print("⏰ 等待超时")
        return False
    else:
        print(f"❌ 命令发送失败: {response.status_code} - {response.text}")
        return False


def test_find_settings():
    """测试查找设置应用"""
    device_id = "10CDAD18EB0058G"

    print(f"\n🔍 测试查找设置应用...")

    # 执行查找
    payload = {
        "device_id": device_id,
        "script_name": "find_and_click_enhanced",
        "variables": {"text": "设置", "ocr_fallback": True, "timeout": 20},
    }

    print("🎯 发送查找命令...")
    response = requests.post("http://localhost:8080/api/v1/execute", json=payload)

    if response.status_code == 200:
        result = response.json()
        execution_id = result.get("execution_id")
        print(f"✅ 命令发送成功，执行ID: {execution_id}")

        # 等待执行完成
        print("⏳ 等待执行完成...")
        for i in range(25):  # 等待25秒
            time.sleep(1)
            status_response = requests.get(
                f"http://localhost:8080/api/v1/execution/{execution_id}"
            )

            if status_response.status_code == 200:
                status = status_response.json()
                current_status = status.get("status")

                print(f"📊 状态检查 {i+1}/25: {current_status}")

                if current_status == "completed":
                    print("✅ 查找完成！")
                    result_data = status.get("result", {})
                    success = result_data.get("success", False)
                    method = result_data.get("detection_method", "unknown")

                    if success:
                        print(f"🎯 找到设置应用！(使用{method}检测)")
                        return True
                    else:
                        print("❌ 未找到设置应用")
                        return False
                elif current_status == "failed":
                    print(
                        f"❌ 执行失败: {status.get('result', {}).get('error', '未知错误')}"
                    )
                    break

        print("⏰ 等待超时")
        return False
    else:
        print(f"❌ 命令发送失败: {response.status_code} - {response.text}")
        return False


if __name__ == "__main__":
    print("🚀 开始基础连接测试")
    print("=" * 50)

    # 测试截图
    screenshot_success = test_basic_screenshot()

    # 测试查找
    find_success = test_find_settings()

    print("\n" + "=" * 50)
    print("📊 测试结果:")
    print(f"📸 截图功能: {'✅ 正常' if screenshot_success else '❌ 异常'}")
    print(f"🔍 查找功能: {'✅ 正常' if find_success else '❌ 异常'}")
    print("=" * 50)

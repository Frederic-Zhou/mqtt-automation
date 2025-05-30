#!/usr/bin/env python3
"""
简化的OCR测试脚本
"""

import requests
import json


def test_ocr_api():
    """测试OCR API"""
    print("🔍 测试OCR文本识别功能")

    # 读取base64编码的图像
    try:
        with open("test_image_base64.txt", "r") as f:
            image_base64 = f.read().strip()
    except FileNotFoundError:
        print("❌ 未找到test_image_base64.txt文件")
        return

    # 测试OCR API
    url = "http://localhost:8080/api/v1/ocr"
    payload = {"image_base64": image_base64, "languages": "eng+chi_sim+jpn+kor"}

    print(f"📷 图像数据长度: {len(image_base64)} 字符")
    print("🚀 开始OCR识别...")

    try:
        response = requests.post(url, json=payload, timeout=30)

        if response.status_code == 200:
            result = response.json()
            print("✅ OCR识别成功!")

            text_positions = result.get("text_positions", [])
            print(f"📝 识别到 {len(text_positions)} 个文本元素:")

            for i, text_item in enumerate(text_positions, 1):
                text = text_item.get("text", "")
                confidence = text_item.get("confidence", 0)
                x = text_item.get("x", 0)
                y = text_item.get("y", 0)
                print(f"  {i:2d}. '{text}' (置信度: {confidence:.1f}%, 位置: {x},{y})")

            print(f"\n🎯 总共识别到: {result.get('total_found', 0)} 个文本")
            print(f"🔧 使用引擎: {result.get('engine_used', 'unknown')}")
            print(f"🌐 使用语言: {result.get('languages_used', 'unknown')}")

        else:
            print(f"❌ OCR识别失败: {response.status_code}")
            print(f"错误信息: {response.text}")

    except requests.exceptions.Timeout:
        print("⏱️ OCR请求超时")
    except Exception as e:
        print(f"❌ OCR请求失败: {e}")


def test_ocr_engines():
    """测试OCR引擎状态"""
    print("\n🔧 检查OCR引擎状态")

    try:
        # 获取可用引擎
        response = requests.get("http://localhost:8080/api/v1/ocr/engines")
        if response.status_code == 200:
            engines = response.json()
            print(f"✅ 可用引擎: {engines.get('engines', [])}")
            print(f"📊 引擎数量: {engines.get('total', 0)}")

        # 获取引擎状态
        response = requests.get("http://localhost:8080/api/v1/ocr/engines/status")
        if response.status_code == 200:
            status = response.json()
            print(
                f"🎯 默认引擎: {status.get('status', {}).get('default_engine', 'unknown')}"
            )

            for engine_name, engine_info in status.get("status", {}).items():
                if engine_name != "default_engine" and isinstance(engine_info, dict):
                    print(f"  📋 {engine_name}: {engine_info.get('name', 'unknown')}")
                    print(
                        f"     可用: {'✅' if engine_info.get('available') else '❌'}"
                    )
                    langs = engine_info.get("supported_languages", [])
                    print(f"     支持语言: {', '.join(langs)}")

    except Exception as e:
        print(f"❌ 获取引擎状态失败: {e}")


if __name__ == "__main__":
    print("🚀 开始OCR功能测试")
    print("=" * 50)

    test_ocr_engines()
    test_ocr_api()

    print("\n" + "=" * 50)
    print("🎉 OCR测试完成!")

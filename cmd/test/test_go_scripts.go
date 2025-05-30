package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"mq_adb/pkg/scripts"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "test-go-scripts",
		Short: "Test Go Scripts System",
		Long:  "测试Go脚本系统功能",
		Run:   runTest,
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("命令执行失败: %v", err)
	}
}

func runTest(cmd *cobra.Command, args []string) {
	fmt.Println("=== Go脚本系统测试 ===")
	fmt.Println("这是一个独立测试，不需要MQTT连接")

	// 测试脚本注册表
	fmt.Println("\n📋 测试脚本注册表...")
	registry := scripts.GlobalRegistry
	availableScripts := registry.List()
	fmt.Printf("✅ 找到 %d 个脚本: %v\n", len(availableScripts), availableScripts)

	// 显示脚本信息
	fmt.Println("\n📝 脚本详细信息:")
	scriptInfo := registry.GetScriptInfo()
	for _, info := range scriptInfo {
		fmt.Printf("  - %s: %s\n", info.Name, info.Description)
	}

	// 测试脚本执行
	fmt.Println("\n🧪 测试脚本执行...")
	testScriptExecution()

	// 交互式测试
	fmt.Println("\n🎮 进入交互式测试模式")
	fmt.Println("可用命令: list, info, test <script_name>, quit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\ntest> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]

		switch command {
		case "list":
			fmt.Printf("可用脚本: %v\n", availableScripts)

		case "info":
			for _, info := range scriptInfo {
				fmt.Printf("\n🔧 %s\n", info.Name)
				fmt.Printf("   描述: %s\n", info.Description)
				if len(info.Parameters) > 0 {
					fmt.Printf("   参数:\n")
					for param, desc := range info.Parameters {
						fmt.Printf("     - %s: %v\n", param, desc)
					}
				}
			}

		case "test":
			if len(parts) < 2 {
				fmt.Println("用法: test <script_name>")
				continue
			}
			testSingleScript(parts[1])

		case "quit", "exit":
			fmt.Println("测试结束！")
			return

		default:
			fmt.Printf("未知命令: %s\n", command)
		}
	}
}

func testScriptExecution() {
	// 创建Mock客户端和日志
	logger := &scripts.DefaultLogger{}
	mockClient := scripts.NewMockScriptClient(logger)

	// 创建测试上下文
	ctx := scripts.NewScriptContext("TEST_DEVICE", "test_execution", map[string]interface{}{
		"username": "test_user",
		"password": "test_pass",
	}, mockClient, logger)

	registry := scripts.GlobalRegistry

	// 测试截图脚本
	fmt.Println("  📸 测试截图脚本...")
	result, err := registry.Execute("screenshot", ctx, nil)
	if err != nil {
		fmt.Printf("    ❌ 失败: %v\n", err)
	} else {
		fmt.Printf("    ✅ 成功: %s\n", result.Message)
	}

	// 测试查找点击脚本
	fmt.Println("  🔍 测试查找点击脚本...")
	result, err = registry.Execute("find_and_click", ctx, map[string]interface{}{
		"text": "登录",
	})
	if err != nil {
		fmt.Printf("    ❌ 失败: %v\n", err)
	} else {
		if result.Success {
			fmt.Printf("    ✅ 成功: %s\n", result.Message)
		} else {
			fmt.Printf("    ⚠️  完成但未成功: %s\n", result.Message)
		}
	}

	// 测试等待脚本
	fmt.Println("  ⏱️  测试等待脚本...")
	result, err = registry.Execute("wait", ctx, map[string]interface{}{
		"seconds": 1,
	})
	if err != nil {
		fmt.Printf("    ❌ 失败: %v\n", err)
	} else {
		fmt.Printf("    ✅ 成功: %s\n", result.Message)
	}
}

func testSingleScript(scriptName string) {
	registry := scripts.GlobalRegistry

	// 检查脚本是否存在
	_, exists := registry.Get(scriptName)
	if !exists {
		fmt.Printf("❌ 脚本 '%s' 不存在\n", scriptName)
		return
	}

	// 创建测试环境
	logger := &scripts.DefaultLogger{}
	mockClient := scripts.NewMockScriptClient(logger)
	ctx := scripts.NewScriptContext("TEST_DEVICE", "test_execution", map[string]interface{}{
		"username": "test_user",
		"password": "test_pass",
	}, mockClient, logger)

	// 根据脚本类型设置不同的测试参数
	var params map[string]interface{}
	switch scriptName {
	case "find_and_click":
		params = map[string]interface{}{"text": "登录"}
	case "login":
		params = map[string]interface{}{"username": "admin", "password": "123456"}
	case "wait":
		params = map[string]interface{}{"seconds": 1}
	case "input_text":
		params = map[string]interface{}{"text": "测试文本"}
	case "check_text":
		params = map[string]interface{}{"text": "登录"}
	case "execute_shell":
		params = map[string]interface{}{"command": "echo hello"}
	case "smart_navigate":
		params = map[string]interface{}{"app_name": "设置"}
	default:
		params = nil
	}

	fmt.Printf("🧪 测试脚本: %s\n", scriptName)
	if params != nil {
		fmt.Printf("   参数: %v\n", params)
	}

	result, err := registry.Execute(scriptName, ctx, params)
	if err != nil {
		fmt.Printf("❌ 执行失败: %v\n", err)
	} else {
		if result.Success {
			fmt.Printf("✅ 执行成功: %s\n", result.Message)
		} else {
			fmt.Printf("⚠️  执行完成但失败: %s\n", result.Message)
			if result.Error != "" {
				fmt.Printf("   错误: %s\n", result.Error)
			}
		}
		fmt.Printf("   耗时: %v\n", result.Duration)

		if result.Data != nil && len(result.Data) > 0 {
			fmt.Printf("   数据: %v\n", result.Data)
		}
	}
}

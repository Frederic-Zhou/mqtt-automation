package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mq_adb/pkg/api"
	"mq_adb/pkg/models"
	"mq_adb/pkg/mqtt"
	"mq_adb/pkg/scripts"

	"github.com/spf13/cobra"
)

var (
	serverMode   bool
	port         string
	interactive  bool
	useGoScripts bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mq-automation",
		Short: "Mobile Automation Server",
		Long:  "基于MQTT的手机自动化脚本执行服务器 - 现在支持Go脚本模式！",
		Run:   runServer,
	}

	rootCmd.Flags().BoolVarP(&serverMode, "server", "s", false, "启动HTTP服务器模式")
	rootCmd.Flags().StringVarP(&port, "port", "p", "8080", "HTTP服务器端口")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "启动交互式模式")
	rootCmd.Flags().BoolVarP(&useGoScripts, "go-scripts", "g", true, "使用Go脚本模式（默认启用）")

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("命令执行失败: %v", err)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// 创建MQTT客户端
	mqttClient := mqtt.NewClient()
	if err := mqttClient.Connect(); err != nil {
		log.Fatalf("MQTT连接失败: %v", err)
	}
	defer mqttClient.Disconnect()

	if useGoScripts {
		// 使用新的Go脚本引擎
		log.Println("🚀 启动Go脚本模式...")
		runWithGoScripts(mqttClient)
	} else {
		// 使用旧的YAML脚本引擎
		log.Println("⚠️  使用传统YAML脚本模式...")
		runWithYAMLScripts(mqttClient)
	}
}

func runWithGoScripts(mqttClient *mqtt.Client) {
	// 创建Go脚本引擎
	scriptEngine := scripts.NewGoScriptEngine(mqttClient)

	// 设置响应处理器
	mqttClient.SetResponseHandler(scriptEngine.HandleResponse)

	// 打印可用脚本
	availableScripts := scriptEngine.ListAvailableScripts()
	log.Printf("✅ Go脚本引擎已启动，可用脚本: %v", availableScripts)

	if serverMode {
		// HTTP服务器模式
		runGoScriptHTTPServer(scriptEngine)
	} else if interactive {
		// 交互式模式
		runGoScriptInteractiveMode(scriptEngine)
	} else {
		// 默认启动HTTP服务器
		runGoScriptHTTPServer(scriptEngine)
	}
}

func runWithYAMLScripts(mqttClient *mqtt.Client) {
	// 这里保持原有的YAML脚本逻辑作为后备
	log.Println("注意：YAML脚本模式已被弃用，建议使用 --go-scripts 模式")

	// 可以在这里调用原有的engine.NewScriptEngine
	// 但我们主要推荐使用Go脚本模式
	log.Println("请使用 --go-scripts 标志启用Go脚本模式")
}

func runGoScriptHTTPServer(scriptEngine *scripts.GoScriptEngine) {
	server := api.NewGoScriptServer(scriptEngine)

	log.Printf("🌐 HTTP服务器启动在端口 %s", port)
	log.Printf("📋 API文档: http://localhost:%s/api/v1/health", port)
	log.Printf("🎨 Web界面: http://localhost:%s/web", port)
	log.Printf("📝 脚本列表: http://localhost:%s/api/v1/scripts", port)
	log.Printf("ℹ️  脚本信息: http://localhost:%s/api/v1/scripts/info", port)

	if err := server.Run(":" + port); err != nil {
		log.Fatalf("HTTP服务器启动失败: %v", err)
	}
}

func runGoScriptInteractiveMode(scriptEngine *scripts.GoScriptEngine) {
	fmt.Println("=== 手机自动化脚本执行器 (Go脚本模式) ===")
	fmt.Println("输入 'help' 查看帮助，输入 'quit' 退出")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n> ")
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
		case "help":
			showGoScriptHelp()

		case "list", "scripts":
			listGoScripts(scriptEngine)

		case "info":
			showGoScriptInfo(scriptEngine)

		case "execute", "exec", "run":
			if len(parts) < 3 {
				fmt.Println("用法: execute <设备ID> <脚本名> [参数...]")
				continue
			}
			executeGoScript(scriptEngine, parts[1], parts[2], parts[3:])

		case "status":
			if len(parts) < 2 {
				fmt.Println("用法: status <执行ID>")
				continue
			}
			showExecutionStatus(scriptEngine, parts[1])

		case "history":
			showExecutionHistory(scriptEngine)

		case "test":
			testGoScripts(scriptEngine)

		case "quit", "exit":
			fmt.Println("再见!")
			return

		default:
			fmt.Printf("未知命令: %s。输入 'help' 查看帮助。\n", command)
		}
	}
}

func showGoScriptHelp() {
	fmt.Println("\n=== 可用命令 ===")
	fmt.Println("help              - 显示此帮助")
	fmt.Println("list/scripts      - 列出所有可用脚本")
	fmt.Println("info              - 显示脚本详细信息")
	fmt.Println("execute <设备ID> <脚本名> [参数...] - 执行脚本")
	fmt.Println("status <执行ID>   - 查看执行状态")
	fmt.Println("history           - 查看执行历史")
	fmt.Println("test              - 测试脚本功能")
	fmt.Println("quit/exit         - 退出程序")
	fmt.Println("\n=== 示例 ===")
	fmt.Println("execute 10CDAD18EB0058G find_and_click")
	fmt.Println("execute 10CDAD18EB0058G login username=admin password=123456")
}

func listGoScripts(scriptEngine *scripts.GoScriptEngine) {
	scripts := scriptEngine.ListAvailableScripts()
	fmt.Printf("\n=== 可用脚本 (%d个) ===\n", len(scripts))
	for i, script := range scripts {
		fmt.Printf("%d. %s\n", i+1, script)
	}
}

func showGoScriptInfo(scriptEngine *scripts.GoScriptEngine) {
	scriptInfo := scriptEngine.GetScriptInfo()
	fmt.Printf("\n=== 脚本详细信息 (%d个) ===\n", len(scriptInfo))

	for _, info := range scriptInfo {
		fmt.Printf("\n📝 %s\n", info.Name)
		fmt.Printf("   描述: %s\n", info.Description)
		if len(info.Parameters) > 0 {
			fmt.Printf("   参数:\n")
			for param, desc := range info.Parameters {
				fmt.Printf("     - %s: %v\n", param, desc)
			}
		}
	}
}

func executeGoScript(scriptEngine *scripts.GoScriptEngine, deviceID, scriptName string, params []string) {
	// 解析参数
	variables := make(map[string]interface{})
	for _, param := range params {
		if strings.Contains(param, "=") {
			parts := strings.SplitN(param, "=", 2)
			variables[parts[0]] = parts[1]
		}
	}

	request := &models.ScriptRequest{
		DeviceID:   deviceID,
		ScriptName: scriptName,
		Variables:  variables,
	}

	fmt.Printf("⚡ 执行脚本: %s (设备: %s)\n", scriptName, deviceID)
	if len(variables) > 0 {
		fmt.Printf("   参数: %v\n", variables)
	}

	response, err := scriptEngine.ExecuteScript(request)
	if err != nil {
		fmt.Printf("❌ 执行失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 脚本已启动\n")
	fmt.Printf("   执行ID: %s\n", response.ExecutionID)
	fmt.Printf("   状态: %s\n", response.Status)
	fmt.Printf("   开始时间: %s\n", response.StartTime.Format("2006-01-02 15:04:05"))

	// 等待执行完成
	fmt.Printf("⏳ 等待执行完成...\n")
	for {
		time.Sleep(2 * time.Second)
		execution, err := scriptEngine.GetExecutionStatus(response.ExecutionID)
		if err != nil {
			fmt.Printf("❌ 获取状态失败: %v\n", err)
			break
		}

		fmt.Printf("   当前状态: %s\n", execution.Status)

		if execution.Status != "running" {
			if execution.Result != nil {
				if execution.Result.Success {
					fmt.Printf("✅ 执行成功: %s\n", execution.Result.Message)
				} else {
					fmt.Printf("❌ 执行失败: %s\n", execution.Result.Message)
					if execution.Result.Error != "" {
						fmt.Printf("   错误: %s\n", execution.Result.Error)
					}
				}
				fmt.Printf("   耗时: %v\n", execution.Result.Duration)
			}
			break
		}
	}
}

func showExecutionStatus(scriptEngine *scripts.GoScriptEngine, executionID string) {
	execution, err := scriptEngine.GetExecutionStatus(executionID)
	if err != nil {
		fmt.Printf("❌ 获取执行状态失败: %v\n", err)
		return
	}

	fmt.Printf("\n=== 执行状态 ===\n")
	fmt.Printf("ID: %s\n", execution.ID)
	fmt.Printf("脚本: %s\n", execution.ScriptName)
	fmt.Printf("设备: %s\n", execution.DeviceID)
	fmt.Printf("状态: %s\n", execution.Status)
	fmt.Printf("开始时间: %s\n", execution.StartTime.Format("2006-01-02 15:04:05"))

	if execution.EndTime != nil {
		fmt.Printf("结束时间: %s\n", execution.EndTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("总耗时: %v\n", execution.EndTime.Sub(execution.StartTime))
	}

	if execution.Result != nil {
		fmt.Printf("结果: %s\n", execution.Result.Message)
		if execution.Result.Error != "" {
			fmt.Printf("错误: %s\n", execution.Result.Error)
		}
	}
}

func showExecutionHistory(scriptEngine *scripts.GoScriptEngine) {
	history := scriptEngine.GetExecutionHistory(10)
	fmt.Printf("\n=== 执行历史 (最近10条) ===\n")

	if len(history) == 0 {
		fmt.Println("暂无执行历史")
		return
	}

	for i, execution := range history {
		status := "🔄"
		switch execution.Status {
		case "completed":
			if execution.Result != nil && execution.Result.Success {
				status = "✅"
			} else {
				status = "❌"
			}
		case "failed":
			status = "❌"
		case "cancelled":
			status = "⏹️"
		}

		fmt.Printf("%d. %s %s - %s (%s)\n",
			i+1, status, execution.ScriptName, execution.DeviceID,
			execution.StartTime.Format("01-02 15:04"))
	}
}

func testGoScripts(scriptEngine *scripts.GoScriptEngine) {
	fmt.Println("\n=== 测试Go脚本功能 ===")

	// 使用Mock客户端测试脚本
	logger := &scripts.DefaultLogger{}
	mockClient := scripts.NewMockScriptClient(logger)

	// 创建测试上下文
	ctx := scripts.NewScriptContext("TEST_DEVICE", "test_execution", map[string]interface{}{
		"username": "test_user",
		"password": "test_pass",
	}, mockClient, logger)

	// 测试截图脚本
	fmt.Println("📸 测试截图脚本...")
	result, err := scripts.GlobalRegistry.Execute("screenshot", ctx, nil)
	if err != nil {
		fmt.Printf("❌ 截图测试失败: %v\n", err)
	} else {
		fmt.Printf("✅ 截图测试成功: %s\n", result.Message)
	}

	// 测试查找点击脚本
	fmt.Println("🔍 测试查找点击脚本...")
	result, err = scripts.GlobalRegistry.Execute("find_and_click", ctx, map[string]interface{}{
		"text": "登录",
	})
	if err != nil {
		fmt.Printf("❌ 查找点击测试失败: %v\n", err)
	} else {
		if result.Success {
			fmt.Printf("✅ 查找点击测试成功: %s\n", result.Message)
		} else {
			fmt.Printf("⚠️  查找点击测试完成但未成功: %s\n", result.Message)
		}
	}

	fmt.Println("🎉 测试完成！")
}

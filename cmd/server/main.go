package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"mq_adb/pkg/api"
	"mq_adb/pkg/engine"
	"mq_adb/pkg/models"
	"mq_adb/pkg/mqtt"

	"github.com/spf13/cobra"
)

var (
	serverMode  bool
	port        string
	interactive bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mq-automation",
		Short: "Mobile Automation Server",
		Long:  "基于MQTT的手机自动化脚本执行服务器",
		Run:   runServer,
	}

	rootCmd.Flags().BoolVarP(&serverMode, "server", "s", false, "启动HTTP服务器模式")
	rootCmd.Flags().StringVarP(&port, "port", "p", "8080", "HTTP服务器端口")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "启动交互式模式")

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

	// 创建脚本引擎
	scriptEngine := engine.NewScriptEngine(mqttClient)

	// 设置响应处理器
	mqttClient.SetResponseHandler(scriptEngine.HandleResponse)

	if serverMode {
		// HTTP服务器模式
		runHTTPServer(scriptEngine)
	} else if interactive {
		// 交互式模式
		runInteractiveMode(scriptEngine)
	} else {
		// 默认启动HTTP服务器
		runHTTPServer(scriptEngine)
	}
}

func runHTTPServer(scriptEngine *engine.ScriptEngine) {
	server := api.NewServer(scriptEngine)

	log.Printf("HTTP服务器启动在端口 %s", port)
	log.Printf("API文档: http://localhost:%s/api/v1/health", port)
	log.Printf("Web界面: http://localhost:%s/web", port)

	if err := server.Run(":" + port); err != nil {
		log.Fatalf("HTTP服务器启动失败: %v", err)
	}
}

func runInteractiveMode(scriptEngine *engine.ScriptEngine) {
	fmt.Println("=== 手机自动化脚本执行器 ===")
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
			showHelp()
		case "quit", "exit":
			fmt.Println("再见！")
			return
		case "list":
			listScripts()
		case "run":
			if len(parts) < 3 {
				fmt.Println("用法: run <script_name> <device_id> [variables...]")
				continue
			}
			runScriptInteractive(scriptEngine, parts[1], parts[2], parts[3:])
		case "status":
			if len(parts) < 2 {
				fmt.Println("用法: status <execution_id>")
				continue
			}
			showExecutionStatus(scriptEngine, parts[1])
		default:
			fmt.Printf("未知命令: %s，输入 'help' 查看帮助\n", command)
		}
	}
}

func showHelp() {
	fmt.Print(`
可用命令:
  help              - 显示此帮助信息
  list              - 列出可用脚本
  run <script> <device> [vars] - 执行脚本
  status <execution_id>        - 查看执行状态
  quit/exit         - 退出程序

示例:
  run login_demo device001 username=test password=123
  status exec_123456789
`)
}

func listScripts() {
	fmt.Println("可用脚本:")
	fmt.Println("  - login_demo      : 登录演示脚本")
	fmt.Println("  - app_launch      : 应用启动脚本")
	fmt.Println("  - text_search_demo: 文本搜索演示")
	fmt.Println("  - system_info     : 系统信息收集")
	fmt.Println("  - network_test    : 网络测试")
	fmt.Println("  - auto_login      : 自动登录")
	fmt.Println("  - test_shell      : Shell命令测试")
}

func runScriptInteractive(scriptEngine *engine.ScriptEngine, scriptName, deviceID string, varArgs []string) {
	variables := make(map[string]interface{})
	for _, arg := range varArgs {
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			variables[parts[0]] = parts[1]
		}
	}

	request := &models.ScriptRequest{
		ScriptName: scriptName,
		DeviceID:   deviceID,
		Variables:  variables,
	}

	response, err := scriptEngine.ExecuteScript(request)
	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		return
	}

	fmt.Printf("脚本已启动，执行ID: %s\n", response.ExecutionID)
	fmt.Printf("状态: %s\n", response.Status)

	// 等待一段时间后显示状态
	time.Sleep(2 * time.Second)
	showExecutionStatus(scriptEngine, response.ExecutionID)
}

func showExecutionStatus(scriptEngine *engine.ScriptEngine, executionID string) {
	status, err := scriptEngine.GetExecutionStatus(executionID)
	if err != nil {
		fmt.Printf("未找到执行ID: %s, 错误: %v\n", executionID, err)
		return
	}

	fmt.Printf("执行ID: %s\n", executionID)
	fmt.Printf("状态: %s\n", status.Status)
	fmt.Printf("开始时间: %s\n", status.StartTime.Format("2006-01-02 15:04:05"))

	if len(status.Results) > 0 {
		fmt.Printf("已完成步骤: %d\n", len(status.Results))
		for i, result := range status.Results {
			fmt.Printf("  步骤 %d: %s (%s)\n", i+1, result.Command, result.Status)
		}
	}
}

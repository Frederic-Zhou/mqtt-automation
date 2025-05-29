package main

import (
"fmt"
"os"
"gopkg.in/yaml.v3"
"mq_adb/pkg/models"
)

func main() {
	data, err := os.ReadFile("scripts/comprehensive_test_fixed.yaml")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	var script models.Script
	if err := yaml.Unmarshal(data, &script); err != nil {
		fmt.Printf("YAML parse error: %v\n", err)
		return
	}

	fmt.Printf("Script parsed successfully: %s\n", script.Name)
	fmt.Printf("Description: %s\n", script.Description)
	fmt.Printf("Version: %s\n", script.Version)
	fmt.Printf("Steps count: %d\n", len(script.Steps))
}

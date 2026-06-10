package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GoVersion = runtime.Version()
	Author    = "David.xcm@gmail.com"
)

// Node 表示文件树中的一个节点
type Node struct {
	Name     string // 节点名称
	IsDir    bool   // 是否为目录
	Depth    int    // 缩进深度（层级）
	Children []*Node
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Go Version: %s\n", GoVersion)
		fmt.Printf("Author: %s\n", Author)
		fmt.Println("----------------------------------------")
		fmt.Println("Usage: pCreate <project-structure-file>")
		os.Exit(1)
	}

	configPath := os.Args[1]

	// 获取配置文件所在目录
	configDir := filepath.Dir(configPath)
	if !filepath.IsAbs(configDir) {
		absDir, err := filepath.Abs(configDir)
		if err != nil {
			fmt.Printf("Error getting absolute path: %v\n", err)
			os.Exit(1)
		}
		configDir = absDir
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Error: config file '%s' does not exist.\n", configPath)
		os.Exit(1)
	}

	// 一次性读取整个文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	// 解析项目结构
	rootNode, err := parseStructure(string(data))
	if err != nil {
		fmt.Printf("Error parsing structure: %v\n", err)
		os.Exit(1)
	}

	// 在配置文件所在目录创建项目结构
	if err := createStructure(configDir, rootNode, 0); err != nil {
		fmt.Printf("Error creating project structure: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Project structure created successfully in '%s'!\n", configDir)
}

// parseStructure 解析项目结构文本
func parseStructure(content string) (*Node, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty configuration")
	}

	// 使用栈来跟踪目录层级
	type stackItem struct {
		node  *Node
		depth int
	}
	stack := []stackItem{}
	var root *Node

	for lineNum, line := range lines {
		// 保留原始行用于提取注释
		originalLine := line

		// 移除注释部分（但保留注释用于显示）
		comment := ""
		if idx := strings.Index(line, "#"); idx != -1 {
			comment = strings.TrimSpace(line[idx+1:])
			line = line[:idx]
		}

		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 计算缩进深度（基于树状符号和空格）
		depth := calculateDepth(line)

		// 清理路径名称
		cleanName := cleanNodeName(line)
		if cleanName == "" {
			continue
		}

		// 判断是否为目录
		isDir := isDirNode(cleanName, line)

		// 创建节点
		node := &Node{
			Name:  cleanName,
			IsDir: isDir,
			Depth: depth,
		}

		// 如果是第一行（根节点）
		if root == nil {
			root = node
			stack = append(stack, stackItem{node: node, depth: depth})
			continue
		}

		// 根据深度找到父节点
		for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			return nil, fmt.Errorf("line %d: invalid indentation for '%s'", lineNum+1, originalLine)
		}

		parent := stack[len(stack)-1].node
		if !parent.IsDir {
			return nil, fmt.Errorf("line %d: cannot add children to file '%s'", lineNum+1, parent.Name)
		}

		parent.Children = append(parent.Children, node)

		if isDir {
			stack = append(stack, stackItem{node: node, depth: depth})
		}

		// 如果有注释，存储注释信息（可选）
		if comment != "" {
			node.Name = cleanName // 保持节点名称为干净路径
		}
	}

	if root == nil {
		return nil, fmt.Errorf("no valid structure found")
	}

	return root, nil
}

// calculateDepth 计算节点的深度层级
func calculateDepth(line string) int {
	depth := 0
	runes := []rune(line)

	for _, r := range runes {
		switch r {
		case '│', '├', '└', '─', '┬', '┌', '┐', '┘':
			depth++
		case ' ':
			depth++
		case '\t':
			depth += 4 // Tab 算作4个空格
		default:
			// 遇到实际字符就停止计算
			goto depthCalculated
		}
	}

depthCalculated:
	// 粗略计算层级：每4个单位算一级
	return depth / 4
}

// cleanNodeName 清理节点名称（移除树状符号）
func cleanNodeName(line string) string {
	// 移除常见的树状符号
	treeSymbols := []string{
		"├──", "└──", "│──", "├─", "└─", "│─",
		"├", "└", "│", "─", "┬", "┌", "┐", "┘", "│",
	}

	result := strings.TrimSpace(line)

	// 移除树状符号组合
	for _, symbol := range treeSymbols {
		result = strings.ReplaceAll(result, symbol, "")
	}

	// 清理多余的斜杠
	result = strings.Trim(result, "/\\")
	result = strings.TrimSpace(result)

	return result
}

// isDirNode 判断节点是否为目录
func isDirNode(name string, originalLine string) bool {
	// 如果名称以 / 结尾，肯定是目录
	if strings.HasSuffix(name, "/") {
		return true
	}

	// 检查原始行中是否有目录标记
	if strings.HasSuffix(strings.TrimSpace(originalLine), "/") {
		return true
	}

	// 如果名称包含点号，进一步判断
	if strings.Contains(name, ".") {
		lastPart := filepath.Base(name)

		// 隐藏目录（如 .git, .vscode）
		commonHiddenDirs := []string{
			".git", ".vscode", ".idea", ".github", ".circleci",
			".env", ".config", ".cache", ".ssh", ".kube",
		}
		for _, dir := range commonHiddenDirs {
			if lastPart == dir {
				return true
			}
		}

		// 如果只有点号开头（如 .gitignore）
		if strings.HasPrefix(lastPart, ".") && !strings.Contains(lastPart[1:], ".") {
			return false // 如 .gitignore
		}

		return false // 一般文件都有扩展名
	}

	// 没有点号的默认为目录
	return true
}

// createStructure 递归创建项目结构
func createStructure(basePath string, node *Node, level int) error {
	fullPath := filepath.Join(basePath, node.Name)

	if node.IsDir {
		// 创建目录
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", fullPath, err)
		}

		// 递归创建子节点
		for _, child := range node.Children {
			if err := createStructure(fullPath, child, level+1); err != nil {
				return err
			}
		}
	} else {
		// 创建文件
		// 确保父目录存在
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory '%s': %w", parentDir, err)
		}

		// 检查文件是否已存在
		if _, err := os.Stat(fullPath); err == nil {
			fmt.Printf("Warning: file '%s' already exists, skipping...\n", fullPath)
			return nil
		}

		// 创建空文件
		if err := os.WriteFile(fullPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create file '%s': %w", fullPath, err)
		}
	}

	return nil
}

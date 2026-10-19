// Package cli 实现 taurus-x 的命令行入口与交互。
//
// 这里只负责「收集输入 + 打印说明」，真正的生成逻辑全部在 internal/generator，
// 自动化测试直接调用 generator.Generate，不经过交互。
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"

	"github.com/stones-hub/taurus-x/internal/components"
	"github.com/stones-hub/taurus-x/internal/generator"
)

// Version taurus-x 版本号，构建时可用 -ldflags 覆盖。
var Version = "v0.1.0"

// 终端颜色
const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

// Run 解析参数并执行对应子命令，返回进程退出码。
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}

	switch args[0] {
	case "new":
		if len(args) != 2 {
			fmt.Fprintf(os.Stderr, "%s用法: taurus-x new <项目名>%s\n", red, reset)
			return 2
		}
		if err := runNew(args[1]); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Fprintf(os.Stderr, "\n%s已取消，未生成任何文件。%s\n", yellow, reset)
				return 130
			}
			fmt.Fprintf(os.Stderr, "%s✗ %v%s\n", red, err, reset)
			return 1
		}
		return 0
	case "version", "-v", "--version":
		fmt.Printf("taurus-x %s\n", Version)
		return 0
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "%s未知命令: %s%s\n", red, args[0], reset)
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `taurus-x %s —— Go + Vue 后台底座脚手架

用法:
  taurus-x new <项目名>   交互式生成一个新项目
  taurus-x version       输出版本号
  taurus-x help          输出本帮助

项目名只能包含字母、数字、下划线和短横线。
`, Version)
}

// runNew 走完整交互并生成项目。
func runNew(projectName string) error {
	if err := generator.ValidateProjectName(projectName); err != nil {
		return err
	}

	module, err := askModule(projectName)
	if err != nil {
		return err
	}
	outputDir, err := askOutputDir(projectName)
	if err != nil {
		return err
	}
	selected, err := askComponents()
	if err != nil {
		return err
	}
	if err := confirmSummary(projectName, module, outputDir, selected); err != nil {
		return err
	}

	result, err := generator.Generate(generator.Options{
		ProjectName: projectName,
		Module:      module,
		OutputDir:   outputDir,
		Components:  selected,
		RunTidy:     true,
		RunWire:     true,
		Stdout:      os.Stdout,
	})
	if err != nil {
		return err
	}

	printNextSteps(result)
	return nil
}

func askModule(projectName string) (string, error) {
	module := ""
	prompt := &survey.Input{
		Message: "Go module 路径:",
		Default: "github.com/stones-hub/" + projectName,
	}
	if err := survey.AskOne(prompt, &module, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}
	return strings.TrimSpace(module), nil
}

// askOutputDir 输入输出目录，当场校验「不存在或为空目录」。
// 校验不通过时 survey 会提示并要求重新输入，不会拖到组件选择和确认之后才报错。
func askOutputDir(projectName string) (string, error) {
	dir := ""
	prompt := &survey.Input{
		Message: "输出目录:",
		Default: "./" + projectName,
	}
	checkDir := func(ans any) error {
		s, ok := ans.(string)
		if !ok {
			return fmt.Errorf("输出目录必须是字符串")
		}
		return generator.ValidateOutputDir(s)
	}
	if err := survey.AskOne(prompt, &dir,
		survey.WithValidator(survey.Required),
		survey.WithValidator(checkDir),
	); err != nil {
		return "", err
	}
	return strings.TrimSpace(dir), nil
}

// askComponents 组件多选：必选项预先勾上并标注「必选」，取消勾选也会被补回。
func askComponents() ([]string, error) {
	options := make([]string, 0, len(components.All))
	defaults := make([]string, 0, len(components.All))
	labelToName := make(map[string]string, len(components.All))

	for _, c := range components.All {
		label := fmt.Sprintf("%s —— %s", c.Name, c.Description)
		if c.Required {
			label = fmt.Sprintf("%s [必选] —— %s", c.Name, c.Description)
			defaults = append(defaults, label)
		}
		options = append(options, label)
		labelToName[label] = c.Name
	}

	fmt.Printf("%s必选组件（已勾选，取消无效）: %s%s\n", cyan, strings.Join(components.RequiredNames(), ", "), reset)

	var picked []string
	prompt := &survey.MultiSelect{
		Message:  "选择要接入的组件（空格勾选，回车确认）:",
		Options:  options,
		Default:  defaults,
		PageSize: len(options),
	}
	if err := survey.AskOne(prompt, &picked); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(picked))
	for _, label := range picked {
		names = append(names, labelToName[label])
	}
	// 必选组件无论是否被取消都补回：底座依赖它们，缺了生成的项目跑不起来。
	names = append(names, components.RequiredNames()...)
	return names, nil
}

func confirmSummary(projectName, module, outputDir string, selected []string) error {
	abs, err := filepath.Abs(outputDir)
	if err != nil {
		abs = outputDir
	}

	var optional []string
	for _, name := range selected {
		if c, ok := components.Find(name); ok && !c.Required {
			optional = append(optional, name)
		}
	}
	if len(optional) == 0 {
		optional = []string{"（无）"}
	}

	fmt.Printf("\n%s==================== 生成汇总 ====================%s\n", cyan, reset)
	fmt.Printf("  项目名      : %s\n", projectName)
	fmt.Printf("  module      : %s\n", module)
	fmt.Printf("  输出目录    : %s\n", abs)
	fmt.Printf("  必选组件    : %s\n", strings.Join(components.RequiredNames(), ", "))
	fmt.Printf("  可选组件    : %s\n", strings.Join(optional, ", "))
	fmt.Printf("%s=================================================%s\n\n", cyan, reset)

	ok := false
	if err := survey.AskOne(&survey.Confirm{Message: "确认生成?", Default: true}, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("已取消，未生成任何文件")
	}
	return nil
}

func printNextSteps(result *generator.Result) {
	rel := result.OutputDir
	if cwd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(cwd, result.OutputDir); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}

	fmt.Printf("\n%s🎉 项目已生成: %s%s\n", green, result.OutputDir, reset)
	fmt.Printf("\n%s下一步:%s\n", cyan, reset)
	fmt.Printf("  1. 建库并导入初始化 SQL:\n")
	fmt.Printf("       mysql -u<user> -p -e \"CREATE DATABASE IF NOT EXISTS <库名> DEFAULT CHARSET utf8mb4\"\n")
	fmt.Printf("       mysql -u<user> -p <库名> < %s/scripts/sql/init.sql\n", rel)
	fmt.Printf("  2. 配置环境变量（含 JWT 密钥、数据库/Redis 地址）:\n")
	fmt.Printf("       cd %s/backend && cp .env.example .env.local && vi .env.local\n", rel)
	fmt.Printf("  3. 安装前端依赖:\n")
	fmt.Printf("       cd %s/frontend && pnpm install\n", rel)
	fmt.Printf("  4. 本地开发（前端 Vite + 后端 Go）:\n")
	fmt.Printf("       cd %s && make dev\n", rel)
	fmt.Printf("\n%s初始管理员:%s 账号 %s%s%s / 默认密码 %s%s%s —— 登录后请立即修改。\n",
		yellow, reset, green, result.AdminUsername, reset, green, result.AdminPassword, reset)
	fmt.Printf("%s生成项目的 README.md「已知风险」章节列出了底座仍待修复的安全问题，上线前请先阅读。%s\n", yellow, reset)
}

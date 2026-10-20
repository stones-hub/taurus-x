// Package generator 负责把 embed 的项目模板渲染成一个可直接构建的项目。
//
// 对外只暴露 Generate 与 Options：交互层（internal/cli）只负责收集输入，
// 自动化测试可以绕过交互直接调用 Generate 测各种组件组合。
package generator

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/stones-hub/taurus-x/internal/components"
	"github.com/stones-hub/taurus-x/templates"
)

// templateRoot embed FS 里项目模板的根目录。
const templateRoot = "project"

// tmplSuffix 只有这个后缀的文件才走 text/template 渲染，其余原样复制。
// Vue/TS/CSS 文件里的 `{{ }}` 是 Vue 插值，必须避开模板渲染。
const tmplSuffix = ".tmpl"

// wirePkg wire 代码生成器的包路径；用 `go run` 调用，不要求全局安装 wire。
const wirePkg = "github.com/google/wire/cmd/wire"

// projectNamePattern 合法项目名：字母、数字、下划线、短横线。
var projectNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// renameOnOutput 模板里为了不干扰 taurus-x 自身仓库而改名的文件，输出时还原。
var renameOnOutput = map[string]string{
	"gitignore": ".gitignore",
}

// optionalConfigDir 可选组件 → 它在 config/autoload/ 下的配置目录名。
// 未选中的组件，其配置目录整个跳过。
var optionalConfigDir = map[string]string{
	"consul":        "consul",
	"opentelemetry": "otel",
	"tcp":           "tcp",
	"grpc":          "grpc",
	"milvus":        "milvus",
}

// Options 生成一个项目所需的全部输入。
type Options struct {
	// ProjectName 项目名，用于二进制名、app_name、默认库名、容器名、前端标题。
	ProjectName string
	// Module 生成项目的 Go module 路径。
	Module string
	// OutputDir 输出目录；必须不存在或为空目录。
	OutputDir string
	// Components 选中的组件名；必选组件缺失时自动补齐。
	Components []string
	// RunTidy 生成后执行 go mod tidy。
	RunTidy bool
	// RunWire 生成后执行 Wire 代码生成（scanner + wire）。
	RunWire bool
	// Stdout 进度输出；nil 时丢弃。
	Stdout io.Writer
}

// Result 生成结果，供调用方打印下一步说明。
type Result struct {
	// OutputDir 实际写入的绝对路径。
	OutputDir string
	// Components 实际生效的组件名（含必选）。
	Components []string
	// AdminUsername 初始管理员账号。
	AdminUsername string
	// AdminPassword 初始管理员随机密码明文。
	// 只用于在终端提示一次，不写进任何生成文件（见 buildTemplateData）。
	AdminPassword string
}

// templateData 传给 text/template 的渲染上下文。
type templateData struct {
	ProjectName string
	// ProjectNameLower 项目名的小写形式，给 npm 包名用（npm 不接受大写）。
	ProjectNameLower string
	Module           string
	// DBName 默认数据库名：项目名里的短横线换成下划线，避免 MySQL 需要反引号。
	DBName string
	// GoVersion 生成项目 go.mod 的 go 指令版本。
	GoVersion string
	// Has 组件选中情况，模板里用 `{{ if .Has.consul }}` 判断。
	Has map[string]bool
	// Requires 生成 go.mod require 块用的组件列表。
	Requires []components.Component
	// WireVersion google/wire 版本。
	WireVersion string

	// AdminUsername 初始管理员账号。
	AdminUsername string
	// AdminPassword 初始管理员随机密码明文，仅用于计算散列，不渲染进任何模板。
	AdminPassword string
	// AdminPasswordSalt 初始管理员密码 salt。
	AdminPasswordSalt string
	// AdminPasswordHash 初始管理员密码散列，写进 init.sql。
	AdminPasswordHash string
}

// Generate 按 opts 生成项目。
//
// 失败时保留已生成的文件，便于排查，不做自动清理（go mod tidy / wire 失败同样如此）。
func Generate(opts Options) (*Result, error) {
	if err := validate(&opts); err != nil {
		return nil, err
	}

	outDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("解析输出目录失败: %w", err)
	}
	if err := ensureEmptyDir(outDir); err != nil {
		return nil, err
	}

	selected := normalizeComponents(opts.Components)
	data, err := buildTemplateData(opts, selected)
	if err != nil {
		return nil, err
	}

	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	if err := writeTree(outDir, data, selected); err != nil {
		return nil, err
	}
	fmt.Fprintf(out, "✅ 项目文件已生成: %s\n", outDir)

	backendDir := filepath.Join(outDir, "backend")
	if opts.RunTidy {
		fmt.Fprintln(out, "→ 执行 go mod tidy ...")
		if err := runCmd(out, backendDir, "go", "mod", "tidy"); err != nil {
			return nil, fmt.Errorf("go mod tidy 失败（已生成的文件保留在 %s，可手动排查）: %w", outDir, err)
		}
	}
	if opts.RunWire {
		fmt.Fprintln(out, "→ 生成 Wire 代码 ...")
		if err := runWire(out, backendDir); err != nil {
			return nil, fmt.Errorf("Wire 代码生成失败（已生成的文件保留在 %s，可手动排查）: %w", outDir, err)
		}
	}

	return &Result{
		OutputDir:     outDir,
		Components:    selected,
		AdminUsername: data.AdminUsername,
		AdminPassword: data.AdminPassword,
	}, nil
}

// ValidateProjectName 校验项目名，交互层可以先用它做即时校验。
func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("项目名不能为空")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("项目名不能包含路径分隔符: %q", name)
	}
	if !projectNamePattern.MatchString(name) {
		return fmt.Errorf("项目名只能包含字母、数字、下划线和短横线: %q", name)
	}
	return nil
}

// ValidateOutputDir 校验输出目录可用：不存在，或存在但为空目录。
// 交互层用它在「输入输出目录」这一步就给出提示，不用等到最后生成时才报错。
// Generate 内部仍会再校验一次 —— 交互与生成之间目录随时可能被别的进程创建。
func ValidateOutputDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("输出目录不能为空")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("解析输出目录失败: %w", err)
	}
	return ensureEmptyDir(abs)
}

func validate(opts *Options) error {
	if err := ValidateProjectName(opts.ProjectName); err != nil {
		return err
	}
	opts.Module = strings.TrimSpace(opts.Module)
	if opts.Module == "" {
		return fmt.Errorf("module 路径不能为空")
	}
	if strings.ContainsAny(opts.Module, " \t\"'`") {
		return fmt.Errorf("module 路径非法: %q", opts.Module)
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("输出目录不能为空")
	}
	for _, name := range opts.Components {
		if _, ok := components.Find(name); !ok {
			return fmt.Errorf("未知组件: %q", name)
		}
	}
	return nil
}

// ensureEmptyDir 输出目录已存在且非空时报错退出，不覆盖已有内容。
func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查输出目录失败: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("输出目录已存在且非空，拒绝覆盖: %s", dir)
	}
	return nil
}

// normalizeComponents 去重、补齐必选组件，并按注册表顺序排列。
func normalizeComponents(chosen []string) []string {
	want := make(map[string]bool, len(chosen))
	for _, name := range chosen {
		want[name] = true
	}
	result := make([]string, 0, len(components.All))
	for _, c := range components.All {
		if c.Required || want[c.Name] {
			result = append(result, c.Name)
		}
	}
	return result
}

func buildTemplateData(opts Options, selected []string) (*templateData, error) {
	has := make(map[string]bool, len(components.All))
	for _, name := range components.AllNames() {
		has[name] = false
	}
	requires := make([]components.Component, 0, len(selected))
	for _, name := range selected {
		has[name] = true
		c, _ := components.Find(name)
		requires = append(requires, c)
	}

	// salt 16 字节 → 32 位 hex，与生成项目 helper.GenerateSalt 的长度一致。
	salt, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	// 初始管理员密码每次生成都不同：固定密码一旦写进文档/仓库，
	// 等于所有用 taurus-x 生成的项目共用一个已知口令。
	const adminUsername = "admin"
	adminPassword, err := randomPassword(16)
	if err != nil {
		return nil, err
	}

	return &templateData{
		ProjectName:       opts.ProjectName,
		ProjectNameLower:  npmName(opts.ProjectName),
		Module:            opts.Module,
		DBName:            strings.ReplaceAll(opts.ProjectName, "-", "_"),
		GoVersion:         "1.25.1",
		Has:               has,
		Requires:          requires,
		WireVersion:       "v0.6.0",
		AdminUsername:     adminUsername,
		AdminPassword:     adminPassword,
		AdminPasswordSalt: salt,
		AdminPasswordHash: passwordHash(adminPassword, salt),
	}, nil
}

// writeTree 遍历 embed 模板树写出项目文件。
func writeTree(outDir string, data *templateData, selected []string) error {
	skipDirs := skippedConfigDirs(selected)

	return fs.WalkDir(templates.FS, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(templateRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDirs[relSlash] {
				return fs.SkipDir
			}
			return os.MkdirAll(filepath.Join(outDir, rel), 0o755)
		}

		target := filepath.Join(outDir, filepath.Dir(rel), outputName(filepath.Base(rel)))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		content, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, tmplSuffix) {
			rendered, rerr := render(relSlash, content, data)
			if rerr != nil {
				return rerr
			}
			content = rendered
		}
		return os.WriteFile(target, content, filePerm(target))
	})
}

// skippedConfigDirs 未选中的可选组件对应的配置目录（相对模板根，slash 分隔）。
func skippedConfigDirs(selected []string) map[string]bool {
	chosen := make(map[string]bool, len(selected))
	for _, name := range selected {
		chosen[name] = true
	}
	skip := make(map[string]bool)
	for name, dir := range optionalConfigDir {
		if !chosen[name] {
			skip["backend/config/autoload/"+dir] = true
		}
	}
	return skip
}

// outputName 去掉 .tmpl 后缀并还原被改名的文件。
func outputName(base string) string {
	base = strings.TrimSuffix(base, tmplSuffix)
	if real, ok := renameOnOutput[base]; ok {
		return real
	}
	return base
}

// filePerm 脚本类文件给可执行位，其余 0644。
func filePerm(target string) os.FileMode {
	if strings.HasSuffix(target, ".sh") {
		return 0o755
	}
	return 0o644
}

func render(name string, content []byte, data *templateData) ([]byte, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("解析模板 %s 失败: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("渲染模板 %s 失败: %w", name, err)
	}
	return buf.Bytes(), nil
}

// runWire 生成依赖注入代码：先扫描 app/ 生成 app/wire.go 并由 wire 产出 wire_gen.go，
// 再对 internal/taurus 执行 wire。两步都用 `go run` 调用 wire，不依赖全局安装。
func runWire(out io.Writer, backendDir string) error {
	if err := runCmd(out, backendDir, "go", "run", "./internal/wire/gen_wire.go", "-project-root", backendDir); err != nil {
		return err
	}
	return runCmd(out, backendDir, "go", "run", "-mod=mod", wirePkg, "./internal/taurus")
}

func runCmd(out io.Writer, dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(out, "%s\n", output)
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// npmName 把项目名规整成合法的 npm 包名：npm 不接受大写，也不接受以 `_`、`-`、`.`
// 开头的名字（项目名校验允许 `_foo` 这种），都在这里削掉。
func npmName(projectName string) string {
	name := strings.TrimLeft(strings.ToLower(projectName), "_-.")
	if name == "" {
		name = "app"
	}
	return name
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机值失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

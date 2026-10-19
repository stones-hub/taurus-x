package generator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stones-hub/taurus-x/internal/components"
)

// tmpRoot 测试产物统一放到 /tmp/taurusx-test-*，不落在仓库里。
const tmpRoot = "/tmp"

// 三种组件组合：只要必选、必选 + grpc + opentelemetry、全选。
func TestGenerate_Combinations(t *testing.T) {
	cases := []struct {
		name     string
		dirName  string
		project  string
		optional []string
	}{
		{
			name:     "只选必选组件",
			dirName:  "taurusx-test-required",
			project:  "demo-required",
			optional: nil,
		},
		{
			name:     "必选 + grpc + opentelemetry",
			dirName:  "taurusx-test-partial",
			project:  "demo-partial",
			optional: []string{"grpc", "opentelemetry"},
		},
		{
			name:     "全部组件",
			dirName:  "taurusx-test-all",
			project:  "demo-all",
			optional: components.OptionalNames(),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			outDir := filepath.Join(tmpRoot, tc.dirName)
			if err := os.RemoveAll(outDir); err != nil {
				t.Fatalf("清理输出目录失败: %v", err)
			}

			var log bytes.Buffer
			result, err := Generate(Options{
				ProjectName: tc.project,
				Module:      "example.com/" + tc.project,
				OutputDir:   outDir,
				Components:  tc.optional,
				RunTidy:     true,
				RunWire:     true,
				Stdout:      &log,
			})
			if err != nil {
				t.Fatalf("生成失败: %v\n%s", err, log.String())
			}

			assertComponents(t, result.Components, tc.optional)
			assertNoBusinessLeftovers(t, outDir)
			assertNoCredentials(t, outDir)
			assertUnselectedAbsent(t, outDir, result.Components)
			assertBaseline(t, outDir, tc.project)

			backend := filepath.Join(outDir, "backend")
			runGo(t, backend, "build", "./...")
			runGo(t, backend, "vet", "./...")
		})
	}
}

// assertBaseline 底座必备产物：初始化 SQL 建齐 12 张表、权限数据与初始管理员到位，
// 且项目名/库名已被模板替换（没有残留 {{ }} 占位或源仓库的 admin 字面量）。
func assertBaseline(t *testing.T, outDir, project string) {
	t.Helper()

	sqlPath := filepath.Join(outDir, "scripts", "sql", "init.sql")
	sql := readFile(t, sqlPath)

	for _, table := range []string{
		"admin_users", "admin_user_logins", "admin_user_login_logs",
		"admin_roles", "admin_permissions", "admin_user_roles", "admin_role_permissions",
		"admin_depts", "admin_user_depts",
		"admin_access_logs", "admin_operation_logs", "admin_front_error_logs",
	} {
		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS `"+table+"`") {
			t.Errorf("init.sql 缺少建表语句: %s", table)
		}
	}

	// 初始管理员：super_admin 角色 + username 登录方式 + 32 位 hex 盐 + 64 位 hex 散列
	if !strings.Contains(sql, "'super_admin'") {
		t.Error("init.sql 缺少 super_admin 角色")
	}
	if !strings.Contains(sql, "INSERT INTO `admin_user_logins`") {
		t.Error("init.sql 缺少初始管理员的登录方式记录")
	}

	// 接口权限 path 必须与后端路由对齐，抽几条代表性的
	for _, p := range []string{
		"'/admin/user/list'", "'/admin/role/list'", "'/admin/permission/tree'",
		"'/admin/dept/list'", "'/admin/login-log/list'", "'/admin/log/front-error/list'",
	} {
		if !strings.Contains(sql, p) {
			t.Errorf("init.sql 缺少接口权限 path %s", p)
		}
	}

	// 模板占位必须全部被渲染掉
	for _, f := range []string{
		filepath.Join("scripts", "sql", "init.sql"),
		filepath.Join("docker-compose.yml"),
		filepath.Join("docs", "部署文档.md"),
		"README.md",
		filepath.Join("frontend", ".env"),
		filepath.Join("frontend", "index.html"),
		filepath.Join("frontend", "package.json"),
	} {
		content := readFile(t, filepath.Join(outDir, f))
		if strings.Contains(content, "{{") {
			t.Errorf("%s 里仍有未渲染的模板占位", f)
		}
	}

	// 前端品牌文案与包名跟项目名对齐（.vue 不渲染，标题走 VITE_APP_TITLE）
	if html := readFile(t, filepath.Join(outDir, "frontend", "index.html")); !strings.Contains(html, "<title>"+project+"</title>") {
		t.Errorf("index.html 的 <title> 未使用项目名 %s", project)
	}
	if env := readFile(t, filepath.Join(outDir, "frontend", ".env")); !strings.Contains(env, "VITE_APP_TITLE="+project) {
		t.Errorf("frontend/.env 的 VITE_APP_TITLE 未使用项目名 %s", project)
	}
	if pkg := readFile(t, filepath.Join(outDir, "frontend", "package.json")); !strings.Contains(pkg, `"name": "`+project+`-frontend"`) {
		t.Errorf("package.json 的 name 未使用项目名 %s", project)
	}

	// docker-compose 的容器名/库名应该是项目自己的
	compose := readFile(t, filepath.Join(outDir, "docker-compose.yml"))
	if !strings.Contains(compose, "container_name: "+project+"-mysql") {
		t.Errorf("docker-compose.yml 的 mysql 容器名未按项目名生成")
	}
	if strings.Contains(compose, "admin-mysql") || strings.Contains(compose, "admin_init.sql") {
		t.Errorf("docker-compose.yml 仍残留源仓库的 admin 命名")
	}
}

func TestGenerate_RejectsBadProjectName(t *testing.T) {
	for _, name := range []string{"", "has space", "a/b", "../escape"} {
		if err := ValidateProjectName(name); err == nil {
			t.Errorf("项目名 %q 应被拒绝", name)
		}
	}
}

func TestGenerate_RefusesNonEmptyDir(t *testing.T) {
	outDir := filepath.Join(tmpRoot, "taurusx-test-nonempty")
	if err := os.RemoveAll(outDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outDir) })

	_, err := Generate(Options{
		ProjectName: "demo",
		Module:      "example.com/demo",
		OutputDir:   outDir,
	})
	if err == nil {
		t.Fatal("非空目录应拒绝生成")
	}
	if !strings.Contains(err.Error(), "拒绝覆盖") {
		t.Fatalf("错误信息应说明拒绝覆盖，实际: %v", err)
	}
}

// TestValidateOutputDir 交互层在「输出目录」这一步就要能判断目录可不可用。
func TestValidateOutputDir(t *testing.T) {
	if err := ValidateOutputDir(""); err == nil {
		t.Error("空目录应被拒绝")
	}

	notExist := filepath.Join(tmpRoot, "taurusx-test-validate-dir")
	if err := os.RemoveAll(notExist); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOutputDir(notExist); err != nil {
		t.Errorf("不存在的目录应被接受，实际: %v", err)
	}

	empty := t.TempDir()
	if err := ValidateOutputDir(empty); err != nil {
		t.Errorf("空目录应被接受，实际: %v", err)
	}

	if err := os.WriteFile(filepath.Join(empty, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateOutputDir(empty)
	if err == nil {
		t.Fatal("非空目录应被拒绝")
	}
	if !strings.Contains(err.Error(), "拒绝覆盖") {
		t.Errorf("错误信息应说明拒绝覆盖，实际: %v", err)
	}
}

// TestGenerate_RequiredComponentsAlwaysPresent 取消勾选必选组件也要被补回。
func TestGenerate_RequiredComponentsAlwaysPresent(t *testing.T) {
	got := normalizeComponents([]string{"grpc"})
	for _, want := range components.RequiredNames() {
		if !contains(got, want) {
			t.Errorf("必选组件 %s 应被自动补齐，实际: %v", want, got)
		}
	}
}

// TestPasswordHash 初始密码散列必须与生成项目 app/helper/crypto.go 的算法一致：
// hex(sha256(password + salt))。
func TestPasswordHash(t *testing.T) {
	// 固定输入的期望值由同一算法手算确定，用于防止算法被悄悄改动
	const salt = "0123456789abcdef0123456789abcdef"
	got := passwordHash("Taurus@2026", salt)
	if len(got) != 64 {
		t.Fatalf("散列长度应为 64 位 hex，实际 %d", len(got))
	}
	if got == passwordHash("Taurus@2026", "ffffffffffffffffffffffffffffffff") {
		t.Fatal("不同 salt 应产生不同散列")
	}
}

func assertComponents(t *testing.T, got []string, optional []string) {
	t.Helper()
	for _, want := range components.RequiredNames() {
		if !contains(got, want) {
			t.Errorf("结果应包含必选组件 %s，实际: %v", want, got)
		}
	}
	for _, want := range optional {
		if !contains(got, want) {
			t.Errorf("结果应包含所选组件 %s，实际: %v", want, got)
		}
	}
}

// businessMarkers 底座里不应再出现的业务标识（CRM/订单/财务/资源/任务/地点），
// 以及只服务于业务功能的三方依赖。用能唯一指向业务的词，避免误伤
// （比如不能直接搜 "location"，`time.LoadLocation` 会命中）。
//
// 注意不能放 "NameResolver"：底座不带这个文件，但 docs 里把它作为「外键 ID → 显示名
// 只读富集」的约定位置写进了公共工具速查表，属于合法出现。改用
// assertBusinessFilesAbsent 直接断言文件不存在。
var businessMarkers = []string{
	"excelize",
	"gomail",
	"golang.org/x/image",
	"libreoffice",
	"customer",
	"contract",
	"supplier",
	"purchase",
	"receivable",
	"invoice",
	"companysubject",
	"company_subject",
	"producttype",
	"product_type",
	"resourceinventory",
	"resourcetemplate",
	"resourcetype",
	"orderboard",
	"orderproduct",
	"paymentrecord",
	"demandsolution",
	"locationcontroller",
	"taskcontroller",
}

// businessFiles 只服务于业务的文件，底座里必须不存在（docs 里可以提到它们的约定位置）。
var businessFiles = []string{
	"backend/app/service/name_resolver.go",
	"backend/app/helper/upload.go",
	"frontend/src/utils/navigation.ts",
	"frontend/src/composables/useLocationCascade.ts",
	"frontend/src/components/FileUploader.vue",
}

// credentialFileMarkers 凭证 / 密钥类文件名片段，对小写文件名做子串匹配。
//
// 这是一条**通用规则**，故意不去匹配任何具体的主机、IP、账号或口令：
// 「抽模板时顺手把参考仓库的凭证文件一起复制进来」是真实发生过的事故，用文件名
// 特征兜住一整类文件才有长期效果；而把已知的具体值写进断言，既换一份凭证就失效，
// 本身也等于把凭证抄进了代码库。
var credentialFileMarkers = []string{
	"account",
	"credential",
	"secret",
	"id_rsa",
	".pem",
	".p12",
	".pfx",
	".jks",
	".keystore",
	".htpasswd",
}

// pemPrivateKeyMarkers 私钥 PEM 文件的头部特征，同样是通用规则（不含任何真实密钥）。
// 两段都命中才算，避免误伤讲解 PEM 格式的文档。
var pemPrivateKeyMarkers = []string{"-----BEGIN", "PRIVATE KEY-----"}

func assertNoBusinessLeftovers(t *testing.T, outDir string) {
	t.Helper()

	// 生成项目里不应残留源仓库名
	assertNoMatch(t, outDir, "taurus-admin")

	for _, marker := range businessMarkers {
		assertNoMatch(t, outDir, marker)
	}
	assertBusinessFilesAbsent(t, outDir)
}

// assertBusinessFilesAbsent 断言业务专属文件没有被带进底座。
func assertBusinessFilesAbsent(t *testing.T, outDir string) {
	t.Helper()

	for _, rel := range businessFiles {
		p := filepath.Join(outDir, filepath.FromSlash(rel))
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("底座不应包含业务专属文件 %s", rel)
		}
	}
}

// assertNoCredentials 凭证护栏：生成的项目里不能有凭证 / 密钥类文件，也不能有
// 「本机私有」的 .env*.local（那类文件必须由使用者自己创建），文件内容里也不能
// 夹带私钥 PEM。全部用通用特征判断，不针对任何具体的主机、账号或口令。
func assertNoCredentials(t *testing.T, outDir string) {
	t.Helper()

	err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "node_modules", ".git", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(outDir, path)
		lowerName := strings.ToLower(info.Name())

		for _, m := range credentialFileMarkers {
			if strings.Contains(lowerName, m) {
				t.Errorf("生成的项目里出现疑似凭证文件 %s（文件名含 %q）", rel, m)
			}
		}
		if isLocalEnvFile(lowerName) {
			t.Errorf("生成的项目里出现本机私有环境文件 %s（.env*.local 必须由使用者自己创建）", rel)
		}

		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if containsAll(string(content), pemPrivateKeyMarkers) {
			t.Errorf("生成的项目里出现私钥 PEM 内容: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历生成目录失败: %v", err)
	}
}

// isLocalEnvFile 判断是不是 `.env.local` / `.env.<mode>.local` 这类本机私有环境文件。
func isLocalEnvFile(lowerName string) bool {
	return strings.HasPrefix(lowerName, ".env") && strings.HasSuffix(lowerName, ".local")
}

func containsAll(s string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}

// optionalDeployMarkers 可选组件在 docker-compose.yml 里的特征串（选中才会出现）。
var optionalDeployMarkers = map[string]string{
	"consul":        "CONSUL_ENABLE",
	"opentelemetry": "OTEL_ENABLE",
	"tcp":           "TCP_ENABLE",
	"grpc":          "GRPC_ENABLE",
	"milvus":        "MILVUS_ENABLE",
}

// optionalExposePorts 占独立端口的组件在 Dockerfile 里对应的 EXPOSE 行。
var optionalExposePorts = map[string]string{
	"tcp":  "EXPOSE 8081",
	"grpc": "EXPOSE 50051",
}

// assertUnselectedAbsent 未选中的组件不能出现在 go.mod、config/autoload、
// internal/taurus、docker-compose.yml 和 Dockerfile 里；选中的则必须出现在编排里。
func assertUnselectedAbsent(t *testing.T, outDir string, selected []string) {
	t.Helper()

	goMod := readFile(t, filepath.Join(outDir, "backend", "go.mod"))
	compose := readFile(t, filepath.Join(outDir, "docker-compose.yml"))
	dockerfile := readFile(t, filepath.Join(outDir, "Dockerfile"))

	for _, c := range components.All {
		if c.Required {
			continue
		}
		marker := optionalDeployMarkers[c.Name]
		if marker == "" {
			t.Fatalf("组件 %s 缺少 optionalDeployMarkers 映射", c.Name)
		}

		if contains(selected, c.Name) {
			// 选中的组件必须被编排进去，否则容器里拿不到配置
			if !strings.Contains(compose, marker) {
				t.Errorf("选中的组件 %s 未出现在 docker-compose.yml 中（缺 %s）", c.Name, marker)
			}
			if expose := optionalExposePorts[c.Name]; expose != "" && !strings.Contains(dockerfile, expose) {
				t.Errorf("选中的组件 %s 未在 Dockerfile 暴露端口（缺 %q）", c.Name, expose)
			}
			continue
		}

		// go.mod 不应 require 它
		if strings.Contains(goMod, c.Package) {
			t.Errorf("未选中的组件 %s 出现在 go.mod 中", c.Name)
		}

		// config/autoload 下不应有它的配置目录
		dir := optionalConfigDir[c.Name]
		if dir == "" {
			t.Fatalf("组件 %s 缺少 optionalConfigDir 映射", c.Name)
		}
		if _, err := os.Stat(filepath.Join(outDir, "backend", "config", "autoload", dir)); !os.IsNotExist(err) {
			t.Errorf("未选中的组件 %s 仍留下配置目录 config/autoload/%s", c.Name, dir)
		}

		// internal/taurus 里不应有它的 Provide 函数
		for _, f := range []string{"components.go", "wire.go", "wire_gen.go"} {
			content := readFile(t, filepath.Join(outDir, "backend", "internal", "taurus", f))
			if strings.Contains(content, c.Package) {
				t.Errorf("未选中的组件 %s 出现在 internal/taurus/%s 中", c.Name, f)
			}
		}

		// docker-compose / Dockerfile 里也不应留下它的编排与端口
		if strings.Contains(compose, marker) {
			t.Errorf("未选中的组件 %s 出现在 docker-compose.yml 中（命中 %s）", c.Name, marker)
		}
		if expose := optionalExposePorts[c.Name]; expose != "" && strings.Contains(dockerfile, expose) {
			t.Errorf("未选中的组件 %s 的端口仍在 Dockerfile 里 EXPOSE（命中 %q）", c.Name, expose)
		}
	}
}

// assertNoMatch 在生成的项目里全文（忽略大小写）搜索 needle，命中即失败。
// 跳过 go.sum 和前端依赖目录：前者是依赖校验和，后者不属于生成产物。
func assertNoMatch(t *testing.T, outDir, needle string) {
	t.Helper()

	var hits []string
	err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "node_modules", ".git", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		switch info.Name() {
		case "go.sum", "pnpm-lock.yaml":
			return nil
		}
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if bytes.Contains(bytes.ToLower(content), bytes.ToLower([]byte(needle))) {
			rel, _ := filepath.Rel(outDir, path)
			hits = append(hits, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历生成目录失败: %v", err)
	}
	if len(hits) > 0 {
		t.Errorf("生成的项目里仍出现 %q: %v", needle, hits)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return string(content)
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s 失败: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

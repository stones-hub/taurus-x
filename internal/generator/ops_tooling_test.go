package generator

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// 运维工具（T9 阶段 3）：备份恢复脚本、压测工具、SBOM / 漏洞扫描入口。
//
// 这里分两层：
//   - assertOpsTooling：静态断言，挂在 TestGenerate_Combinations 里，三种组合都跑；
//   - TestDBScripts_*：用假的 mysql / mysqldump 跑真实脚本，只验证控制流（参数、失败处理、
//     拒绝保护、临时文件清理、密码不进 argv），**不是**真库演练 —— 真库演练见生成项目
//     README.md「八、备份与恢复」的人工步骤。

// 固定的工具版本：与模板 Makefile 一致，改版本要两边一起改并重新实测。
const (
	cyclonedxGomodVersion = "v1.12.0"
	cdxgenVersion         = "12.9.0"
	govulncheckVersion    = "v1.9.0"
)

func assertOpsTooling(t *testing.T, outDir string) {
	t.Helper()

	// ---- 备份 / 恢复脚本 ----
	for _, name := range []string{"backup.sh", "restore.sh", "common.sh"} {
		path := filepath.Join(outDir, "scripts", "db", name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("缺少 scripts/db/%s: %v", name, err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("scripts/db/%s 权限应为 0755，实际 %v", name, info.Mode().Perm())
		}
		if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
			t.Errorf("scripts/db/%s 语法错误: %v\n%s", name, err, out)
		}
		src := readFile(t, path)
		for _, bad := range []string{"DROP DATABASE", "DROP SCHEMA", "--password", "MYSQL_PWD", "root123456", "change-me", "eval "} {
			if strings.Contains(src, bad) {
				t.Errorf("scripts/db/%s 不应出现 %q", name, bad)
			}
		}
		// 密码不能以 -p<密码> 的形式进命令行。
		if regexp.MustCompile(`\s-p["$]`).MatchString(src) {
			t.Errorf("scripts/db/%s 不应用 -p 在命令行传密码", name)
		}
	}
	common := readFile(t, filepath.Join(outDir, "scripts", "db", "common.sh"))
	for _, want := range []string{
		`DBOPS_USER=${MYSQL_OPS_USER:-}`,      // 账号无默认值
		`"${MYSQL_OPS_PASSWORD+set}"`,         // 密码必须显式设置
		`umask 077`, `chmod 600 "$DBOPS_CNF"`, // 临时 client 配置只给自己读
		`*[!A-Za-z0-9_]*`,          // 库名白名单
		`unset MYSQL_OPS_PASSWORD`, // 密码移出环境
		`export -n DBOPS_PASSWORD`, // 私有副本不导出
		"set +a",                   // allexport 下也不自动导出
		`SELECT @@hostname, @@port, @@server_uuid`,
		// 连接参数：--defaults-file 第一，其后命令行参数压过任何选项文件（含 ~/.mylogin.cnf）
		"DBOPS_CONN=(\n\t\t--defaults-file=\"$DBOPS_CNF\"\n\t\t--protocol=TCP\n\t\t--host=\"$DBOPS_HOST\"\n\t\t--port=\"$DBOPS_PORT\"\n\t\t--user=\"$DBOPS_USER\"\n\t)",
		`MYSQL_TEST_LOGIN_FILE="$DBOPS_NO_LOGIN_FILE" mysql "${DBOPS_CONN[@]}" --local-infile=0 "$@"`,
		`MYSQL_TEST_LOGIN_FILE="$DBOPS_NO_LOGIN_FILE" mysqldump "${DBOPS_CONN[@]}" "$@"`,
		"/^CREATE DEFINER=/", "/^CREATE (PROCEDURE|FUNCTION|TRIGGER|EVENT|VIEW) /",
	} {
		if !strings.Contains(common, want) {
			t.Errorf("scripts/db/common.sh 应包含 %q", want)
		}
	}
	backup := readFile(t, filepath.Join(outDir, "scripts", "db", "backup.sh"))
	restore := readFile(t, filepath.Join(outDir, "scripts", "db", "restore.sh"))
	for _, want := range []string{"set -euo pipefail", "trap cleanup EXIT", "trap 'on_signal 130' INT", "if ! dbops_mysqldump \\",
		"--single-transaction", "--set-gtid-purged=OFF", "dbops_verify_dump", `ln -- "$partial" "$final"`, "stored_object_markers=",
		`remove_if_ours "$final" "$partial"`, `[ "$1" -ef "$2" ]`, "backup_done=1", "dbops_take_password"} {
		if !strings.Contains(backup, want) {
			t.Errorf("backup.sh 应包含 %q", want)
		}
	}
	for name, src := range map[string]string{"backup.sh": backup, "restore.sh": restore} {
		for _, line := range strings.Split(src, "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "#") || strings.HasPrefix(code, "dbops_require_tools") || strings.Contains(code, "dbops_die") || strings.Contains(code, "dbops_info") {
				continue
			}
			if regexp.MustCompile(`(^|[^_a-z])(mysql|mysqldump) `).MatchString(code) {
				t.Errorf("%s 里的 mysql / mysqldump 调用必须走 dbops_mysql / dbops_mysqldump（统一连接参数）: %s", name, code)
			}
		}
	}
	// 中断清理按 inode 识别本次落下的正式文件，不按名字登记后无条件删除。
	if strings.Contains(backup, "published") {
		t.Error("backup.sh 不应再用「ln 成功后登记文件名」的方式清理：ln 刚成功、还没登记时收到信号会漏删")
	}
	// 空密码也写显式的 password=""，不能按是否为空跳过这一行。
	if !strings.Contains(common, "\t\tprintf 'password=\"%s\"\\n' \"$escaped\"\n\t} >\"$DBOPS_CNF\"") || strings.Contains(common, `if [ -n "$DBOPS_PASSWORD" ]`) {
		t.Error("common.sh 应总是写 password 行（空密码写成 password=\"\"）")
	}
	if strings.Contains(backup, "--databases") {
		t.Error("backup.sh 不应使用 --databases：导出里不能带 CREATE DATABASE / USE，否则无法恢复到独立测试库")
	}
	for _, want := range []string{"set -euo pipefail", "trap dbops_cleanup_config EXIT", "trap 'on_signal 130' INT",
		`dbops_mysql --binary-mode --one-database`, // 禁用客户端命令；--one-database 不是隔离
		"dbops_take_password",
		`if [ "$confirm" != "$target" ]`, "拒绝恢复", "它不是来源认证", "**不是**隔离手段",
		`[ "$database" != "$meta_source_db" ]`, // 含库级对象时只允许同名库
		"dbops_server_identity", "可能已写入部分数据"} {
		if !strings.Contains(restore, want) {
			t.Errorf("restore.sh 应包含 %q", want)
		}
	}
	for _, bad := range []string{"--force", "--overwrite"} {
		if strings.Contains(restore, bad) {
			t.Errorf("restore.sh 不应提供覆盖非空库的开关 %q", bad)
		}
	}

	// ---- 压测工具：独立 module、只用标准库 ----
	ltDir := filepath.Join(outDir, "scripts", "loadtest")
	goMod := readFile(t, filepath.Join(ltDir, "go.mod"))
	if !strings.Contains(goMod, "\nmodule loadtest\n") || !strings.Contains(goMod, "\ngo "+goVersion+"\n") || strings.Contains(goMod, "require") {
		t.Errorf("scripts/loadtest/go.mod 应是只用标准库的独立 module（go %s）:\n%s", goVersion, goMod)
	}
	if _, err := os.Stat(filepath.Join(ltDir, "go.sum")); err == nil {
		t.Error("scripts/loadtest 不应有 go.sum（不引第三方依赖）")
	}
	scDir := filepath.Join(outDir, "scripts", "sbomcheck")
	scMod := readFile(t, filepath.Join(scDir, "go.mod"))
	if !strings.Contains(scMod, "\nmodule sbomcheck\n") || !strings.Contains(scMod, "\ngo "+goVersion+"\n") || strings.Contains(scMod, "require") {
		t.Errorf("scripts/sbomcheck/go.mod 应是只用标准库的独立 module（go %s）:\n%s", goVersion, scMod)
	}
	var ltSrc strings.Builder
	for _, name := range []string{"main.go", "client.go", "report.go"} {
		ltSrc.WriteString(readFile(t, filepath.Join(ltDir, name)))
	}
	src := ltSrc.String()
	for _, want := range []string{`envPassword = "LOADTEST_PASSWORD"`, "isLoopback(u.Hostname())", `"allow-non-loopback"`,
		`req.Header.Set(csrfName, c.csrfToken())`, "ck.HttpOnly", "maxDuration    = 10 * time.Minute", "maxConcurrency = 64",
		"io.LimitReader(resp.Body, c.cfg.MaxBody+1)", "http.ErrUseLastResponse", "rank := (p*n + 99) / 100",
		"if c.cookie(jwtCookieName) == \"\" {", "r.LogoutFailures > 0", "os.Rename(tmp.Name(), path)", "预登录阶段被中断",
		`(res.outcome == outcomeSuccess || c.cookie(jwtCookieName) != "") && !c.logout()`, "cleanup = append(cleanup, c)"} {
		if !strings.Contains(src, want) {
			t.Errorf("压测工具源码应包含 %q", want)
		}
	}
	// 白名单里的每个列表接口：必须真实注册在路由里；从路由找到处理函数，再从处理函数找到它绑定的
	// 请求 DTO（var req dto.X）；「不过滤」条件的每个字段都必须是**这个** DTO 的 json 字段。
	// 路由、处理函数或 DTO 改名后，压测请求会静默变成 404 或被忽略的条件。
	assertLoadtestEndpoints(t, outDir, src)
	for _, bad := range []string{`Header.Set("Authorization"`, "Bearer ", `"password", "`, "InsecureSkipVerify"} {
		if strings.Contains(src, bad) {
			t.Errorf("压测工具源码不应包含 %q", bad)
		}
	}

	// ---- Makefile：固定版本、退出码原样、值经环境传入 ----
	mk := readFile(t, filepath.Join(outDir, "Makefile"))
	for _, want := range []string{
		"CYCLONEDX_GOMOD_VERSION := " + cyclonedxGomodVersion,
		"CDXGEN_VERSION := " + cdxgenVersion,
		"GOVULNCHECK_VERSION := " + govulncheckVersion,
		"SBOM_DIR := artifacts/sbom",
		"go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@$(CYCLONEDX_GOMOD_VERSION)",
		"npm exec --yes @cyclonedx/cdxgen@$(CDXGEN_VERSION) --",
		"-t pnpm --no-install-deps",
		`env -i PATH="$$PATH" HOME="$$HOME"`,
		"cd backend && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...",
		"cd scripts/loadtest && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...",
		"cd scripts/sbomcheck && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...",
		`env -i PATH="$$PATH" HOME="$$HOME" npm_config_ignore_scripts=true`,
		`./scripts/db/backup.sh --database "$$DB"`,
		`./scripts/db/restore.sh --file "$$BACKUP_FILE" --database "$$DB" --confirm "$$CONFIRM"`,
	} {
		if !strings.Contains(mk, want) {
			t.Errorf("Makefile 应包含 %q", want)
		}
	}
	// 每份 SBOM 都必须先校验、再改名成正式文件：校验行在 mv 之前，且紧跟生成步骤。
	for _, part := range []string{"backend", "frontend"} {
		tmp := "$(SBOM_DIR)/." + part + ".cdx.json.tmp"
		check := strings.Index(mk, "cd scripts/sbomcheck && go run . -spec 1.6 ../../"+tmp)
		mv := strings.Index(mk, "mv "+tmp+" $(SBOM_DIR)/"+part+".cdx.json")
		if check < 0 || mv < 0 || check > mv {
			t.Errorf("sbom-%s 应在 mv 成正式文件之前用 sbomcheck 校验", part)
		}
	}
	if strings.Contains(mk, "退出码原样") {
		t.Error("Makefile 不应声称退出码原样返回：经过 go run / make 包装后看不到 govulncheck 的原始退出码")
	}
	for _, want := range []string{"go run 遇到程序非 0 退出时统一返回 1", "make 再统一返回 2", "make 立即停止"} {
		if !strings.Contains(mk, want) {
			t.Errorf("Makefile 应写明退出码的包装方式 %q", want)
		}
	}
	for _, target := range []string{"db-backup:", "db-restore:", "sbom:", "sbom-backend:", "sbom-frontend:", "vuln:"} {
		if !strings.Contains(mk, "\n"+target) {
			t.Errorf("Makefile 缺少目标 %s", target)
		}
	}
	if regexp.MustCompile(`(^|[^p])npm (install|i |ci)`).MatchString(mk) {
		t.Error("Makefile 不应用 npm 安装依赖（包管理器是 pnpm）")
	}
	for _, bad := range []string{"@latest", "|| true", "| grep", "go install", "go get ", "$(DB)", "$(CONFIRM)", "$(BACKUP_FILE)"} {
		if strings.Contains(mk, bad) {
			t.Errorf("Makefile 不应出现 %q", bad)
		}
	}

	// ---- 生成物忽略、文档同步 ----
	gi := readFile(t, filepath.Join(outDir, ".gitignore"))
	for _, want := range []string{"\nbackups/\n", "\nartifacts/\n"} {
		if !strings.Contains(gi, want) {
			t.Errorf(".gitignore 应忽略 %q", strings.TrimSpace(want))
		}
	}
	// 备份恢复的唯一说明在生成 README「八、备份与恢复」（原运维手册第一节迁入）；运维手册只留压测 / SBOM / 漏洞判读。
	readme := readFile(t, filepath.Join(outDir, "README.md"))
	backupDoc := markdownSection(t, readme, "\n## 八、备份与恢复\n")
	for _, want := range []string{"--single-transaction",
		"备份期间不要做 DDL", "非事务表", "目标库非空一律拒绝", "人工删测试库", "登录验证", "不支持 `docker exec`",
		"MYSQL_OPS_PASSWORD",
		// P2：恢复的信任边界与客户端命令
		"--binary-mode", "只恢复本项目 `backup.sh` 生成", "不是来源认证", "**不是隔离手段**", "不要用 root",
		"**只允许恢复到与源库同名的库**", "另一个 MySQL 实例",
		// 权限与事务限制
		"SHOW_ROUTINE", "SET_USER_ID", "SET_ANY_DEFINER", "不需要 `LOCK TABLES`", "undo 历史", "元数据锁",
		// 连接身份
		"protocol=TCP", "server_uuid", "证明不了", "`~/.mylogin.cnf` 会覆盖 `--defaults-file`", "MYSQL_TEST_LOGIN_FILE",
		"--local-infile=0", "不会停下来等你确认", "只在脚本新建时", "kill -- -<PGID>", "没有在真库上确认过"} {
		if !strings.Contains(backupDoc, want) {
			t.Errorf("README「八、备份与恢复」应说明 %q", want)
		}
	}
	// 事先核对身份的命令：一行、显式 TCP / 主机 / 端口 / 账号 / -p、不读 login 文件。
	if !strings.Contains(backupDoc, "\nMYSQL_TEST_LOGIN_FILE=/nonexistent/mylogin.cnf mysql --protocol=TCP -h 127.0.0.1 -P 3306 -u <账号> -p -e \"SELECT @@hostname, @@port, @@server_uuid\"\n") {
		t.Error("README「八、备份与恢复」应给出完整的单行身份核对命令")
	}
	manual := readFile(t, filepath.Join(outDir, "docs", "运维手册.md"))
	for _, want := range []string{"GO-2026-5932", "不等于没有漏洞", "Symbol Results",
		"cyclonedx-npm",
		// SBOM / vuln
		"scripts/sbomcheck", "npm_config_ignore_scripts=true", "传递依赖", "exit status 3", "make 立即停止",
		// govulncheck 结果层级与退出码：JSON 输出有 finding 也返回 0
		"trace 只有 `module` / `version`", "trace 里有 `package`、没有 `function`", "Package / Module 层**不是零风险证明**",
		"**exit 0 不等于没有 finding**",
		// GO-2026-5932：历史离线扫描记录，不是当前结论；有修复元数据但不能盲升伪版本
		"**历史记录**，不是当前结论", "之后没有联网重扫或复核", "漏洞库、公告和依赖版本都会变化",
		`"fixed_version": "v0.58.1-0.20261009181718-2f1f834330b2"`, `{ "module": "golang.org/x/crypto", "version": "v0.58.0" }`,
		"**不是 bcrypt 的漏洞**", "Module 层不等于零风险，也不等于永远不可达", "**伪版本**",
		"**有修复元数据不等于已有\n  可以直接升级的、受支持的正式版本。**", "**不要盲目把 `go.mod` 改成伪版本**",
		"https://pkg.go.dev/vuln/GO-2026-5932", "go list -m -versions golang.org/x/crypto", "与项目其它依赖兼容的版本"} {
		if !strings.Contains(manual, want) {
			t.Errorf("docs/运维手册.md 应说明 %q", want)
		}
	}
	// 运维手册不再重复备份恢复步骤（不出现两份口径），并指回 README
	for _, dup := range []string{"make db-backup", "make db-restore", "MYSQL_OPS_", "--single-transaction"} {
		if strings.Contains(manual, dup) {
			t.Errorf("docs/运维手册.md 不应再重复备份恢复步骤（命中 %q），唯一说明在 README", dup)
		}
	}
	if !strings.Contains(manual, "](../README.md)") {
		t.Error("docs/运维手册.md 应链接回项目 README.md")
	}
	// GO-2026-5932 旧文案：「没有修复版本 / 无法升级 / 不需要处理 / 保持不可达」及未复核的示例输出都已过时。
	for _, bad := range []string{"Fixed in: N/A", "没有修复版本", "无法通过升级", "不需要处理", "保持不可达", "待独立复核",
		"实现时用 govulncheck", "This scan also found 0 vulnerabilities in packages you import and 1"} {
		if strings.Contains(manual, bad) {
			t.Errorf("docs/运维手册.md 不应再有 GO-2026-5932 的过时说法 %q", bad)
		}
	}
	for _, bad := range []string{"退出码原样返回", "忽略导出里切到其它库的语句，只写目标库", "`CONFIRM` 必须逐字等于实际连接的",
		"主机和端口才真正决定连到哪里", "目录权限 `700`，文件权限 `600`", "实测", "2026-11-02"} {
		for name, doc := range map[string]string{"README.md": readme, "docs/运维手册.md": manual} {
			if strings.Contains(doc, bad) {
				t.Errorf("%s 不应再有过头的说法 %q", name, bad)
			}
		}
	}
	ltReadme := readFile(t, filepath.Join(ltDir, "README.md"))
	for _, want := range []string{"故意留空", "nearest-rank", "429 计入错误", "LOADTEST_PASSWORD", "-allow-non-loopback", "没有图形 / 滑块验证码", "不要改代码里的默认值",
		"**预登录同样受登录限流约束**", "**再按一次** Ctrl+C", "**有会话没能登出**", "替换的是链接本身", "失败的那一次也参与清理",
		"登录判为失败但 jar 里已有 `jwt_token` 时"} {
		if !strings.Contains(ltReadme, want) {
			t.Errorf("scripts/loadtest/README.md 应包含 %q", want)
		}
	}
	// 基准表不能预填结果：表头之后每一行的「记录」列都必须是空的。
	_, bench, found := strings.Cut(ltReadme, "## 五、基准记录")
	if !found {
		t.Fatal("scripts/loadtest/README.md 缺少基准记录一节")
	}
	rows := 0
	for _, line := range strings.Split(bench, "\n") {
		cells := strings.Split(strings.TrimSpace(line), "|")
		if len(cells) != 4 || strings.TrimSpace(cells[1]) == "项" || strings.HasPrefix(strings.TrimSpace(cells[1]), "---") {
			continue
		}
		rows++
		if strings.TrimSpace(cells[2]) != "" {
			t.Errorf("基准记录表不应预填结果: %s", line)
		}
	}
	if rows < 8 {
		t.Errorf("基准记录表应列出硬件、配置、限流、样本等记录项，实际 %d 行", rows)
	}
	for _, doc := range []string{"README.md", filepath.Join("docs", "后端开发流程.md")} {
		if !strings.Contains(readFile(t, filepath.Join(outDir, doc)), "运维手册.md") {
			t.Errorf("%s 应链接到 docs/运维手册.md", doc)
		}
	}
}

// assertLoadtestEndpoints 见调用处说明。
func assertLoadtestEndpoints(t *testing.T, outDir, ltSrc string) {
	t.Helper()
	routes := readFile(t, filepath.Join(outDir, "backend", "app", "routes", "routes.go"))
	var controllers, dtos strings.Builder
	for dir, sb := range map[string]*strings.Builder{"controller": &controllers, "dto": &dtos} {
		files, _ := filepath.Glob(filepath.Join(outDir, "backend", "app", dir, "*.go"))
		for _, f := range files {
			if !strings.HasSuffix(f, "_test.go") {
				sb.WriteString(readFile(t, f) + "\n")
			}
		}
	}
	pageQuery := structJSONTags(t, dtos.String(), "PageQuery")

	endpoints := regexp.MustCompile(`\{Path: "(/admin/[a-z/-]+/list)", Filter: map\[string\]any\{([^}]*)\}\}`).FindAllStringSubmatch(ltSrc, -1)
	if len(endpoints) != 7 {
		t.Errorf("压测工具应有 7 个只读列表接口，解析到 %d 个", len(endpoints))
	}
	groups := strings.Split(routes, "AddRouterGroup(")
	for _, ep := range endpoints {
		path := ep[1]
		// 1. 路由：某个路由组的 Prefix + 组内 Path 拼起来等于该路径，取出 Handler。
		handler := ""
		for _, g := range groups {
			m := regexp.MustCompile(`Prefix:\s+"([^"]+)"`).FindStringSubmatch(g)
			if m == nil || !strings.HasPrefix(path, m[1]) {
				continue
			}
			rest := strings.TrimPrefix(path, m[1])
			if h := regexp.MustCompile(`\{Path: "` + regexp.QuoteMeta(rest) + `", Handler: http\.HandlerFunc\(app\.Core\.(\w+\.\w+)\)\}`).FindStringSubmatch(g); h != nil {
				handler = h[1]
			}
		}
		if handler == "" {
			t.Errorf("压测列表接口 %s 没有在 routes.go 注册", path)
			continue
		}
		// 2. 处理函数绑定的 DTO（funcBody 见 generator_test.go）。
		recv, method, _ := strings.Cut(handler, ".")
		body := funcBody(controllers.String(), `func (c *`+recv+`) `+method+`(`)
		m := regexp.MustCompile(`var req dto\.(\w+)`).FindStringSubmatch(body)
		if m == nil {
			t.Errorf("找不到 %s 的处理函数 %s 或它绑定的请求 DTO", path, handler)
			continue
		}
		// 3. 条件字段必须属于这个 DTO（含嵌入的 PageQuery）。
		tags := structJSONTags(t, dtos.String(), m[1])
		if strings.Contains(structBody(dtos.String(), m[1]), "\tPageQuery\n") {
			for k := range pageQuery {
				tags[k] = true
			}
		}
		for _, key := range regexp.MustCompile(`"([a-z_]+)":`).FindAllStringSubmatch(ep[2], -1) {
			if !tags[key[1]] {
				t.Errorf("压测列表接口 %s 的条件字段 %q 不是 dto.%s 的字段（%s 绑定的 DTO）", path, key[1], m[1], handler)
			}
		}
		if !tags["page_no"] || !tags["page_size"] {
			t.Errorf("dto.%s 应带分页字段 page_no / page_size", m[1])
		}
	}
}

// structBody 取 Go 源码里 type name struct { ... } 的结构体主体。
func structBody(src, name string) string {
	_, after, ok := strings.Cut(src, "type "+name+" struct {\n")
	if !ok {
		return ""
	}
	body, _, _ := strings.Cut(after, "\n}\n")
	return body
}

func structJSONTags(t *testing.T, src, name string) map[string]bool {
	t.Helper()
	body := structBody(src, name)
	if body == "" {
		t.Errorf("找不到 dto.%s 的定义", name)
	}
	tags := map[string]bool{}
	for _, m := range regexp.MustCompile(`json:"([a-z_]+)"`).FindAllStringSubmatch(body, -1) {
		tags[m[1]] = true
	}
	return tags
}

// ---------------------------- 假 mysql / mysqldump 控制流测试 ----------------------------

// （夹具里的标识符不带反引号：这两段是 Go 原始字符串。真实导出带反引号，标记规则只看行首，不受影响。）
// 「停住等中断」的写法：先装 INT/TERM trap，后台起 sleep（fd 都重定向到 /dev/null）并记下 PID，最后才写 started
// 标记，然后 wait。测试看到标记才发 SIGINT，那时 trap 和 PID 都已就绪：信号在任何时刻到达都会被 trap 接住，
// 杀掉 sleep 后退出（wait 会被已 trap 的信号立即打断）。不能写成「touch 标记; sleep 30」：
// 信号可能正好赶上 touch 正常退出，或赶在 bash fork 出 sleep 之前被 bash 自己吞掉，sleep 就会照常睡满并占住输出管道。
// 两个假命令都会检查自己的环境里有没有密码（按变量名，也按密码值），并逐次记录完整 argv
// （每次调用以 --END-- 结尾）和 MYSQL_TEST_LOGIN_FILE 的值、该路径当时是否存在。
// stat 先试 GNU 的 -c，再试 BSD 的 -f：GNU stat 的 -f 是「文件系统信息」模式，可能退出 0 但输出不对。
const fakeMysqldump = `#!/usr/bin/env bash
set -u
if [ "${1:-}" = "--version" ]; then
	echo "${FAKE_VERSION:-mysqldump  Ver 8.0.40 for Linux on x86_64 (MySQL Community Server - GPL)}"
	exit 0
fi
if [ -n "${MYSQL_OPS_PASSWORD+set}" ] || env | grep -q -F 'w;rd'; then echo mysqldump >>"$FAKE_DIR/env.leak"; fi
{ printf '%s\n' "$@"; printf -- '--END--\n'; } >>"$FAKE_DIR/mysqldump.argv"
printf '%s|%s\n' "${MYSQL_TEST_LOGIN_FILE-<unset>}" "$([ -e "${MYSQL_TEST_LOGIN_FILE:-/nonexistent}" ] && echo exists || echo absent)" >>"$FAKE_DIR/loginfile.log"
cnf=${1#--defaults-file=}
cat "$cnf" >"$FAKE_DIR/mysqldump.cnf"
(stat -c %a "$cnf" 2>/dev/null || stat -f %Lp "$cnf") >"$FAKE_DIR/mysqldump.cnf.mode"
case ${FAKE_DUMP:-ok} in
ok) printf -- '-- MySQL dump\nCREATE TABLE t (id int);\n-- Dump completed on 2026-11-02 12:00:00\n' ;;
stored)
	printf -- '-- MySQL dump\nCREATE TABLE t (id int);\n'
	printf -- 'ALTER DATABASE ops_demo CHARACTER SET latin1 ;\n'
	printf -- '/*!50003 CREATE*/ /*!50020 DEFINER=x@y*/ /*!50003 PROCEDURE p() BEGIN END */;;\n'
	printf -- '-- Dump completed on 2026-11-02 12:00:00\n'
	;;
fail)
	printf -- '-- MySQL dump\nCREATE TABLE'
	echo 'mysqldump: Got error: 2013: Lost connection' >&2
	exit 2
	;;
bare_procedure)
	printf -- '-- MySQL dump\nCREATE TABLE t (id int);\nDELIMITER ;;\n'
	printf -- 'CREATE DEFINER=u@localhost PROCEDURE p()\nBEGIN\n  INSERT INTO ops_demo.t VALUES (1);\nEND ;;\n'
	printf -- 'DELIMITER ;\n-- Dump completed on 2026-11-02 12:00:00\n'
	;;
bare_function)
	printf -- '-- MySQL dump\nCREATE TABLE t (id int);\nDELIMITER ;;\n'
	printf -- 'CREATE FUNCTION f() RETURNS int\n    DETERMINISTIC\nRETURN 1 ;;\n'
	printf -- 'DELIMITER ;\n-- Dump completed on 2026-11-02 12:00:00\n'
	;;
truncated) printf -- '-- MySQL dump\nCREATE TABLE t (id int);\n' ;;
sleep)
	printf -- '-- MySQL dump\n'
	trap 'kill "$pause_pid" 2>/dev/null; exit 130' INT TERM
	sleep 30 </dev/null >/dev/null 2>&1 &
	pause_pid=$!
	: >"$FAKE_DIR/started"
	wait "$pause_pid"
	;;
esac
`

const fakeMysql = `#!/usr/bin/env bash
set -u
if [ "${1:-}" = "--version" ]; then
	echo "${FAKE_VERSION:-mysql  Ver 8.0.40 for Linux on x86_64 (MySQL Community Server - GPL)}"
	exit 0
fi
if [ -n "${MYSQL_OPS_PASSWORD+set}" ] || env | grep -q -F 'w;rd'; then echo mysql >>"$FAKE_DIR/env.leak"; fi
{ printf '%s\n' "$@"; printf -- '--END--\n'; } >>"$FAKE_DIR/mysql.argv"
printf '%s|%s\n' "${MYSQL_TEST_LOGIN_FILE-<unset>}" "$([ -e "${MYSQL_TEST_LOGIN_FILE:-/nonexistent}" ] && echo exists || echo absent)" >>"$FAKE_DIR/loginfile.log"
cnf=${1#--defaults-file=}
cat "$cnf" >"$FAKE_DIR/mysql.cnf"
query=""
prev=""
for a in "$@"; do
	if [ "$prev" = "-e" ]; then query=$a; fi
	prev=$a
done
if [ -n "$query" ]; then
	printf '%s\n----\n' "$query" >>"$FAKE_DIR/mysql.queries"
	case $query in
	"/* dbops:identity */"*) printf 'fakehost\t3306\t%s\n' "${FAKE_UUID:-11111111-2222-3333-4444-555555555555}" ;;
	"/* dbops:exists */"*) echo "${FAKE_EXISTS:-1}" ;;
	"/* dbops:objects */"*) echo "${FAKE_OBJECTS:-0}" ;;
	"/* dbops:create */"*) ;;
	"/* dbops:tables */"*) echo 1 ;;
	*) echo "未知查询" >&2; exit 9 ;;
	esac
	exit 0
fi
cat >"$FAKE_DIR/imported.sql"
if [ "${FAKE_IMPORT_SLEEP:-}" = 1 ]; then
	trap 'kill "$pause_pid" 2>/dev/null; exit 130' INT TERM
	sleep 30 </dev/null >/dev/null 2>&1 &
	pause_pid=$!
	: >"$FAKE_DIR/started"
	wait "$pause_pid"
fi
exit "${FAKE_IMPORT_EXIT:-0}"
`

// wrappedTools 脚本会调用的外部命令。测试给每个都套一层透明包装：先检查自己的环境里有没有密码
// （按变量名，也按密码值），再 exec 真正的命令。脚本必须在调用任何外部命令之前就把密码移出环境。
// ln 的包装另外支持停住，用来测试「落正式文件名」阶段被中断：FAKE_LN_SLEEP_AT=k 在第 k 次调用的真实 ln
// **之前**停住；FAKE_LN_SLEEP_AFTER=k 先执行真实 ln（正式文件已经落下），**之后**再停住。
var wrappedTools = []string{"gzip", "awk", "sha256sum", "shasum", "ln", "mktemp", "date", "wc", "tr", "basename", "sed", "chmod", "mkdir", "rm", "dirname"}

const toolWrapper = `#!/bin/bash
if [ -n "${MYSQL_OPS_PASSWORD+set}" ] || [ -n "${DBOPS_PASSWORD+set}" ] || env | grep -q -F 'w;rd'; then echo "%[1]s" >>"$FAKE_DIR/env.leak"; fi
if [ "%[1]s" = ln ] && [ -n "${FAKE_LN_SLEEP_AT:-}${FAKE_LN_SLEEP_AFTER:-}" ]; then
	n=$(( $(cat "$FAKE_DIR/ln.count" 2>/dev/null || echo 0) + 1 ))
	echo "$n" >"$FAKE_DIR/ln.count"
	if [ "$n" = "${FAKE_LN_SLEEP_AT:-}" ]; then
		trap 'kill "$pause_pid" 2>/dev/null; exit 130' INT TERM
		sleep 30 </dev/null >/dev/null 2>&1 &
		pause_pid=$!
		: >"$FAKE_DIR/started"
		wait "$pause_pid"
	fi
	if [ "$n" = "${FAKE_LN_SLEEP_AFTER:-}" ]; then
		"%[2]s" "$@" || exit $?
		trap 'kill "$pause_pid" 2>/dev/null; exit 130' INT TERM
		sleep 30 </dev/null >/dev/null 2>&1 &
		pause_pid=$!
		: >"$FAKE_DIR/started"
		wait "$pause_pid"
		exit 0
	fi
fi
exec "%[2]s" "$@"
`

// 密码里放齐 option 文件里的特殊字符：双引号、反斜杠、#（注释）、;、空格、单引号。
const opsTestPassword = `p"a\ss #w;rd'x`

const sourceUUID = "11111111-2222-3333-4444-555555555555"

type opsEnv struct {
	project string
	fake    string
	tmp     string
	extra   []string
	keep    map[string]bool // fake 目录里的假命令与包装，reset 时保留
}

func newOpsEnv(t *testing.T) *opsEnv {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if _, err := Generate(Options{ProjectName: "ops-demo", Module: "example.com/ops-demo", OutputDir: project}); err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	e := &opsEnv{project: project, fake: filepath.Join(root, "fake"), tmp: filepath.Join(root, "tmp")}
	for _, dir := range []string{e.fake, e.tmp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e.keep = map[string]bool{"mysqldump": true, "mysql": true}
	for name, body := range map[string]string{"mysqldump": fakeMysqldump, "mysql": fakeMysql} {
		if err := os.WriteFile(filepath.Join(e.fake, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range wrappedTools {
		real, err := exec.LookPath(name)
		if err != nil {
			continue // 本机没有这个命令（如 macOS 上的 sha256sum），脚本也就不会调用它
		}
		if err := os.WriteFile(filepath.Join(e.fake, name), []byte(fmt.Sprintf(toolWrapper, name, real)), 0o755); err != nil {
			t.Fatal(err)
		}
		e.keep[name] = true
	}
	return e
}

func (e *opsEnv) reset(t *testing.T) {
	t.Helper()
	for _, dir := range []string{e.fake, e.tmp} {
		entries, _ := os.ReadDir(dir)
		for _, ent := range entries {
			if e.keep[ent.Name()] {
				continue
			}
			os.RemoveAll(filepath.Join(dir, ent.Name()))
		}
	}
	e.extra = nil
}

func (e *opsEnv) command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = e.project
	cmd.Env = append([]string{
		"PATH=" + e.fake + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + e.tmp,
		"FAKE_DIR=" + e.fake,
		"MYSQL_OPS_USER=ops_user",
		"MYSQL_OPS_PASSWORD=" + opsTestPassword,
	}, e.extra...)
	return cmd
}

// run 在项目根目录执行命令。环境从零构造：只带 PATH（假命令优先）、HOME、TMPDIR 和本测试的变量。
func (e *opsEnv) run(t *testing.T, name string, args ...string) (int, string) {
	t.Helper()
	out, err := e.command(name, args...).CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("执行 %s 失败: %v", name, err)
		}
		code = exitErr.ExitCode()
	}
	return code, string(out)
}

func (e *opsEnv) fakeFile(name string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(e.fake, name))
	return string(data), err == nil
}

// assertClean 临时 client 配置必须已删除；argv 与输出里不能有密码；子进程环境里不能还有密码变量。
func (e *opsEnv) assertClean(t *testing.T, output string) {
	t.Helper()
	if entries, _ := os.ReadDir(e.tmp); len(entries) != 0 {
		t.Errorf("临时 client 配置未清理: %v", entries)
	}
	for _, f := range []string{"mysql.argv", "mysqldump.argv", "mysql.queries"} {
		if data, ok := e.fakeFile(f); ok && strings.Contains(data, "w;rd") {
			t.Errorf("%s 里出现了密码", f)
		}
	}
	if leak, ok := e.fakeFile("env.leak"); ok {
		t.Errorf("这些子进程的环境里带着密码: %s", strings.Join(strings.Fields(leak), ", "))
	}
	// 每次 mysql / mysqldump 调用：MYSQL_TEST_LOGIN_FILE 指向脚本私有临时目录里一个不存在的路径。
	if log, ok := e.fakeFile("loginfile.log"); ok {
		for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
			path, state, _ := strings.Cut(line, "|")
			if !strings.HasPrefix(path, e.tmp+"/dbops.") || state != "absent" {
				t.Errorf("MYSQL_TEST_LOGIN_FILE 应指向私有临时目录里不存在的路径，实际 %q（%s）", path, state)
			}
		}
	}
	// 每次 mysql / mysqldump 调用的连接参数：位置和值都固定。
	for _, tool := range []string{"mysql", "mysqldump"} {
		for _, inv := range e.invocations(tool) {
			e.assertConnArgs(t, tool, inv)
		}
	}
	if strings.Contains(output, "w;rd") {
		t.Error("脚本输出里出现了密码")
	}
}

// invocations 把假命令记录的 argv 按调用拆开。
func (e *opsEnv) invocations(tool string) [][]string {
	data, ok := e.fakeFile(tool + ".argv")
	if !ok {
		return nil
	}
	var out [][]string
	for _, chunk := range strings.Split(data, "--END--\n") {
		if chunk != "" {
			out = append(out, strings.Split(strings.TrimSuffix(chunk, "\n"), "\n"))
		}
	}
	return out
}

// assertConnArgs 连接参数必须是这几个、按这个顺序排在最前面：--defaults-file 第一（客户端只在
// 第一个位置识别它），其后的命令行参数压过任何选项文件（含 ~/.mylogin.cnf）。mysql 另加 --local-infile=0。
func (e *opsEnv) assertConnArgs(t *testing.T, tool string, inv []string) {
	t.Helper()
	want := []string{"--protocol=TCP", "--host=127.0.0.1", "--port=3306", "--user=ops_user"}
	if tool == "mysql" {
		want = append(want, "--local-infile=0")
	}
	if len(inv) < len(want)+1 || !strings.HasPrefix(inv[0], "--defaults-file="+e.tmp+"/dbops.") || !strings.HasSuffix(inv[0], "/client.cnf") {
		t.Errorf("%s 的第一个参数应是私有临时目录里的 --defaults-file，实际 %q", tool, inv)
		return
	}
	for i, w := range want {
		if inv[i+1] != w {
			t.Errorf("%s 的第 %d 个参数应为 %q，实际 %q（完整参数 %q）", tool, i+2, w, inv[i+1], inv)
		}
	}
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, ent := range entries {
		names = append(names, ent.Name())
	}
	return names
}

func gunzip(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// backupSet 一份备份的三个文件（相对项目根目录）。
type backupSet struct{ dump, meta, sum string }

// makeBackup 跑一次 backup.sh，返回生成的文件。
func (e *opsEnv) makeBackup(t *testing.T, extra ...string) backupSet {
	t.Helper()
	os.RemoveAll(filepath.Join(e.project, "backups"))
	e.reset(t)
	e.extra = extra
	if code, out := e.run(t, "scripts/db/backup.sh", "--database", "ops_demo"); code != 0 {
		t.Fatalf("准备备份失败: %s", out)
	}
	files := listDir(t, filepath.Join(e.project, "backups"))
	if len(files) != 3 {
		t.Fatalf("备份应生成 3 个文件: %v", files)
	}
	dump := filepath.Join("backups", files[0])
	return backupSet{dump: dump, meta: dump + ".meta", sum: dump + ".sha256"}
}

// rehash 改动 .meta 后重算 .sha256，用来单独测试「内容一致性」而不是被 sha256 先拦下。
func (e *opsEnv) rehash(t *testing.T, b backupSet) {
	t.Helper()
	dump, _ := os.ReadFile(filepath.Join(e.project, b.dump))
	meta, _ := os.ReadFile(filepath.Join(e.project, b.meta))
	content := sha256Hex(dump) + "  " + filepath.Base(b.dump) + "\n" + sha256Hex(meta) + "  " + filepath.Base(b.meta) + "\n"
	if err := os.WriteFile(filepath.Join(e.project, b.sum), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDBScripts_BackupSuccess(t *testing.T) {
	e := newOpsEnv(t)
	code, out := e.run(t, "scripts/db/backup.sh", "--database", "ops_demo")
	if code != 0 {
		t.Fatalf("备份应成功，退出码 %d\n%s", code, out)
	}
	e.assertClean(t, out)

	files := listDir(t, filepath.Join(e.project, "backups"))
	if len(files) != 3 || !strings.HasSuffix(files[0], ".sql.gz") || files[1] != files[0]+".meta" || files[2] != files[0]+".sha256" {
		t.Fatalf("应只留下 .sql.gz、.meta 与 .sha256（无临时文件），实际 %v", files)
	}
	if info, _ := os.Stat(filepath.Join(e.project, "backups")); info.Mode().Perm() != 0o700 {
		t.Errorf("脚本新建的备份目录权限应为 0700，实际 %v", info.Mode().Perm())
	}
	dump := filepath.Join(e.project, "backups", files[0])
	for _, f := range []string{dump, dump + ".meta", dump + ".sha256"} {
		if info, _ := os.Stat(f); info.Mode().Perm() != 0o600 {
			t.Errorf("%s 权限应为 0600，实际 %v", filepath.Base(f), info.Mode().Perm())
		}
	}
	data, _ := os.ReadFile(dump)
	meta, _ := os.ReadFile(dump + ".meta")
	wantSum := sha256Hex(data) + "  " + files[0] + "\n" + sha256Hex(meta) + "  " + files[1] + "\n"
	if got := readFile(t, dump+".sha256"); got != wantSum {
		t.Errorf(".sha256 应同时覆盖数据文件与 .meta:\n%q\n%q", got, wantSum)
	}
	for _, want := range []string{"format=taurus-dbops-mysqldump-1\n", "source_database=ops_demo\n", "source_server_uuid=" + sourceUUID + "\n", "stored_object_markers=0\n"} {
		if !strings.Contains(string(meta), want) {
			t.Errorf(".meta 应包含 %q:\n%s", want, meta)
		}
	}
	if !strings.Contains(out, sha256Hex(data)) || !strings.Contains(out, "server_uuid="+sourceUUID) {
		t.Errorf("备份报告应输出 sha256 与源实例 server_uuid:\n%s", out)
	}

	dumps := e.invocations("mysqldump")
	if len(dumps) != 1 || dumps[0][len(dumps[0])-1] != "ops_demo" {
		t.Fatalf("应只调用一次 mysqldump，库名是最后一个参数: %q", dumps)
	}
	if len(e.invocations("mysql")) != 1 {
		t.Errorf("备份只应调用一次 mysql（读服务端身份）: %q", e.invocations("mysql"))
	}
	argv, _ := e.fakeFile("mysqldump.argv")
	for _, want := range []string{"--single-transaction", "--quick", "--set-gtid-purged=OFF", "--no-tablespaces", "--routines", "--triggers", "--events"} {
		if !strings.Contains(argv, want+"\n") {
			t.Errorf("mysqldump 应带 %s", want)
		}
	}
	// 选项文件里只有密码：主机、端口、账号、协议都在命令行上（见 assertConnArgs）。
	cnf, _ := e.fakeFile("mysqldump.cnf")
	if cnf != "[client]\npassword=\"p\\\"a\\\\ss #w;rd'x\"\n" {
		t.Errorf("临时选项文件应只含加双引号并转义 \\ 与 \" 的密码:\n%s", cnf)
	}
	if mode, _ := e.fakeFile("mysqldump.cnf.mode"); strings.TrimSpace(mode) != "600" {
		t.Errorf("临时 client 配置权限应为 600，实际 %q", mode)
	}

	// 含库级对象的备份：.meta 如实记录标记数，报告提示恢复约束。
	b := e.makeBackup(t, "FAKE_DUMP=stored")
	if got := readFile(t, filepath.Join(e.project, b.meta)); !strings.Contains(got, "stored_object_markers=2\n") {
		t.Errorf("含存储过程与 ALTER DATABASE 的备份应记录 2 个库级对象标记:\n%s", got)
	}
}

func TestDBScripts_BackupFailureLeavesNoFile(t *testing.T) {
	e := newOpsEnv(t)
	for _, mode := range []string{"fail", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			e.reset(t)
			os.RemoveAll(filepath.Join(e.project, "backups"))
			e.extra = []string{"FAKE_DUMP=" + mode}
			code, out := e.run(t, "scripts/db/backup.sh", "--database", "ops_demo")
			if code == 0 {
				t.Fatalf("mysqldump %s 时备份必须失败\n%s", mode, out)
			}
			if files := listDir(t, filepath.Join(e.project, "backups")); len(files) != 0 {
				t.Fatalf("失败时不能留下任何文件（含临时文件），实际 %v", files)
			}
			e.assertClean(t, out)
		})
	}
}

// 失败时的清理只删「本次落下的」正式文件（按 inode 识别），同名但不是本次写的文件必须原样保留。
// 提前放好与本次备份同名的 .sha256（覆盖接下来几秒可能用到的时间戳）：脚本在落盘前的检查处拒绝，
// 此时临时文件都已建好、cleanup 会运行，诱饵文件不能被当成本次的产物删掉。
func TestDBScripts_BackupFailureKeepsForeignFiles(t *testing.T) {
	e := newOpsEnv(t)
	dir := filepath.Join(e.project, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	var decoys []string
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, "ops_demo-"+now.Add(time.Duration(i)*time.Second).Format("20060102-150405")+".sql.gz.sha256")
		if err := os.WriteFile(name, []byte("someone else's file\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		decoys = append(decoys, name)
	}
	code, out := e.run(t, "scripts/db/backup.sh", "--database", "ops_demo")
	if code == 0 || !strings.Contains(out, "目标文件已存在，拒绝覆盖") {
		t.Fatalf("同名 .sha256 已存在时应拒绝，实际 %d:\n%s", code, out)
	}
	for _, d := range decoys {
		if data, err := os.ReadFile(d); err != nil || string(data) != "someone else's file\n" {
			t.Fatalf("不是本次写的同名文件被删除或改写了: %s", filepath.Base(d))
		}
	}
	if files := listDir(t, dir); len(files) != len(decoys) {
		t.Fatalf("除诱饵外不应留下任何文件: %v", files)
	}
	e.assertClean(t, out)
}

func TestDBScripts_RejectBadInput(t *testing.T) {
	e := newOpsEnv(t)
	cases := []struct {
		name  string
		args  []string
		extra []string
	}{
		{name: "库名注入", args: []string{"--database", "x;touch pwned"}},
		{name: "库名带反引号", args: []string{"--database", "x`id`"}},
		{name: "空库名", args: []string{"--database", ""}},
		{name: "未知参数", args: []string{"--database", "ops_demo", "--drop"}},
		{name: "缺账号", args: []string{"--database", "ops_demo"}, extra: []string{"MYSQL_OPS_USER="}},
		{name: "主机名注入", args: []string{"--database", "ops_demo"}, extra: []string{"MYSQL_OPS_HOST=h;id"}},
		{name: "端口非数字", args: []string{"--database", "ops_demo"}, extra: []string{"MYSQL_OPS_PORT=33a"}},
		{name: "MariaDB 客户端", args: []string{"--database", "ops_demo"}, extra: []string{"FAKE_VERSION=mysqldump from 11.4.2-MariaDB"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.reset(t)
			e.extra = tc.extra
			code, out := e.run(t, "scripts/db/backup.sh", tc.args...)
			if code == 0 {
				t.Fatalf("应拒绝\n%s", out)
			}
			if _, ran := e.fakeFile("mysqldump.argv"); ran {
				t.Fatal("拒绝时不应调用 mysqldump")
			}
			if _, err := os.Stat(filepath.Join(e.project, "pwned")); err == nil {
				t.Fatal("库名被当成命令执行了")
			}
			e.assertClean(t, out)
		})
	}

	// 密码变量未设置（而不是设为空）时拒绝：不能静默地用无密码账号去连。
	e.reset(t)
	cmd := exec.Command("scripts/db/backup.sh", "--database", "ops_demo")
	cmd.Dir = e.project
	cmd.Env = []string{"PATH=" + e.fake + ":" + os.Getenv("PATH"), "TMPDIR=" + e.tmp, "FAKE_DIR=" + e.fake, "MYSQL_OPS_USER=ops_user"}
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "MYSQL_OPS_PASSWORD") {
		t.Fatalf("未设置 MYSQL_OPS_PASSWORD 时应拒绝: %v\n%s", err, out)
	}
	// --help 也会调用外部命令（sed）：那时密码同样已经移出环境。
	e.reset(t)
	if code, out := e.run(t, "scripts/db/restore.sh", "--help"); code != 0 || !strings.Contains(out, "--confirm") {
		t.Fatalf("--help 应输出用法: %s", out)
	}
	e.assertClean(t, "")
	// 显式设为空：允许，且 client 配置里不写 password 行。
	e.reset(t)
	e.extra = []string{"MYSQL_OPS_PASSWORD="}
	if code, out := e.run(t, "scripts/db/backup.sh", "--database", "ops_demo"); code != 0 {
		t.Fatalf("显式空密码应允许: %s", out)
	}
	// 空密码也必须写成显式的 password=""：不写时客户端没有任何密码选项，可能退回用调用方环境里的 MYSQL_PWD。
	if cnf, _ := e.fakeFile("mysqldump.cnf"); cnf != "[client]\npassword=\"\"\n" {
		t.Errorf("空密码时选项文件应只有显式的 password=\"\":\n%s", cnf)
	}
	e.assertClean(t, "")
}

func TestDBScripts_RestoreGuards(t *testing.T) {
	e := newOpsEnv(t)
	b := e.makeBackup(t)
	confirm := "ops_drill@127.0.0.1:3306"
	base := []string{"--file", b.dump, "--database", "ops_drill", "--confirm", confirm}

	cases := []struct {
		name      string
		args      []string
		extra     []string
		wantOK    bool
		wantMsg   string
		noQueries bool // 校验失败必须发生在连库之前
		wantSQL   string
	}{
		{name: "confirm 不一致", args: []string{"--file", b.dump, "--database", "ops_drill", "--confirm", "ops_drill@10.0.0.1:3306"}, wantMsg: "目标核验失败", noQueries: true},
		{name: "缺 confirm", args: []string{"--file", b.dump, "--database", "ops_drill"}, wantMsg: "目标核验失败", noQueries: true},
		{name: "端口不同也算不一致", args: base, extra: []string{"MYSQL_OPS_PORT=3307"}, wantMsg: "目标核验失败", noQueries: true},
		{name: "目标库非空", args: base, extra: []string{"FAKE_EXISTS=1", "FAKE_OBJECTS=12"}, wantMsg: "非空"},
		{name: "库不存在且未 --create", args: base, extra: []string{"FAKE_EXISTS=0"}, wantMsg: "--create"},
		{name: "库不存在且 --create", args: append(append([]string{}, base...), "--create"), extra: []string{"FAKE_EXISTS=0"}, wantOK: true, wantSQL: "CREATE DATABASE `ops_drill` CHARACTER SET utf8mb4"},
		{name: "空库", args: base, extra: []string{"FAKE_EXISTS=1", "FAKE_OBJECTS=0", "FAKE_UUID=99999999-0000-0000-0000-000000000000"}, wantOK: true, wantMsg: "server_uuid=99999999-0000-0000-0000-000000000000"},
		{name: "同一实例给出提示", args: base, wantOK: true, wantMsg: "同一个"},
		{name: "导入失败", args: base, extra: []string{"FAKE_IMPORT_EXIT=1"}, wantMsg: "部分数据"},
		{name: "非 .sql.gz", args: []string{"--file", "Makefile", "--database", "ops_drill", "--confirm", confirm}, wantMsg: ".sql.gz", noQueries: true},
		{name: "库名注入", args: []string{"--file", b.dump, "--database", "a';DROP TABLE x;'", "--confirm", confirm}, noQueries: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.reset(t)
			e.extra = tc.extra
			code, out := e.run(t, "scripts/db/restore.sh", tc.args...)
			if (code == 0) != tc.wantOK {
				t.Fatalf("退出码 %d 与预期不符\n%s", code, out)
			}
			if tc.wantMsg != "" && !strings.Contains(out, tc.wantMsg) {
				t.Errorf("输出应包含 %q:\n%s", tc.wantMsg, out)
			}
			queries, queried := e.fakeFile("mysql.queries")
			if tc.noQueries && queried {
				t.Fatalf("校验失败时不应连库，实际执行了:\n%s", queries)
			}
			if strings.Contains(strings.ToUpper(queries), "DROP") {
				t.Fatalf("恢复脚本不应执行任何 DROP:\n%s", queries)
			}
			imported, didImport := e.fakeFile("imported.sql")
			if tc.wantOK || tc.name == "导入失败" {
				if !didImport || !strings.Contains(imported, "-- Dump completed") {
					t.Fatalf("应导入完整备份内容:\n%s", imported)
				}
				argv, _ := e.fakeFile("mysql.argv")
				if !strings.Contains(argv, "--binary-mode\n--one-database\n--default-character-set=utf8mb4\nops_drill\n") {
					t.Errorf("导入应带 --binary-mode（禁用客户端命令）并显式指定目标库:\n%s", argv)
				}
				if !strings.Contains(queries, "/* dbops:identity */ SELECT @@hostname, @@port, @@server_uuid") {
					t.Errorf("连库后应读取服务端身份:\n%s", queries)
				}
			} else if didImport {
				t.Fatal("拒绝时不应导入任何数据")
			}
			if tc.wantSQL != "" && !strings.Contains(queries, tc.wantSQL) {
				t.Errorf("应执行 %q，实际:\n%s", tc.wantSQL, queries)
			}
			e.assertClean(t, out)
		})
	}
}

// 文件层面的拒绝：全部发生在连库之前。
func TestDBScripts_RestoreFileChecks(t *testing.T) {
	e := newOpsEnv(t)
	confirm := "ops_drill@127.0.0.1:3306"
	restore := func(t *testing.T, b backupSet, database string, extraArgs ...string) (int, string) {
		t.Helper()
		e.reset(t)
		args := append([]string{"--file", b.dump, "--database", database, "--confirm", database + "@127.0.0.1:3306"}, extraArgs...)
		return e.run(t, "scripts/db/restore.sh", args...)
	}
	expectRefused := func(t *testing.T, code int, out, msg string) {
		t.Helper()
		if code == 0 || !strings.Contains(out, msg) {
			t.Fatalf("应以「%s」拒绝，退出码 %d:\n%s", msg, code, out)
		}
		if q, queried := e.fakeFile("mysql.queries"); queried {
			t.Fatalf("文件校验失败时不应连库:\n%s", q)
		}
		e.assertClean(t, out)
	}

	t.Run("数据文件被改动", func(t *testing.T) {
		b := e.makeBackup(t)
		f := filepath.Join(e.project, b.dump)
		data, _ := os.ReadFile(f)
		os.WriteFile(f, append(data, 0), 0o600)
		code, out := restore(t, b, "ops_drill")
		expectRefused(t, code, out, "sha256 不匹配，备份文件可能损坏")
	})
	t.Run(".meta 被改动", func(t *testing.T) {
		b := e.makeBackup(t)
		f := filepath.Join(e.project, b.meta)
		os.WriteFile(f, []byte(strings.Replace(readFile(t, f), "ops_demo", "ops_other", 1)), 0o600)
		code, out := restore(t, b, "ops_drill")
		expectRefused(t, code, out, "元数据文件可能损坏")
	})
	t.Run("缺 .meta", func(t *testing.T) {
		b := e.makeBackup(t)
		os.Remove(filepath.Join(e.project, b.meta))
		code, out := restore(t, b, "ops_drill", "--skip-checksum")
		expectRefused(t, code, out, "只支持恢复 backup.sh 生成的备份")
	})
	t.Run(".meta 格式不支持", func(t *testing.T) {
		b := e.makeBackup(t)
		f := filepath.Join(e.project, b.meta)
		os.WriteFile(f, []byte(strings.Replace(readFile(t, f), "taurus-dbops-mysqldump-1", "other-2", 1)), 0o600)
		e.rehash(t, b)
		code, out := restore(t, b, "ops_drill")
		expectRefused(t, code, out, "格式不受支持")
	})
	t.Run("库级对象标记与 .meta 不一致", func(t *testing.T) {
		b := e.makeBackup(t, "FAKE_DUMP=stored")
		f := filepath.Join(e.project, b.meta)
		os.WriteFile(f, []byte(strings.Replace(readFile(t, f), "stored_object_markers=2", "stored_object_markers=0", 1)), 0o600)
		e.rehash(t, b)
		code, out := restore(t, b, "ops_drill")
		expectRefused(t, code, out, "不一致")
	})
	t.Run("含库级对象时拒绝恢复到别的库名", func(t *testing.T) {
		b := e.makeBackup(t, "FAKE_DUMP=stored")
		code, out := restore(t, b, "ops_drill")
		expectRefused(t, code, out, "只支持恢复到同名库")
	})
	// 不带版本注释的例程（CREATE DEFINER=... PROCEDURE / CREATE FUNCTION），且没有 ALTER DATABASE：
	// 也必须算库级对象，只许恢复到同名库。
	for _, fixture := range []string{"bare_procedure", "bare_function"} {
		t.Run("只有裸例程时拒绝恢复到别的库名/"+fixture, func(t *testing.T) {
			b := e.makeBackup(t, "FAKE_DUMP="+fixture)
			if meta := readFile(t, filepath.Join(e.project, b.meta)); !strings.Contains(meta, "stored_object_markers=1\n") {
				t.Fatalf("%s 应记 1 个库级对象标记:\n%s", fixture, meta)
			}
			if dump := gunzip(t, filepath.Join(e.project, b.dump)); strings.Contains(dump, "ALTER DATABASE") || !strings.Contains(dump, "CREATE ") {
				t.Fatalf("夹具应只有裸例程、不含 ALTER DATABASE:\n%s", dump)
			}
			code, out := restore(t, b, "ops_drill")
			expectRefused(t, code, out, "只支持恢复到同名库")
		})
	}
	t.Run("含库级对象时允许恢复到同名库", func(t *testing.T) {
		b := e.makeBackup(t, "FAKE_DUMP=stored")
		e.reset(t)
		e.extra = []string{"FAKE_EXISTS=0", "FAKE_UUID=99999999-0000-0000-0000-000000000000"}
		code, out := e.run(t, "scripts/db/restore.sh", "--file", b.dump, "--database", "ops_demo", "--confirm", "ops_demo@127.0.0.1:3306", "--create")
		if code != 0 {
			t.Fatalf("同名库（另一个实例）应允许恢复:\n%s", out)
		}
		e.assertClean(t, out)
	})
	t.Run("缺 .sha256 默认拒绝", func(t *testing.T) {
		b := e.makeBackup(t)
		os.Remove(filepath.Join(e.project, b.sum))
		code, out := restore(t, b, "ops_drill")
		expectRefused(t, code, out, "--skip-checksum")
		e.reset(t)
		if code, out := e.run(t, "scripts/db/restore.sh", "--file", b.dump, "--database", "ops_drill", "--confirm", confirm, "--skip-checksum"); code != 0 || !strings.Contains(out, "跳过 sha256 校验") {
			t.Fatalf("显式 --skip-checksum 后应放行并给出警告:\n%s", out)
		}
	})
	t.Run("gzip 损坏且 --skip-checksum", func(t *testing.T) {
		b := e.makeBackup(t)
		os.Remove(filepath.Join(e.project, b.sum))
		f := filepath.Join(e.project, b.dump)
		data, _ := os.ReadFile(f)
		data[len(data)/2] ^= 0xff
		os.WriteFile(f, data, 0o600)
		code, out := restore(t, b, "ops_drill", "--skip-checksum")
		expectRefused(t, code, out, "gzip 校验失败")
	})
}

// Makefile 入口把值经环境变量交给脚本（"$$DB"），值里的 shell 元字符不会被执行。
func TestDBScripts_MakefileTargets(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Fatalf("需要 make: %v", err)
	}
	e := newOpsEnv(t)
	code, out := e.run(t, "make", "db-backup", "DB=x;touch pwned")
	if code == 0 {
		t.Fatalf("非法库名应失败\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(e.project, "pwned")); err == nil {
		t.Fatal("make 变量被当成命令执行了")
	}
	if _, ran := e.fakeFile("mysqldump.argv"); ran {
		t.Fatal("非法库名时不应调用 mysqldump")
	}

	if code, out := e.run(t, "make", "db-backup", "DB=ops_demo"); code != 0 {
		t.Fatalf("make db-backup 应成功\n%s", out)
	}
	files := listDir(t, filepath.Join(e.project, "backups"))
	if len(files) != 3 {
		t.Fatalf("应生成一份备份: %v", files)
	}
	e.reset(t)
	e.extra = []string{"FAKE_EXISTS=0"}
	args := []string{"db-restore", "BACKUP_FILE=backups/" + files[0], "DB=ops_drill", "CONFIRM=ops_drill@127.0.0.1:3306"}
	if code, out := e.run(t, "make", args...); code == 0 || !strings.Contains(out, "--create") {
		t.Fatalf("不带 CREATE=1 时库不存在应拒绝\n%s", out)
	}
	if code, out := e.run(t, "make", append(args, "CREATE=0")...); code == 0 || !strings.Contains(out, "--create") {
		t.Fatalf("CREATE=0 不应传 --create\n%s", out)
	}
	code, out = e.run(t, "make", append(args, "CREATE=1")...)
	if code != 0 {
		t.Fatalf("make db-restore CREATE=1 应成功\n%s", out)
	}
	if queries, _ := e.fakeFile("mysql.queries"); !strings.Contains(queries, "CREATE DATABASE `ops_drill`") {
		t.Errorf("CREATE=1 应传成 --create:\n%s", queries)
	}
	e.assertClean(t, out)
}

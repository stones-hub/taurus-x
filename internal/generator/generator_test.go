package generator

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/stones-hub/taurus-x/internal/components"
	"github.com/stones-hub/taurus-x/templates"
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
			assertInitialSecrets(t, outDir, result)
			assertSelfServicePassword(t, outDir)
			assertNoDuplicateSetPasswordEntry(t, outDir)
			assertSessionRevocationWiring(t, outDir)
			assertUserManageGuardWiring(t, outDir)
			assertStartupSelfCheckWiring(t, outDir)
			assertPasswordHashing(t, outDir)
			assertSystemPermissionOnlyPromotable(t, outDir)
			assertRateLimiterBounded(t, outDir)
			assertLogRetentionWiring(t, outDir)
			assertEnvFileFailFast(t, outDir)
			assertSQLLogNoLeak(t, outDir)
			assertLoginAuditStatus(t, outDir)
			assertNoDeadRegisterFlow(t, outDir)
			assertUnselectedAbsent(t, outDir, result.Components)
			assertDockerCrossPlatform(t, outDir)
			assertComposeNoFixedPlatform(t, outDir)
			assertBaseline(t, outDir, tc.project)

			backend := filepath.Join(outDir, "backend")
			runGo(t, backend, "build", "./...")
			runGo(t, backend, "vet", "./...")
			runGo(t, backend, "test", "./...")
		})
	}
}

// adminLoginInsertPattern 从生成的 init.sql 里取出初始管理员的 bcrypt 散列。
//
// 同时把 salt 列必须是空串钉进正则：bcrypt 的盐在散列串里，salt 列只是历史遗留，
// 写了值就说明生成侧还在按旧算法走。
var adminLoginInsertPattern = regexp.MustCompile(`\(1, 'username', '[^']*', '(\$2[aby]\$[^']{50,})', '', 1\)`)

// assertInitialSecrets 初始凭证相关的硬要求：
//
//  1. init.sql 里的散列确实能校验出 CLI 打印的那个随机密码（否则用户照提示登不进去）；
//  2. 明文初始密码不出现在任何生成文件里（只在终端打印一次）；
//  3. .env.example 不带可直接用的 JWT 密钥，且留空（该文件会进使用者的版本库）。
func assertInitialSecrets(t *testing.T, outDir string, result *Result) {
	t.Helper()

	if result.AdminPassword == "" {
		t.Fatal("生成结果未返回初始管理员密码")
	}

	// 1. init.sql 的散列能校验出返回的随机密码。
	//
	//    这里**不能**再用「重算一遍散列比字符串」的办法：bcrypt 自带随机盐，
	//    同一密码每次散列都不同。唯一有意义的断言是拿散列去校验密码 ——
	//    这也正是生成项目登录时做的事。
	sql := readFile(t, filepath.Join(outDir, "scripts", "sql", "init.sql"))
	m := adminLoginInsertPattern.FindStringSubmatch(sql)
	if m == nil {
		t.Fatal("init.sql 里找不到形如 (1, 'username', ..., '<bcrypt 散列>', '', 1) 的初始管理员登录记录")
	}
	gotHash := m[1]
	if err := bcrypt.CompareHashAndPassword([]byte(gotHash), []byte(result.AdminPassword)); err != nil {
		t.Errorf("init.sql 里的 bcrypt 散列校验不过返回的初始密码：按提示登录会失败（%v）", err)
	}

	// 2. 明文密码不能出现在生成产物里（含 README / SQL / .env.example）
	assertNoMatch(t, outDir, result.AdminPassword)

	// 3. .env.example 里 JWT_SECRET 必须留空
	envExample := readFile(t, filepath.Join(outDir, "backend", ".env.example"))
	if !strings.Contains(envExample, "\nJWT_SECRET=\n") && !strings.HasSuffix(envExample, "\nJWT_SECRET=") {
		t.Error(".env.example 的 JWT_SECRET 必须留空（该文件进版本库，不能带现成签名密钥）")
	}
	// 注意要锚到行首：文件里的注释示例（# 对的：JWT_SECRET=xxx）不算带值
	if regexp.MustCompile(`(?m)^\s*JWT_SECRET=\S`).MatchString(envExample) {
		t.Error(".env.example 的 JWT_SECRET 带了值")
	}

	// 弱默认值只能作为「没配置」的哨兵留在配置文件里，启动自检会拒绝它
	httpYAML := readFile(t, filepath.Join(outDir, "backend", "config", "autoload", "http", "http.yaml"))
	if !strings.Contains(httpYAML, "${JWT_SECRET:") {
		t.Error("http.yaml 的 jwt secret 应从 JWT_SECRET 环境变量取值")
	}
}

// assertSelfServicePassword 「已登录普通用户能改自己的密码」这条要求有两半，
// 中间件行为那半由生成项目的 app/middleware/permission_test.go 覆盖，
// 这里只校验布线那半（权限数据与路由链的一致性）：
//
//  1. 权限数据里不能再有 change-password 的接口权限行，否则没配该权限的普通角色
//     仍然会被 PermissionMiddleware 挡掉；
//  2. 路由上 /change-password 必须挂在 userSelfServiceChain 上，
//     而该链不含 PermissionMiddleware。
//
// 两者只要有一边回退，普通用户就又改不了自己的密码了。
func assertSelfServicePassword(t *testing.T, outDir string) {
	t.Helper()

	sql := readFile(t, filepath.Join(outDir, "scripts", "sql", "init.sql"))
	for _, forbidden := range []string{"'/admin/user/change-password'", "api.admin.user.change-password"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("init.sql 仍有 change-password 的接口权限（%s）：普通用户会被权限中间件挡住", forbidden)
		}
	}
	// 自助改密唯一入口是 change-password，必须带旧密码；
	// 管理员重置他人密码走 /admin/user/update（下面单独断言它仍受接口权限保护）。
	dtoSrc := readFile(t, filepath.Join(outDir, "backend", "app", "dto", "user_dto.go"))
	if !strings.Contains(dtoSrc, "OldPassword string `json:\"old_password\"`") {
		t.Error("dto.ChangePasswordRequest 必须收 old_password：自助改密不能没有旧密码校验")
	}
	changePassword := funcBody(
		readFile(t, filepath.Join(outDir, "backend", "app", "service", "auth_service.go")),
		"func (s *AuthService) ChangePassword(")
	if !strings.Contains(changePassword, "helper.VerifyPassword(req.OldPassword") {
		t.Error("AuthService.ChangePassword 必须校验旧密码")
	}

	routes := readFile(t, filepath.Join(outDir, "backend", "app", "routes", "routes.go"))

	selfServiceChain := blockAfter(routes, "userSelfServiceChain := []router.MiddlewareFunc{")
	if selfServiceChain == "" {
		t.Fatal("routes.go 找不到 userSelfServiceChain 定义")
	}
	if strings.Contains(selfServiceChain, "PermissionMiddleware") {
		t.Error("userSelfServiceChain 不应挂 PermissionMiddleware")
	}
	for _, must := range []string{"JWTMiddleware", "SessionValidatorMiddleware", "OperationLogMiddleware", "CSRFMiddleware"} {
		if !strings.Contains(selfServiceChain, must) {
			t.Errorf("userSelfServiceChain 缺少 %s：自助写操作仍要登录/会话/CSRF 校验与操作留痕", must)
		}
	}

	group := groupContaining(routes, "/change-password")
	if group == "" {
		t.Fatal("routes.go 找不到注册 /change-password 的路由组")
	}
	if !strings.Contains(group, "Middleware: userSelfServiceChain") {
		t.Error("/change-password 应挂在 userSelfServiceChain 上")
	}

	// 管理员重置他人密码的入口是 /admin/user/update（password 字段），
	// 它必须留在受接口权限校验的链上 —— 删掉重复的自助入口不能顺带放开它。
	updateGroup := groupContaining(routes, "Handler: http.HandlerFunc(app.Core.UserController.UpdateUser)")
	if updateGroup == "" {
		t.Fatal("routes.go 找不到注册 UserController.UpdateUser 的路由组")
	}
	if !strings.Contains(updateGroup, "Middleware: userProtectedChain") {
		t.Error("/admin/user/update（管理员重置他人密码）必须留在 userProtectedChain 上")
	}
	if !strings.Contains(sql, "'/admin/user/update'") {
		t.Error("init.sql 缺少 /admin/user/update 的接口权限：管理员重置他人密码不应无权限校验")
	}
}

// assertNoDuplicateSetPasswordEntry 「无需旧密码的自助改密入口」必须整条链路都不存在。
//
// 这个入口曾经从 JWT 取本人 ID、DTO 只收 new_password，等于「拿到一个有效 Token
// 就能静默改掉密码」，和个人中心需要旧密码的 change-password 重复且更弱。
// 这里逐层断言残留：路由 / controller / service / DTO / 权限行 / 前端页面 / API / 类型 / 路由表。
func assertNoDuplicateSetPasswordEntry(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	for _, tc := range []struct {
		path      string
		forbidden string
		why       string
	}{
		{filepath.Join(backend, "app", "routes", "routes.go"), "/set-password", "路由仍注册了无需旧密码的改密入口"},
		{filepath.Join(backend, "app", "controller", "user_controller.go"), "func (c *UserController) SetPassword(", "controller 仍有 SetPassword"},
		{filepath.Join(backend, "app", "service", "auth_service.go"), "func (s *AuthService) SetPassword(", "service 仍有 SetPassword"},
		{filepath.Join(backend, "app", "dto", "user_dto.go"), "SetPasswordRequest", "DTO 仍有 SetPasswordRequest"},
		{filepath.Join(outDir, "scripts", "sql", "init.sql"), "/admin/user/set-password", "权限数据里仍有该接口权限行"},
		{filepath.Join(outDir, "scripts", "sql", "init.sql"), "api.admin.user.set-password", "权限数据里仍有该接口权限编码"},
		{filepath.Join(outDir, "frontend", "src", "router", "index.ts"), "set-password", "前端路由仍指向已删除的页面（死链接）"},
		{filepath.Join(outDir, "frontend", "src", "api", "user.ts"), "apiSetPassword", "前端 API 仍保留 apiSetPassword"},
		{filepath.Join(outDir, "frontend", "src", "types", "user.ts"), "SetPasswordRequest", "前端类型仍保留 SetPasswordRequest"},
	} {
		if strings.Contains(readFile(t, tc.path), tc.forbidden) {
			t.Errorf("%s：%s（命中 %q）", tc.path, tc.why, tc.forbidden)
		}
	}

	if _, err := os.Stat(filepath.Join(outDir, "frontend", "src", "views", "profile", "set-password.vue")); !os.IsNotExist(err) {
		t.Error("frontend/src/views/profile/set-password.vue 仍存在：重复改密页面应已删除")
	}

	// 删掉重复入口不能把个人中心的改密也删了：页面仍要调自助改密接口
	profile := readFile(t, filepath.Join(outDir, "frontend", "src", "views", "profile", "index.vue"))
	for _, must := range []string{"apiChangePassword", "old_password"} {
		if !strings.Contains(profile, must) {
			t.Errorf("profile/index.vue 缺少 %s：个人中心的旧密码+新密码改密流程必须保留可用", must)
		}
	}
}

// assertSessionRevocationWiring 会话吊销的权威标记必须是数据库列 auth_version，
// 而且凭证/状态变更与版本自增必须在同一个事务里。
//
// 这条是 Pi 在生成项目上实测复现过的阻断（Redis 计数丢失 → 旧会话复活；
// MySQL 已改、Redis 吊销写失败恢复后旧 Token 仍可用）的结构性兜底：
// 行为由生成项目的单测覆盖，但「两步在一个事务里」只能在源码结构上断言
// —— 不接数据库的单测无法观察事务边界，这个限制是明确的。
func assertSessionRevocationWiring(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")

	// 1. 底座列：NOT NULL DEFAULT 1（0 不是合法版本，用来表示「查不到用户」）
	sql := readFile(t, filepath.Join(outDir, "scripts", "sql", "init.sql"))
	authVersionCol := regexp.MustCompile("`auth_version`\\s+BIGINT UNSIGNED NOT NULL DEFAULT 1")
	if !authVersionCol.MatchString(sql) {
		t.Error("admin_users 缺少 `auth_version` BIGINT UNSIGNED NOT NULL DEFAULT 1：吊销标记必须在库里，Redis 丢了会让旧会话复活")
	}
	if !strings.Contains(sql, "`user_source`, `auth_version`") {
		t.Error("初始管理员的 INSERT 未写入 auth_version：首次登录会因版本非法被拒")
	}

	// 2. 模型字段不能序列化给前端（认证内部状态）
	user := readFile(t, filepath.Join(backend, "app", "model", "admin_user.go"))
	if !regexp.MustCompile(`AuthVersion\s+int64\s+` + "`" + `json:"-"`).MatchString(user) {
		t.Error("model.AdminUser.AuthVersion 必须存在且 json:\"-\"（不下发给前端）")
	}

	// 3. 会话存储里不能再有「版本缺失就回退成 0 继续用」的路径
	store := readFile(t, filepath.Join(backend, "app", "helper", "store", "auth_redis_store.go"))
	for _, forbidden := range []string{"InvalidateUserSessions", "sessionEpochKeyPrefix", "CurrentEpoch"} {
		if strings.Contains(store, forbidden) {
			t.Errorf("auth_redis_store.go 仍残留 Redis 计数吊销的 %s：计数被淘汰后已吊销会话会复活", forbidden)
		}
	}
	if !strings.Contains(store, "AuthVersionResolver") {
		t.Error("auth_redis_store.go 应通过 AuthVersionResolver 读数据库里的权威版本")
	}

	// 4. 凭证/状态变更与版本自增同事务：每个吊销点都要落在 RunInTx 的闭包里
	for _, tc := range []struct {
		file string
		fn   string
	}{
		{filepath.Join("app", "service", "auth_service.go"), "func (s *AuthService) ChangePassword("},
		// 管理员重置他人密码的唯一入口（原来在 AuthService.UpdateUserPassword，
		// 那个无操作人、无目标账号保护的函数已删除）
		{filepath.Join("app", "service", "user_service.go"), "func (s *AdminUserService) UpdateUser("},
		{filepath.Join("app", "service", "user_service.go"), "func (s *AdminUserService) UpdateUserStatus("},
	} {
		body := funcBody(readFile(t, filepath.Join(backend, tc.file)), tc.fn)
		if body == "" {
			t.Errorf("%s 找不到 %s", tc.file, tc.fn)
			continue
		}
		if !strings.Contains(body, "s.store.RunInTx(") {
			t.Errorf("%s%s 必须在 RunInTx 内完成变更：分两次写会留下「已改凭证、未吊销」的窗口", tc.file, tc.fn)
		}
		if !strings.Contains(body, "BumpAuthVersion(") {
			t.Errorf("%s%s 未自增认证版本：已签发的 Token 不会失效", tc.file, tc.fn)
		}
		if strings.Contains(body, "authStore") {
			t.Errorf("%s%s 仍在用 Redis 侧吊销（authStore）", tc.file, tc.fn)
		}
	}

	// 5. 登录必须把「校验凭证那一刻的版本」交给签发侧，签发侧不许自己现读
	login := funcBody(readFile(t, filepath.Join(backend, "app", "service", "auth_service.go")),
		"func (s *AuthService) PasswordLogin(")
	if !strings.Contains(login, "s.store.RunInTx(") {
		t.Error("PasswordLogin 必须在同一事务（同一快照）内校验凭证并读取认证版本")
	}
	if !strings.Contains(login, "u.AuthVersion <= 0") {
		t.Error("PasswordLogin 读到非法认证版本时必须拒绝登录（fail closed）")
	}

	// 新建用户必须显式带上初始版本，否则 0 会被当成「查不到用户」，一登录就被拒
	addUser := funcBody(readFile(t, filepath.Join(backend, "app", "service", "user_service.go")),
		"func (s *AdminUserService) AddUser(")
	if !strings.Contains(addUser, "AuthVersion: 1") {
		t.Error("AddUser 应显式设置 AuthVersion: 1，不要依赖列默认值")
	}

	jwt := readFile(t, filepath.Join(backend, "app", "middleware", "jwt.go"))
	if !strings.Contains(jwt, "func GenerateJWTToken(uid, username string, authVersion int64)") {
		t.Error("GenerateJWTToken 必须由调用方传入 authVersion，不能自己现读当前版本")
	}
	rotate := funcBody(jwt, "func rotateSession(")
	if rotate == "" {
		t.Fatal("jwt.go 找不到 rotateSession")
	}
	// 刷新必须沿用 ValidateSession 返回的旧版本；现读当前版本会让刷新
	// 变成绕过改密/禁用吊销的后门。
	if !strings.Contains(rotate, "RegisterSession(userID, newJTI, ttl, authVersion)") {
		t.Error("rotateSession 必须以旧会话校验时拿到的版本登记新会话")
	}
	if strings.Contains(rotate, "CurrentAuthVersion") {
		t.Error("rotateSession 不得重新读取当前认证版本")
	}

	// 6. 权威来源要在启动时注入；没注入时会话校验 fail closed，所以必须有这步
	main := readFile(t, filepath.Join(backend, "bin", "main.go"))
	if !strings.Contains(main, "SetAuthVersionResolver(app.Core.Store.User())") {
		t.Error("main.go 未注入认证版本权威来源：会话校验会全量 fail closed")
	}
}

// assertUserManageGuardWiring 目标账号保护的布线契约：
// 有 user/update、update-status、delete 接口权限不等于可以动超管账号。
//
// 这里只断结构（判定是否在写之前、是否在同一事务里、handler 是否把 JWT 操作人传下去）；
// 行为由生成项目里的 app/controller/user_admin_guard_test.go 用 sqlmock 在**真实调用链**
// 上验证「拒绝时零写入」，两者缺一不可：
// 原来的漏洞恰恰是「纯守卫单测全绿，但守卫排在三次独立提交的最后一步」。
func assertUserManageGuardWiring(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")

	// 1. 统一守卫存在，且操作人/目标事实都在锁定之后当前读
	guard := readFile(t, filepath.Join(backend, "app", "service", "user_guard.go"))
	for _, want := range []string{
		"func guardUserWrite(ctx context.Context, tx *repo.Tx, operatorID, targetID uint64) (*userWriteGuard, error)",
		"func CheckTargetUserManageable(operatorIsSuperAdmin bool, targetFacts []RoleFact) error",
		"func lockUserRowsAscending(ctx context.Context, tx *repo.Tx, userIDs ...uint64)",
		"func sortedUniqueUserIDs(userIDs []uint64) []uint64",
		"func lockSelfUserRow(ctx context.Context, tx *repo.Tx, userID uint64) error",
		"func currentUserRoles(ctx context.Context, tx *repo.Tx, userID uint64)",
	} {
		if !strings.Contains(guard, want) {
			t.Errorf("user_guard.go 缺少 %s", want)
		}
	}

	// 1b. 身份锁协议的顺序：先锁稳定用户行，再当前读身份。
	// 反过来（先读身份再锁）等于判定依据仍是加锁前那份事实，锁加得再对也没用。
	guardBody := funcBody(guard, "func guardUserWrite(")
	if guardBody == "" {
		t.Error("user_guard.go 找不到 guardUserWrite 函数体")
	} else {
		lockAt := strings.Index(guardBody, "lockUserRowsAscending(ctx, tx,")
		readAt := strings.Index(guardBody, "currentUserRoles(ctx, tx,")
		switch {
		case lockAt < 0:
			t.Error("guardUserWrite 未调用 lockUserRowsAscending：没有锁住稳定用户行，并发改身份挡不住")
		case readAt < 0:
			t.Error("guardUserWrite 未调用 currentUserRoles：身份读不是加锁当前读，拿到的是事务快照")
		case readAt < lockAt:
			t.Error("guardUserWrite 在加锁之前就读了身份：判定依据仍是加锁前的旧事实")
		}
	}
	// 目标用户行必须由锁定读提供，不能再出现「锁前先普通读一遍目标」——
	// 那条普通读会成为事务第一条 SELECT，在 REPEATABLE READ 下就此钉死快照。
	if !strings.Contains(guard, "tx.User().LockByUserID(") {
		t.Error("lockUserRowsAscending 未走 LockByUserID：用户行没有被 FOR UPDATE 锁住")
	}

	// 1c. 仓储层：加锁读必须真的渲染出 FOR UPDATE / FOR SHARE，
	// 并且角色关联的整体替换按 user_id 条件删（逐行按主键删会漏掉快照外新增的行）。
	base := readFile(t, filepath.Join(backend, "app", "repo", "base.go"))
	for _, want := range []string{"clause.Locking", "lockForUpdate = \"UPDATE\"", "lockForShare  = \"SHARE\""} {
		if !strings.Contains(base, want) {
			t.Errorf("repo/base.go 缺少 %s：加锁当前读没有落到 SQL 上", want)
		}
	}
	userRepo := readFile(t, filepath.Join(backend, "app", "repo", "admin_user_repo.go"))
	if !strings.Contains(userRepo, "firstLocking(ctx, lockForUpdate") {
		t.Error("admin_user_repo.go 的 LockByUserID 不是排他锁当前读")
	}
	userRoleRepo := readFile(t, filepath.Join(backend, "app", "repo", "admin_user_role_repo.go"))
	for _, want := range []string{
		"findLocking(ctx, lockForUpdate",
		"func (r *AdminUserRoleRepository) DeleteByUserID(",
		"Where(\"user_id = ?\", userID).Delete(",
	} {
		if !strings.Contains(userRoleRepo, want) {
			t.Errorf("admin_user_role_repo.go 缺少 %s", want)
		}
	}
	roleRepo := readFile(t, filepath.Join(backend, "app", "repo", "admin_role_repo.go"))
	if !strings.Contains(roleRepo, "firstLocking(ctx, lockForShare") {
		t.Error("admin_role_repo.go 缺少共享锁当前读：角色事实读的是事务快照")
	}

	// 2. 每个「写用户」的入口都必须：在 RunInTx 内、且 guardUserWrite 排在任何写入之前
	for _, tc := range []struct {
		file   string
		fn     string
		writes []string
	}{
		{
			file:   filepath.Join("app", "service", "user_service.go"),
			fn:     "func (s *AdminUserService) UpdateUser(",
			writes: []string{"UpdateByID(", "SyncPasswordHash(", "replaceUserRolesTx("},
		},
		{
			file:   filepath.Join("app", "service", "user_service.go"),
			fn:     "func (s *AdminUserService) UpdateUserStatus(",
			writes: []string{"UpdateByID("},
		},
		{
			file:   filepath.Join("app", "service", "role_service.go"),
			fn:     "func (s *AdminRoleService) UpdateUserRoles(",
			writes: []string{"replaceUserRolesTx("},
		},
	} {
		body := funcBody(readFile(t, filepath.Join(backend, tc.file)), tc.fn)
		if body == "" {
			t.Errorf("%s 找不到 %s", tc.file, tc.fn)
			continue
		}
		if !strings.Contains(body, "s.store.RunInTx(") {
			t.Errorf("%s%s 必须在 RunInTx 内完成：判定与写入不同事务时，判定完成后目标仍可能被授予超管", tc.file, tc.fn)
		}
		guardAt := strings.Index(body, "guardUserWrite(ctx, tx,")
		if guardAt < 0 {
			t.Errorf("%s%s 未调用 guardUserWrite：非超管可借接口权限改超管账号", tc.file, tc.fn)
			continue
		}
		for _, write := range tc.writes {
			writeAt := strings.Index(body, write)
			if writeAt < 0 {
				continue
			}
			if writeAt < guardAt {
				t.Errorf("%s%s 的 %s 排在 guardUserWrite 之前：拒绝时会留下已写入的半成功状态", tc.file, tc.fn, write)
			}
		}
	}

	// 3. 删除走同一条带守卫的路径，不另开一条绕过判定的写入
	del := funcBody(readFile(t, filepath.Join(backend, "app", "service", "user_service.go")),
		"func (s *AdminUserService) DeleteUser(")
	if !strings.Contains(del, "s.UpdateUserStatus(ctx, operatorID,") {
		t.Error("DeleteUser 应复用带守卫的 UpdateUserStatus，不要自己直接写库")
	}

	// 3b. 角色关联的整体替换与入参角色校验都要走锁协议的那套读写
	roleSvc := readFile(t, filepath.Join(backend, "app", "service", "role_service.go"))
	replace := funcBody(roleSvc, "func replaceUserRolesTx(")
	if replace == "" {
		t.Error("role_service.go 找不到 replaceUserRolesTx")
	} else {
		if !strings.Contains(replace, "tx.UserRole().DeleteByUserID(ctx, userID)") {
			t.Error("replaceUserRolesTx 未按 user_id 条件删除：逐行按快照主键删会漏掉并发新增的关联行，" +
				"替换完会多出一个本次没打算赋的角色")
		}
		if strings.Contains(replace, "FindByUserID(") {
			t.Error("replaceUserRolesTx 仍先查关联行再删：那是按快照主键删的写法")
		}
	}
	roleFacts := funcBody(roleSvc, "func txRoleFactsByIDs(")
	if roleFacts == "" {
		t.Error("role_service.go 找不到 txRoleFactsByIDs")
	} else {
		if !strings.Contains(roleFacts, "FindByRoleIDAnyStatusForShare(") {
			t.Error("txRoleFactsByIDs 读角色事实没有用「共享锁当前读 + 不过滤 status」：" +
				"用快照读会按旧角色事实放行；按 status 过滤则会把「目标已持有、后来被禁用的角色」" +
				"当成角色不存在，编辑一次资料就整笔失败")
		}
		if strings.Contains(roleFacts, "FindByRoleIDForShare(") {
			t.Error("txRoleFactsByIDs 仍用只查启用角色的 FindByRoleIDForShare：" +
				"状态策略应交给 CheckRoleStatusAssignable 判定，不要在查询里悄悄过滤")
		}
	}

	// 3c. 资料编辑的角色更新语义：缺失/null = 不改，显式 [] = 清空；
	// 禁用角色已持有可保留、新增必须拒绝。
	userDTO := readFile(t, filepath.Join(backend, "app", "dto", "user_dto.go"))
	provided := funcBody(userDTO, "func (r *UpdateUserRequest) RolesProvided()")
	if provided == "" {
		t.Error("user_dto.go 缺少 RolesProvided：无法区分「role_ids 字段缺失」与「显式空数组」")
	} else {
		if !strings.Contains(provided, "r.RoleIDs != nil") {
			t.Error("RolesProvided 不是按切片 nil 性判断：缺失/null 解码后是 nil 切片，`[]` 是非 nil 空切片，" +
				"只有 nil 性能把两者分开")
		}
		if strings.Contains(provided, "len(") {
			t.Error("RolesProvided 用了长度判断：len(RoleIDs) == 0 把「没传」和「显式清空」混成一种，" +
				"于是「只改昵称」的保存会把角色关联全删掉")
		}
	}
	updateUser := funcBody(readFile(t, filepath.Join(backend, "app", "service", "user_service.go")),
		"func (s *AdminUserService) UpdateUser(")
	if !strings.Contains(updateUser, "req.RolesProvided()") {
		t.Error("UpdateUser 未按 RolesProvided 选择性更新角色：没传 role_ids 时会把关联全部删掉")
	}
	guardAt := strings.Index(updateUser, "guardUserWrite(ctx, tx,")
	providedAt := strings.Index(updateUser, "req.RolesProvided()")
	if guardAt >= 0 && providedAt >= 0 && providedAt < guardAt {
		t.Error("UpdateUser 先看 role_ids 再做目标账号保护：不传 role_ids 就成了绕过账号保护的口子")
	}
	roleGuard := readFile(t, filepath.Join(backend, "app", "service", "role_guard.go"))
	if !strings.Contains(roleGuard, "func CheckRoleStatusAssignable(") {
		t.Error("role_guard.go 缺少 CheckRoleStatusAssignable：禁用角色的「可保留、不可新增」没有判定")
	}
	if !strings.Contains(roleGuard, "Status: role.Status") {
		t.Error("RoleFact 没带上角色状态（RoleFactOf 未取 Status）：赋角判定看不到角色状态，" +
			"只能回退到在查询里按 status 过滤")
	}
	for _, tc := range []struct {
		file string
		fn   string
	}{
		{filepath.Join("app", "service", "user_service.go"), "func (s *AdminUserService) UpdateUser("},
		{filepath.Join("app", "service", "role_service.go"), "func (s *AdminRoleService) UpdateUserRoles("},
	} {
		body := funcBody(readFile(t, filepath.Join(backend, tc.file)), tc.fn)
		if !strings.Contains(body, "CheckRoleStatusAssignable(") {
			t.Errorf("%s%s 未校验角色状态：可以借编辑页把已禁用的角色挂到新账号上", tc.file, tc.fn)
		}
	}
	// 编辑页的角色事实必须以库里的关联表为准：只列启用角色时，持有中的禁用角色
	// 回显不出来，整体替换就把这条关联顺手删了。
	userRolesDetail := funcBody(roleSvc, "func (s *AdminRoleService) GetUserRolesAndPermissions(")
	if !strings.Contains(userRolesDetail, "FindByRoleIDAnyStatus(") {
		t.Error("GetUserRolesAndPermissions 没有补查「持有中但不在启用列表里」的角色：" +
			"编辑页丢掉这条关联，保存资料时会静默撤销该角色")
	}

	// 3d. 赋角路由只有 /admin/role/assign 一个：文档/注释里写成 /admin/user/assign-roles
	// 会让后续评审按一份不存在的入口清单核对覆盖面。
	assertNoMatch(t, outDir, "assign-roles")

	// 3d'. 菜单/按钮下发的真实路径是 /admin/user/menus-buttons（handler 在
	// RoleController 上，但路由注册在 /admin/user 前缀的 userBaseChain 里）。
	// 写成 /admin/role/menus-buttons 会让评审按一条不存在的路径去核对
	// 「这个接口挂没挂接口权限」，而这两个前缀的中间件链恰好不一样。
	assertNoMatch(t, outDir, "/admin/role/menus-buttons")

	// 3e. 禁用角色不贡献权限：过滤只能落在那条**有效权限**联表查询上。
	//
	// 这是全项目非超管「实际被授权了哪些接口/菜单/按钮」的唯一**授权**出口：
	// 接口权限校验（helper/permission.Checker）和菜单、按钮下发都从它取数。
	// （菜单树的祖先节点另有一次按 parent_id 的补查，那只补结构、只为已授权的后代
	// 发生、且祖先必须启用 —— 见 GetUserMenusAndButtons 的说明，不是第二条授权路径。）
	// 原实现只有 ur→rp→p 三张表、只过滤 p.status，于是角色被禁用后持有它的人
	// 照样能调接口、照样看得见菜单 —— 「禁用角色」只是在角色列表里灰了一下。
	userPerms := funcBody(userRoleRepo, "func (r *AdminUserRoleRepository) FindUserPermissions(")
	if userPerms == "" {
		t.Error("admin_user_role_repo.go 找不到 FindUserPermissions")
	} else {
		for _, tc := range []struct {
			fragment string
			why      string
		}{
			{`Joins("JOIN admin_roles r ON r.role_id = ur.role_id")`,
				"没联角色表就无从过滤角色状态：禁用角色会继续贡献权限"},
			{"r.status = 1", "被禁用的角色必须不贡献任何接口/菜单/按钮权限"},
			{"p.status = 1", "权限行自身禁用不生效（原有语义，不能顺手删掉）"},
			{"DISTINCT", "多角色并集要去重"},
		} {
			if !strings.Contains(userPerms, tc.fragment) {
				t.Errorf("FindUserPermissions 缺少 %s：%s", tc.fragment, tc.why)
			}
		}
	}
	// 配对的另一半：角色**关联**读不过滤状态，否则禁用角色启用后恢复不了、
	// 编辑页也回显不出这条关联，一次「改昵称」的整体替换就把它删了。
	assocRead := funcBody(userRoleRepo, "func (r *AdminUserRoleRepository) FindByUserID(")
	if strings.Contains(assocRead, "status") {
		t.Error("FindByUserID 按角色状态过滤了关联：禁用角色的关联会在保存资料时被静默删掉，" +
			"启用后也恢复不了。过滤属于 FindUserPermissions 那条有效权限查询")
	}
	// 「有没有超管权力」必须要求角色启用中，与「目标账号是否受保护」（刻意不看状态）相反
	isSuper := funcBody(roleSvc, "func (s *AdminRoleService) IsUserSuperAdmin(")
	if !strings.Contains(isSuper, "role.Status == constants.RoleStatusEnabled") {
		t.Error("IsUserSuperAdmin 未要求 super_admin 角色启用中：给权力的判定要和 FindUserPermissions " +
			"过滤禁用角色同一口径")
	}
	// 菜单与按钮的**授权**来源只能是那个唯一出口，不能另起一条从关联表自己拼权限的路
	// （祖先补查走 permissionRepo.FindByPermissionID，只补结构、不扩授权，不在此禁之列）
	menus := funcBody(roleSvc, "func (s *AdminRoleService) GetUserMenusAndButtons(")
	if !strings.Contains(menus, "s.GetUserAllPermissions(ctx, userID)") {
		t.Error("GetUserMenusAndButtons 非超管分支未走 GetUserAllPermissions：过滤条件会分叉，" +
			"出现「接口不通了但菜单还在」这类半截效果")
	}
	if strings.Contains(menus, "FindByRoleID(") || strings.Contains(menus, "rolePermissionRepo") {
		t.Error("GetUserMenusAndButtons 自己从角色权限关联拼权限：绕过了角色状态过滤")
	}
	checker := funcBody(readFile(t, filepath.Join(backend, "app", "helper", "permission", "checker.go")),
		"func (c *Checker) CheckAPIPermission(")
	if !strings.Contains(checker, "c.roleService.GetUserAllPermissions(ctx, userID)") {
		t.Error("CheckAPIPermission 未走 GetUserAllPermissions：接口权限会绕过角色状态过滤")
	}

	// 4. 无操作人、无目标保护的覆盖式重置他人密码入口必须已删除
	auth := readFile(t, filepath.Join(backend, "app", "service", "auth_service.go"))
	if strings.Contains(auth, "func (s *AuthService) UpdateUserPassword(") {
		t.Error("AuthService.UpdateUserPassword 仍存在：它不认操作人、不做目标账号保护，是重置超管密码的旁路")
	}

	// 4b. 自助改密必须按同一个加锁顺序开头（先用户行、再凭证行）。
	// 管理员重置密码是「锁 admin_users → 改 admin_user_logins → 自增 auth_version」，
	// 自助改密若反过来，两条路径在同一账号上并发就是一对相反锁序，互等成死锁。
	changePwd := funcBody(auth, "func (s *AuthService) ChangePassword(")
	if changePwd == "" {
		t.Error("auth_service.go 找不到 ChangePassword")
	} else {
		lockAt := strings.Index(changePwd, "lockSelfUserRow(ctx, tx,")
		readAt := strings.Index(changePwd, "tx.UserLogin()")
		switch {
		case lockAt < 0:
			t.Error("ChangePassword 未先锁本人用户行：与管理员重置密码形成相反加锁顺序，并发会死锁")
		case readAt >= 0 && readAt < lockAt:
			t.Error("ChangePassword 先碰凭证行再锁用户行：加锁顺序与管理员重置密码相反")
		}
	}

	// 5. handler 必须从 JWT 取操作人并传给 service（请求体里给不了），
	//    且用 FailByError 把 ErrForbidden 映射成 403
	ctrl := readFile(t, filepath.Join(backend, "app", "controller", "user_controller.go"))
	for _, tc := range []struct {
		fn   string
		call string
	}{
		{"func (c *UserController) UpdateUser(", "c.UserService.UpdateUser(r.Context(), operatorID, &req)"},
		{"func (c *UserController) UpdateUserStatus(", "c.UserService.UpdateUserStatus(r.Context(), operatorID, req.UserID, req.Status)"},
		{"func (c *UserController) DeleteUser(", "c.UserService.DeleteUser(r.Context(), operatorID, req.UserID)"},
	} {
		body := funcBody(ctrl, tc.fn)
		if body == "" {
			t.Errorf("user_controller.go 找不到 %s", tc.fn)
			continue
		}
		if !strings.Contains(body, "middleware.CurrentUserIDFromJWT(r)") {
			t.Errorf("%s 未从 JWT 取操作人", tc.fn)
		}
		if !strings.Contains(body, tc.call) {
			t.Errorf("%s 未把操作人传给 service（应为 %s）", tc.fn, tc.call)
		}
		if !strings.Contains(body, "FailByError(w, err)") {
			t.Errorf("%s 应用 FailByError 返回 403，否则拒绝会被当成 500", tc.fn)
		}
	}

	// 6. 真实调用链的行为回归必须在场（纯守卫单测无法发现「守卫排在写之后」）
	regression := readFile(t, filepath.Join(backend, "app", "controller", "user_admin_guard_test.go"))
	for _, want := range []string{
		"repo.NewStoreWithDB(db)",
		"c.UpdateUser(rec, jwtRequest(",
		"mock.ExpectationsWereMet()",
		"TestUpdateUser_NonSuperAdminCannotWriteSuperAdminAccount",
		"TestUpdateUser_IdentityQueryErrorFailsClosed",
		"TestUpdateUser_SuperAdminManagesOrdinaryUser",
		"TestDeleteUser_NonSuperAdminCannotDeleteSuperAdmin",
		// 身份锁协议：锁序、同一 ID 只锁一次、用户行缺失 fail closed、
		// 以及零角色账号的首次赋角色（锁锚在用户行上才覆盖得到）
		"TestUpdateUser_LocksUserRowsInAscendingOrder",
		"TestUpdateUser_SameUserLockedOnce",
		"TestUpdateUser_MissingUserRowFailsClosed",
		"TestUpdateUserRoles_FirstAssignmentToRolelessTarget",
		"TestUpdateUserRoles_NonSuperAdminCannotGrantSuperAdmin",
		"TestUpdateUserRoles_NonSuperAdminCannotRevokeSuperAdmin",
		// role_ids 的选择性更新语义（缺失/null 不改、[] 清空）与禁用角色的
		// 「可保留、不可新增」，都必须在 handler 的真实 JSON 解码上验证
		"TestUpdateUser_RoleIDsSelectiveUpdate",
		"TestUpdateUser_ResetPasswordWithoutRoleIDs",
		"TestUpdateUser_KeepsHeldDisabledRole",
		"TestUpdateUser_RejectsNewlyAddedDisabledRole",
		// 已持有的禁用角色要能被**明确移除**：只放行「保留」而不放行「取消」，
		// 等于禁用角色一旦挂上就摘不掉
		"TestUpdateUser_RemovesHeldDisabledRole",
		"TestGetUserInfo_IncludesHeldDisabledRole",
		// 缺失与 null 只能用原始 JSON 文本表达，结构体请求体表达不了
		"json.RawMessage(body)",
		`"role_ids":null`,
		`"role_ids":[]`,
		// 期望里必须要求加锁子句，否则快照读也能过
		"FOR UPDATE",
		"FOR SHARE",
	} {
		if !strings.Contains(regression, want) {
			t.Errorf("user_admin_guard_test.go 缺少 %s", want)
		}
	}

	// 6b. 加锁 SQL 的形状回归（MySQL dialector + DryRun，不用 SQLite 冒充）
	lockSQL := readFile(t, filepath.Join(backend, "app", "repo", "identity_lock_sql_test.go"))
	for _, want := range []string{
		"TestIdentityLockSQL_LockingReadsCarryLockClause",
		"TestIdentityLockSQL_LockStrengthsAreNotInterchangeable",
		"TestIdentityLockSQL_PlainReadsStayUnlocked",
		"TestIdentityLockSQL_DeleteRolesByUserIDCondition",
		"newDryRunDB(t)",
	} {
		if !strings.Contains(lockSQL, want) {
			t.Errorf("identity_lock_sql_test.go 缺少 %s", want)
		}
	}
	if strings.Contains(lockSQL, "sqlite") {
		t.Error("加锁 SQL 测试不得用 SQLite：它没有 FOR UPDATE / FOR SHARE，等于什么都没测")
	}

	// 6d. 禁用角色不贡献权限的行为回归：
	// 语句形状（repo，MySQL dialector + DryRun）+ 真实调用链（接口中间件、菜单/按钮 handler）。
	//
	// 两层都要在场：只有 SQL 形状测试证不了「调用链真的走这条查询」，
	// 只有调用链测试又证不了「执行到库的语句带着过滤」——
	// 喂几行假数据回来，实现过不过滤都能拼出一样的响应。
	permSQL := readFile(t, filepath.Join(backend, "app", "repo", "user_permission_sql_test.go"))
	for _, want := range []string{
		"TestUserPermissionSQL_DisabledRolesContributeNothing",
		"TestUserPermissionSQL_AssociationReadKeepsDisabledRoles",
		"newDryRunDB(t)",
		"r.status = 1",
		"p.status = 1",
	} {
		if !strings.Contains(permSQL, want) {
			t.Errorf("user_permission_sql_test.go 缺少 %s", want)
		}
	}
	permScope := readFile(t, filepath.Join(backend, "app", "controller", "user_permission_scope_test.go"))
	for _, want := range []string{
		"TestAPIPermissionMiddleware_OnlyEnabledRolesGrantAccess",
		"TestAPIPermissionMiddleware_IdentityQueryErrorFailsClosed",
		"TestAPIPermissionMiddleware_PermissionQueryErrorFailsClosed",
		"TestMenusButtons_OnlyEnabledRolesContribute",
		// parent_id != 0 的嵌套菜单：祖先补查只为已授权后代发生，且祖先必须启用
		"TestMenusButtons_AncestorBackfillOnlyForAuthorizedDescendants",
		// 跑的是真的中间件 + 真的 Checker，不是直接调 service
		"middleware.PermissionMiddleware()",
		"permission.NewChecker(",
		"c.MenusButtons(rec, jwtRequest(",
		// 期望正则必须要求角色状态过滤：去掉 r.status = 1 就匹配不上
		`JOIN admin_roles r ON r\.role_id = ur\.role_id`,
		`AND r\.status = 1 AND p\.status = 1`,
	} {
		if !strings.Contains(permScope, want) {
			t.Errorf("user_permission_scope_test.go 缺少 %s", want)
		}
	}
	// 禁用角色的「保留 / 取消 / 新增拒绝」纯判定也要有单测兜底（超管无豁免口）
	roleGuardUnit := readFile(t, filepath.Join(backend, "app", "service", "role_guard_test.go"))
	for _, want := range []string{
		"TestCheckRoleStatusAssignable",
		"TestCheckRoleStatusAssignable_NoSuperAdminExemption",
	} {
		if !strings.Contains(roleGuardUnit, want) {
			t.Errorf("role_guard_test.go 缺少 %s", want)
		}
	}

	// 6c. 固定锁序的纯单测（去重、丢 0、升序）
	guardUnit := readFile(t, filepath.Join(backend, "app", "service", "user_guard_test.go"))
	if !strings.Contains(guardUnit, "TestSortedUniqueUserIDs") {
		t.Error("user_guard_test.go 缺少 TestSortedUniqueUserIDs：固定锁序没有单测兜底")
	}

	// 7. 个人中心入口：复用已有 profile 路由，键盘可达（原生 button），不新增页面
	home := readFile(t, filepath.Join(outDir, "frontend", "src", "views", "home", "index.vue"))
	if !strings.Contains(home, "<button type=\"button\" class=\"user-entry\"") {
		t.Error("顶栏个人中心入口应是原生 button（Tab 可聚焦、Enter/Space 可触发），不要给 div 挂 click")
	}
	if !strings.Contains(home, "router.push({ name: 'profile' })") {
		t.Error("顶栏入口未跳转到已有的 profile 路由")
	}
	router := readFile(t, filepath.Join(outDir, "frontend", "src", "router", "index.ts"))
	if !strings.Contains(router, "name: 'profile'") {
		t.Error("前端路由缺少 profile 路由：顶栏入口会跳到死链接")
	}

	// 8. 用户编辑页：持有中的禁用角色要回显并可保留，但不能新增；
	// 角色列表不能只拉启用角色，否则这条关联在页面上根本不存在。
	edit := readFile(t, filepath.Join(outDir, "frontend", "src", "views", "user", "edit.vue"))
	for _, want := range []string{
		"heldRoleIds",
		"function roleSelectable(",
		":disabled=\"!roleSelectable(role)\"",
		"status: QUERY_ALL",
	} {
		if !strings.Contains(edit, want) {
			t.Errorf("user/edit.vue 缺少 %s：禁用角色要么丢关联、要么能被新增", want)
		}
	}
	userTypes := readFile(t, filepath.Join(outDir, "frontend", "src", "types", "user.ts"))
	if !strings.Contains(userTypes, "role_ids?: number[]") {
		t.Error("UpdateUserRequest.role_ids 不是可选字段：前端表达不了「本次不改角色」，" +
			"只能被迫传 [] 而把角色全撤销")
	}
}

// 登录签发 / 刷新 / 解析 Token 都直接用 http.jwt.secret，那个开关只关掉
// 请求侧的鉴权中间件，关不掉签名功能。
func assertStartupSelfCheckWiring(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	check := funcBody(readFile(t, filepath.Join(backend, "app", "bootstrap.go")), "func checkSecurityConfig(")
	if check == "" {
		t.Fatal("bootstrap.go 找不到 checkSecurityConfig")
	}
	if strings.Contains(check, "http.jwt.enabled") {
		t.Error("checkSecurityConfig 不得读 http.jwt.enabled：enabled=false 时签名功能仍在用密钥")
	}
	if !strings.Contains(check, "helper.ValidateStartupSecurity(") {
		t.Error("checkSecurityConfig 应调用 helper.ValidateStartupSecurity 做无条件自检")
	}

	startup := readFile(t, filepath.Join(backend, "app", "helper", "startup_check.go"))
	if !strings.Contains(startup, "func ValidateStartupSecurity(getString func(key string) string) error") {
		t.Error("helper 缺少可测的 ValidateStartupSecurity(getString) 契约入口")
	}
}

// assertPasswordHashing 密码散列算法的落地契约（T5 第 1 项）。
//
// 这几条断言钉的是「新密码一律 bcrypt、老散列登录即升级」这条链路在生成项目里
// 真实存在。退化的后果是静默的：散列算法退回 SHA-256 不会有任何报错，
// 功能照常，只是库被拖走之后口令能被离线批量爆破。
func assertPasswordHashing(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	crypto := readFile(t, filepath.Join(backend, "app", "helper", "crypto.go"))

	// 1. 生成新散列只能走 bcrypt，且 cost 不低于 10
	if !strings.Contains(crypto, "bcrypt.GenerateFromPassword") {
		t.Error("helper.HashPassword 必须用 bcrypt.GenerateFromPassword 生成新散列")
	}
	if !strings.Contains(crypto, "const bcryptCost = bcrypt.DefaultCost") {
		t.Error("bcryptCost 应取 bcrypt.DefaultCost（=10）：cost 低于 10 的 bcrypt 已经不足以抵御离线爆破")
	}
	// 旧算法只允许出现在「校验历史数据」的路径上，不能用来生成新密码
	hash := funcBody(crypto, "func HashPassword(password string) (string, error) {")
	if hash == "" {
		t.Fatal("crypto.go 找不到 HashPassword")
	}
	if strings.Contains(hash, "sha256") || strings.Contains(hash, "LegacyHashPassword") {
		t.Error("HashPassword 里出现了旧算法：新密码必须一律 bcrypt")
	}

	// 2. 超过 72 字节必须显式拒绝，不能静默截断（否则「前 72 字节相同」的两个密码互通）
	if !strings.Contains(crypto, "MaxPasswordBytes = 72") || !strings.Contains(crypto, "ErrPasswordTooLong") {
		t.Error("crypto.go 缺少 72 字节上限与 ErrPasswordTooLong：bcrypt 会忽略超出部分，必须拒绝而不是截断")
	}
	if !strings.Contains(hash, "return \"\", ErrPasswordTooLong") {
		t.Error("HashPassword 必须对超长密码返回 ErrPasswordTooLong")
	}

	// 3. 校验按存储格式分流，旧路径用常量时间比较
	verify := funcBody(crypto, "func VerifyPassword(password, stored, salt string) bool {")
	if verify == "" {
		t.Fatal("crypto.go 找不到 VerifyPassword")
	}
	for _, want := range []string{"IsBcryptHash(stored)", "bcrypt.CompareHashAndPassword", "subtle.ConstantTimeCompare"} {
		if !strings.Contains(verify, want) {
			t.Errorf("VerifyPassword 缺少 %q", want)
		}
	}
	// 只认 Go 会生成的三个前缀：$2$ / $2x$ 是有缺陷的历史变体
	isBcrypt := funcBody(crypto, "func IsBcryptHash(stored string) bool {")
	for _, want := range []string{`"$2a$"`, `"$2b$"`, `"$2y$"`} {
		if !strings.Contains(isBcrypt, want) {
			t.Errorf("IsBcryptHash 缺少前缀 %s", want)
		}
	}

	// 4. 旧散列登录成功后立刻升级，且必须在事务提交之后 ——
	//    放在事务里的话，写回失败会把「校验已通过」一起回滚掉，用户登不进去。
	authSrc := readFile(t, filepath.Join(backend, "app", "service", "auth_service.go"))
	login := funcBody(authSrc, "func (s *AuthService) PasswordLogin(")
	if login == "" {
		t.Fatal("auth_service.go 找不到 PasswordLogin")
	}
	if !strings.Contains(login, "helper.NeedsRehash(") {
		t.Error("PasswordLogin 必须用 helper.NeedsRehash 识别旧格式散列并触发升级")
	}
	if !strings.Contains(login, "s.upgradeLegacyCredential(") {
		t.Error("PasswordLogin 未调用 upgradeLegacyCredential：旧散列永远不会被升级")
	}
	// 事务闭包的范围就是「提交之前」，升级调用不能落在里面
	txBody := blockAfter(login, "s.store.RunInTx(ctx, func(tx *repo.Tx) error {")
	if txBody == "" {
		t.Fatal("PasswordLogin 找不到事务闭包")
	}
	if strings.Contains(txBody, "upgradeLegacyCredential(") {
		t.Error("散列升级必须在事务提交之后：写回失败不能回滚掉已经成功的登录")
	}
	// 写回要带上刚校验过的旧值做条件，否则会覆盖掉并发改密的结果
	upgrade := funcBody(authSrc, "func (s *AuthService) upgradeLegacyCredential(")
	if !strings.Contains(upgrade, "UpgradeCredentialIfUnchanged(") {
		t.Error("散列写回必须走 UpgradeCredentialIfUnchanged（WHERE 带旧值），否则可能覆盖并发改密")
	}
	if strings.Contains(upgrade, "AuthVersion") || strings.Contains(upgrade, "auth_version") {
		t.Error("散列升级不得动 auth_version：那会把用户自己刚建立的会话踢掉")
	}

	// 5. 旧的「生成盐」入口不得残留：留着就会有人继续用它建新密码
	if strings.Contains(crypto, "func GenerateSalt(") {
		t.Error("crypto.go 仍保留 GenerateSalt：bcrypt 自带盐，这个入口只会把人引回旧算法")
	}

	// 6. 自带「忘记管理员密码」的离线工具：只从 stdin 读、只输出散列
	hashpw := readFile(t, filepath.Join(backend, "bin", "hashpw", "main.go"))
	if !strings.Contains(hashpw, "helper.HashPassword(") {
		t.Error("bin/hashpw 必须复用 helper.HashPassword，不能自己实现一套散列")
	}
	if !strings.Contains(hashpw, "os.Stdin") {
		t.Error("bin/hashpw 必须从标准输入读密码")
	}
	// 命令行参数会进 shell history 和 ps 输出，必须直接拒绝
	if !strings.Contains(hashpw, "if len(os.Args) > 1 {") {
		t.Error("bin/hashpw 必须拒绝命令行参数形式的密码：参数会进 shell history 和 ps 输出")
	}

	// 7. 任何路径都不得在事务内计算 bcrypt。
	//
	//    bcrypt 是**故意**慢的（cost=10 约 50~100ms）。放进事务就等于让这一百毫秒
	//    全程压在已经 FOR UPDATE 锁住的用户行上：并发改密 / 管理员批量重置时，
	//    锁等待会直接叠成接口超时。散列是纯计算，不依赖事务内的任何事实，
	//    所以必须在 RunInTx 之前算好 —— 顺带也让「密码超长」这类错误在开事务前返回。
	for _, tc := range []struct {
		file string
		fn   string
	}{
		{filepath.Join("app", "service", "auth_service.go"), "func (s *AuthService) ChangePassword("},
		{filepath.Join("app", "service", "user_service.go"), "func (s *AdminUserService) UpdateUser("},
	} {
		body := funcBody(readFile(t, filepath.Join(backend, tc.file)), tc.fn)
		if body == "" {
			t.Errorf("%s 找不到 %s", tc.file, tc.fn)
			continue
		}
		hashAt := strings.Index(body, "helper.HashPassword(")
		if hashAt < 0 {
			t.Errorf("%s%s 没有调用 helper.HashPassword", tc.file, tc.fn)
			continue
		}
		txAt := strings.Index(body, "s.store.RunInTx(")
		if txAt < 0 {
			t.Errorf("%s%s 找不到 RunInTx", tc.file, tc.fn)
			continue
		}
		if hashAt > txAt {
			t.Errorf("%s%s 在事务内计算 bcrypt：cost=10 的散列会让行锁多持约 100ms", tc.file, tc.fn)
		}
	}
	// 仓储方法全都可能在事务里被调用，所以 repo 包整体不该碰明文散列
	repoDir := filepath.Join(backend, "app", "repo")
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		t.Fatalf("读取 repo 目录失败: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if strings.Contains(readFile(t, filepath.Join(repoDir, e.Name())), "HashPassword(") {
			t.Errorf("repo/%s 里出现了 HashPassword：仓储方法会在事务内被调用，散列必须由调用方在事务外算好", e.Name())
		}
	}
}

// assertSystemPermissionOnlyPromotable 系统权限只能升不能降（T5 第 2 项）。
//
// is_system=1 是一道保护：UpdatePermission / DeletePermission 都靠它拦住
// 「改掉或删掉系统内置权限」。一旦这个位能被接口改回 0，攻击者就有了一条
// 两步走的前门 —— 先降级，再随便改编码或删掉，权限校验随之失效。
func assertSystemPermissionOnlyPromotable(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	svc := readFile(t, filepath.Join(backend, "app", "service", "permission_service.go"))
	repoSrc := readFile(t, filepath.Join(backend, "app", "repo", "admin_permission_repo.go"))

	// 1. 专门的 is_system 写入入口必须拒绝 1 → 0
	toggle := funcBody(svc, "func (s *AdminPermissionsService) UpdatePermissionIsSystem(")
	if toggle == "" {
		t.Fatal("permission_service.go 找不到 UpdatePermissionIsSystem")
	}
	if !strings.Contains(toggle, "perm.IsSystem == 1 && isSystem == 0") {
		t.Error("UpdatePermissionIsSystem 必须显式拒绝「系统权限改为普通权限」")
	}
	if !strings.Contains(toggle, "系统权限不能改为普通权限") {
		t.Error("拒绝降级时要给出明确错误信息，不能静默忽略（调用方会以为改成功了）")
	}
	// 入参只能是 0/1。放过 2 这类值等于变相降级：全代码库判定系统权限用的都是 == 1
	if !strings.Contains(toggle, "isSystem != 0 && isSystem != 1") {
		t.Error("UpdatePermissionIsSystem 必须校验入参只能是 0 或 1：is_system=2 等于已经被降级")
	}
	if !strings.Contains(toggle, "PromoteToSystemByID(") {
		t.Error("0 → 1 的升级应走 PromoteToSystemByID（SQL 层带只升不降的条件）")
	}

	// 2. 升级语句在 SQL 层就不可能反向写
	promote := funcBody(repoSrc, "func (r *AdminPermissionsRepository) PromoteToSystemByID(")
	if promote == "" {
		t.Fatal("admin_permission_repo.go 找不到 PromoteToSystemByID")
	}
	if !strings.Contains(promote, `"permission_id = ? AND is_system = 0"`) {
		t.Error("PromoteToSystemByID 的 WHERE 必须带 is_system = 0：这样任何时序下都只能 0→1，不依赖上层先读后判")
	}

	// 3. 通用更新入口不得再写 is_system
	//    它是「先读后判再写」的形状，读完到写之间这行可能已被升级，
	//    updateData 里带着 is_system=0 就会把它降回去。
	update := funcBody(svc, "func (s *AdminPermissionsService) UpdatePermission(")
	if update == "" {
		t.Fatal("permission_service.go 找不到 UpdatePermission")
	}
	if strings.Contains(update, `"is_system"`) {
		t.Error("UpdatePermission 不得把 is_system 写进 updateData：先读后判存在时序窗口，会把并发升级的权限降级")
	}

	// 4. 除了「新建时定性」和「只升不降的升级语句」，不应再有别的 is_system 写入路径
	for _, forbidden := range []string{`Update("is_system", 0)`, `"is_system": 0`, `IsSystem: 0,`} {
		if strings.Contains(svc, forbidden) || strings.Contains(repoSrc, forbidden) {
			t.Errorf("出现把 is_system 写成 0 的代码（%s）：系统权限降级只应通过直接改库完成", forbidden)
		}
	}

	// 5. 生成项目自带这项不变量的单元测试
	if _, err := os.Stat(filepath.Join(backend, "app", "service", "permission_is_system_test.go")); err != nil {
		t.Errorf("生成项目缺少 app/service/permission_is_system_test.go: %v", err)
	}
}

// assertRateLimiterBounded per-IP 限流记录必须有界（T5 第 3 项）。
//
// 公共库的 tlimit.CompositeRateLimiter 用一张只增不减的 map 存 per-IP 桶，
// 公网入口（登录 / 验证码 / 前端错误上报）每来一个新 IP 就永久多一条记录，
// 换 IP 的扫描流量能一直把它撑到 OOM。这里断言两件事：这些入口确实换成了
// 项目内带回收的实现；回收条件没有被放松成「可以把正在被限流的 IP 一笔勾销」。
func assertRateLimiterBounded(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	mwDir := filepath.Join(backend, "app", "middleware")
	rateLimit := readFile(t, filepath.Join(mwDir, "rate_limit.go"))
	limiter := readFile(t, filepath.Join(mwDir, "ip_rate_limiter.go"))

	// 1. 两处 per-IP 限流都用项目内的实现，不再用无界的那个
	if strings.Contains(rateLimit, "tlimit.NewCompositeRateLimiter") {
		t.Error("rate_limit.go 仍在用 tlimit.NewCompositeRateLimiter：它的 per-IP map 只增不减")
	}
	if strings.Contains(rateLimit, "*tlimit.CompositeRateLimiter") {
		t.Error("rate_limit.go 仍在用 tlimit.CompositeRateLimiter 类型")
	}
	if n := strings.Count(rateLimit, "newIPRateLimiter("); n != 2 {
		t.Errorf("rate_limit.go 里构造 ipRateLimiter 的地方应有 2 处（敏感接口 + 全局限流），实际 %d 处", n)
	}

	// 2. 记录数有上限，且有摊还式回收
	for _, want := range []string{"rateLimitMaxEntries", "rateLimitSweepInterval", "rateLimitIdleTTL"} {
		if !strings.Contains(limiter, want) {
			t.Errorf("ip_rate_limiter.go 缺少 %s", want)
		}
	}
	if !strings.Contains(limiter, "rateLimitMaxEntries = 100000") {
		t.Error("per-IP 记录数上限应显式写成 100000：没有上限时内存保护形同虚设")
	}
	track := funcBody(limiter, "func (l *ipRateLimiter) trackLocked(ip string, now time.Time) *tokenBucket {")
	if !strings.Contains(track, "len(l.buckets) >= l.maxEntries") {
		t.Error("新建 per-IP 记录前必须检查记录数上限")
	}

	// 3. 回收只能丢「已经回满」的桶。
	//    丢掉没回满的桶，等于把一个正在被限流的 IP 的欠账勾销 ——
	//    攻击者停够一个清扫周期就能重置配额，口令爆破的速率上限随之失效。
	sweep := funcBody(limiter, "func (l *ipRateLimiter) sweepLocked(now time.Time) {")
	if sweep == "" {
		t.Fatal("ip_rate_limiter.go 找不到 sweepLocked")
	}
	if !strings.Contains(sweep, "bucket.refillAndIsFull(") {
		t.Error("回收条件必须含 refillAndIsFull：回收没回满的桶等于给被限流的 IP 重置配额")
	}
	if !strings.Contains(sweep, "rateLimitIdleTTL") {
		t.Error("回收条件应含闲置时长，避免低频 IP 删了又建")
	}

	// 4. 限流语义不变：双层都要过，顺序和被替换的实现一致
	allow := funcBody(limiter, "func (l *ipRateLimiter) Allow(ip string) rateLimitResult {")
	if allow == "" {
		t.Fatal("ip_rate_limiter.go 找不到 Allow")
	}
	ipTake := strings.Index(allow, "bucket.take(")
	globalTake := strings.Index(allow, "l.global.take(")
	switch {
	case ipTake < 0 || globalTake < 0:
		t.Error("Allow 必须同时判定单 IP 桶和全局桶")
	case ipTake > globalTake:
		t.Error("判定顺序应与被替换实现一致：先单 IP 桶再全局桶（改顺序就是改限流语义）")
	}

	// 5. 配置键名不变：改了名字等于线上配置静默失效
	for _, key := range []string{
		"http.rate_limit.composite.enabled",
		"http.rate_limit.composite.ip_capacity",
		"http.rate_limit.composite.global_capacity",
		"http.rate_limit.composite.fill_interval",
		"http.rate_limit.basic.enabled",
		"http.rate_limit.sensitive.",
	} {
		if !strings.Contains(rateLimit, key) {
			t.Errorf("rate_limit.go 缺少配置键 %q：键名变动会让既有部署的限流配置静默失效", key)
		}
	}

	// 6. 生成项目自带回收与并发的单元测试
	if _, err := os.Stat(filepath.Join(mwDir, "ip_rate_limiter_test.go")); err != nil {
		t.Errorf("生成项目缺少 app/middleware/ip_rate_limiter_test.go: %v", err)
	}
}

// assertLogRetentionWiring 日志表定期清理（T5 第 4 项）。
//
// 四张流水表（访问 / 操作 / 登录 / 前端错误）只写不删，撑满磁盘后数据库整体不可写，
// 等于整个服务挂掉。这里断言清理任务确实被接到启动流程上、保留期默认 180 天、
// 删除语句带时间条件和分批上限、退出时能停。
func assertLogRetentionWiring(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")

	// 1. 启动时挂上，退出时停掉；不依赖默认关闭的 cron 组件
	bootstrap := readFile(t, filepath.Join(backend, "app", "bootstrap.go"))
	start := funcBody(bootstrap, "func startLogCleanup() {")
	if start == "" {
		t.Fatal("bootstrap.go 找不到 startLogCleanup")
	}
	if !strings.Contains(bootstrap, "startLogCleanup()\n") {
		t.Error("startLogCleanup 没有在 init 里被调用：清理任务不会启动")
	}
	if !strings.Contains(start, "service.ResolveLogRetentionDays(") {
		t.Error("startLogCleanup 应通过 service.ResolveLogRetentionDays 解析保留期")
	}
	// 停止函数必须注册进 cleanups，而且必须**头插**：cleanups 是顺序执行的，
	// BuildComponents 的 cleanup（关 DB / Redis / 日志）已经在队首。
	// 直接 append 的话会先关数据库，再去停这个还在发 DELETE 的后台任务，
	// 退出日志里全是 `sql: database is closed`。
	if !strings.Contains(start, "cleanups = append([]func(){stop}, cleanups...)") {
		t.Error("清理任务的停止函数必须头插进 cleanups：append 会让数据库先于后台任务关闭")
	}
	if strings.Contains(start, "cleanups = append(cleanups, stop)") {
		t.Error("清理任务的停止函数不能尾插：cleanups 的顺序就是关闭顺序")
	}
	gcleanup := funcBody(bootstrap, "func gracefulCleanup(")
	if !strings.Contains(gcleanup, "for _, cleanup := range cleanups {") {
		t.Error("gracefulCleanup 应顺序遍历 cleanups：头插的前提是它按切片顺序执行")
	}
	// 配置非法时拒绝启动：这个值控制不可逆的删除，不能按一个猜出来的保留期去删
	if !strings.Contains(start, "log.Fatalf") {
		t.Error("保留期配置非法时必须让启动失败，不能回退到某个默认值继续删数据")
	}
	cleanupSrc := readFile(t, filepath.Join(backend, "app", "service", "log_cleanup.go"))
	if strings.Contains(cleanupSrc, "crontab") || strings.Contains(cleanupSrc, "cron.") {
		t.Error("日志清理不得挂在 cron 组件上：cron 默认关闭，等于默认不清理")
	}

	// 2. 默认 180 天，0 关闭，负数/非整数报错
	if !strings.Contains(cleanupSrc, "DefaultLogRetentionDays = 180") {
		t.Error("默认保留期应为 180 天（已确认的方案）")
	}
	resolve := funcBody(cleanupSrc, "func ResolveLogRetentionDays(getString func(string) string) (int, error) {")
	if resolve == "" {
		t.Fatal("log_cleanup.go 找不到 ResolveLogRetentionDays")
	}
	// 必须 GetString + 显式解析：GetInt 对 `abc` 返回 0，而 0 在这里是「永久保留」的合法值，
	// 一个拼错的配置会被静默解读成「不清理」
	if !strings.Contains(resolve, "strconv.Atoi(") {
		t.Error("保留期应取字符串后显式解析：GetInt 会把非法值变成 0，而 0 是「不清理」的合法值")
	}
	if !strings.Contains(resolve, "days < 0") {
		t.Error("负数保留期必须报错（大概率是想表达「不要删」）")
	}

	// 3. 配置项在 config.yaml 和 .env.example 里都给到位
	cfg := readFile(t, filepath.Join(backend, "config", "config.yaml"))
	if !strings.Contains(cfg, `log_retention_days: "${LOG_RETENTION_DAYS:180}"`) {
		t.Error("config.yaml 缺少 log_retention_days（默认 180）")
	}
	envExample := readFile(t, filepath.Join(backend, ".env.example"))
	if !strings.Contains(envExample, "LOG_RETENTION_DAYS=180") {
		t.Error(".env.example 缺少 LOG_RETENTION_DAYS=180")
	}

	// 4. 四张表都在清理清单里，且登录日志用的是自己的时间列
	retentionRepo := readFile(t, filepath.Join(backend, "app", "repo", "log_retention_repo.go"))
	for _, want := range []string{
		`Table: "admin_access_logs", TimeColumn: "created_at"`,
		`Table: "admin_operation_logs", TimeColumn: "created_at"`,
		`Table: "admin_front_error_logs", TimeColumn: "created_at"`,
		// 登录日志没有 created_at，照抄会生成一条无效 SQL
		`Table: "admin_user_login_logs", TimeColumn: "login_time"`,
	} {
		if !strings.Contains(retentionRepo, want) {
			t.Errorf("日志清理清单缺少 %s：漏掉的表永远不会被清理", want)
		}
	}

	// 5. 删除语句：只按时间条件 + 必须分批
	del := funcBody(retentionRepo, "func (r *LogRetentionRepository) DeleteOlderThan(")
	if del == "" {
		t.Fatal("log_retention_repo.go 找不到 DeleteOlderThan")
	}
	if !strings.Contains(del, `target.TimeColumn+" < ?"`) {
		t.Error("删除条件必须是「时间列 < cutoff」：条件写错或丢失会删到保留期内的数据")
	}
	if !strings.Contains(del, "Limit(limit)") {
		t.Error("删除必须带 LIMIT：否则第一次清理就是一条删几百万行的巨型事务")
	}
	if !strings.Contains(cleanupSrc, "logCleanupBatchSize = 5000") {
		t.Error("单批删除行数应为 5000（毫秒级小事务，锁持有时间短）")
	}
	if !strings.Contains(cleanupSrc, "logCleanupBatchPause") {
		t.Error("批次之间需要停顿，给业务查询让出 IO 和行锁")
	}
	// 首次执行延迟要带随机抖动：多实例同时启动时，四张表的 DELETE 会在同一秒
	// 撞到同一个库上互相争行锁。抖动把它们摊开，不需要分布式锁。
	if !strings.Contains(cleanupSrc, "logCleanupStartJitter") {
		t.Error("首次清理延迟缺少随机抖动：多实例同时启动会让各进程的 DELETE 撞在一起")
	}
	if !strings.Contains(cleanupSrc, "rand.Int64N(") {
		t.Error("抖动应由 rand 生成（这里只要「别撞在一起」，不需要密码学随机）")
	}

	// 6. 日志里只报行数，不报日志内容（访问日志带 IP/UA/路径，操作日志带操作参数）
	if strings.Contains(cleanupSrc, "%+v") || strings.Contains(cleanupSrc, "deleted, rows") {
		t.Error("清理日志不得打印被删记录的内容，只报表名和行数")
	}

	// 7. 生成项目自带单元测试
	for _, rel := range [][]string{
		{"app", "service", "log_cleanup_test.go"},
		{"app", "repo", "log_retention_sql_test.go"},
	} {
		p := filepath.Join(append([]string{backend}, rel...)...)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("生成项目缺少 %s: %v", filepath.Join(rel...), err)
		}
	}
}

// assertEnvFileFailFast -env 指定的环境变量文件读不到时必须启动失败。
//
// 配置库 taurus-pro-config 的 Initialize 只在加载 env 文件失败时打一行
// `Error loading .env file` 就继续，进程会带着默认配置起来：端口不监听、
// 数据库反复 Access denied，而 supervisor 只显示 RUNNING。所以校验必须在
// Initialize 之前，并把错误返回出去（bootstrap 的 log.Fatal 负责非 0 退出）。
func assertEnvFileFailFast(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	componentsSrc := readFile(t, filepath.Join(backend, "internal", "taurus", "components.go"))

	if !strings.Contains(componentsSrc, "func ensureEnvFileReadable(env string) error") {
		t.Fatal("internal/taurus/components.go 缺少 ensureEnvFileReadable(env) 校验入口")
	}

	// 1. 必须在 Initialize 之前调用，且失败直接返回错误
	provide := funcBody(componentsSrc, "func ProvideConfigComponent(opts *ConfigOptions) (*config.Config, error) {")
	if provide == "" {
		t.Fatal("components.go 找不到 ProvideConfigComponent")
	}
	call := strings.Index(provide, "ensureEnvFileReadable(opts.Env)")
	initialize := strings.Index(provide, "configComponent.Initialize(")
	switch {
	case call < 0:
		t.Error("ProvideConfigComponent 未调用 ensureEnvFileReadable(opts.Env)")
	case initialize < 0:
		t.Error("ProvideConfigComponent 找不到 configComponent.Initialize 调用")
	case call > initialize:
		t.Error("ensureEnvFileReadable 必须在 configComponent.Initialize 之前调用")
	}
	if !strings.Contains(provide, "if err := ensureEnvFileReadable(opts.Env); err != nil {") {
		t.Error("env 文件校验失败必须直接返回错误，不能只打日志")
	}

	// 2. -env ""（Docker CMD 的用法）保持跳过加载
	check := funcBody(componentsSrc, "func ensureEnvFileReadable(env string) error {")
	if !strings.Contains(check, `if env == "" {`) || !strings.Contains(check, "return nil") {
		t.Error("ensureEnvFileReadable 必须在 env 为空串时跳过检查：Docker CMD 用 -env \"\"")
	}

	// 3. 存在 + 普通文件 + 实际 open 一次（Stat 通过不代表运行用户能读）
	for _, want := range []string{"os.Stat(env)", "os.IsNotExist(err)", "info.IsDir()", "info.Mode().IsRegular()", "os.Open(env)"} {
		if !strings.Contains(check, want) {
			t.Errorf("ensureEnvFileReadable 缺少 %q", want)
		}
	}

	// 4. 错误信息要给出路径和可执行指引，且不读文件内容
	if !strings.Contains(componentsSrc, "cp .env.example .env.local") {
		t.Error("env 校验的错误信息缺少 `cp .env.example .env.local` 指引")
	}
	if !strings.Contains(componentsSrc, "读权限") {
		t.Error("env 校验的错误信息缺少运行用户读权限的提示")
	}
	for _, forbidden := range []string{"os.ReadFile(env)", "io.ReadAll"} {
		if strings.Contains(check, forbidden) {
			t.Errorf("ensureEnvFileReadable 不得读取文件内容（出现 %q）：错误信息可能带出配置值", forbidden)
		}
	}

	// 5. 生成项目自带该校验的单元测试
	if _, err := os.Stat(filepath.Join(backend, "internal", "taurus", "env_check_test.go")); err != nil {
		t.Errorf("生成项目缺少 internal/taurus/env_check_test.go: %v", err)
	}
}

// assertNoDeadRegisterFlow 未路由的自助注册流程（T1 已要求删除）不得残留：
// 它是一条没人走、也没人审计的建号路径。
// assertLoginAuditStatus 登录审计状态的持久化契约：
// 登录失败必须以 login_status=0 落库。model 上一旦出现 gorm default，GORM 就会把
// 零值（失败）当成「未赋值」并用默认值替换，于是失败被记成成功、接口却照常返回 401，
// 真实环境里曾出现「12 条带失败原因的日志全是成功」。
func assertLoginAuditStatus(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")

	// 1. model 的 login_status 不得带任何 gorm 默认值
	modelSrc := readFile(t, filepath.Join(backend, "app", "model", "admin_user_login_log.go"))
	m := loginStatusTagPattern.FindStringSubmatch(modelSrc)
	if m == nil {
		t.Fatal("admin_user_login_log.go 里找不到 LoginStatus int8 字段定义")
	}
	if strings.Contains(m[1], "default:") {
		t.Errorf("LoginStatus 的 gorm tag 带了默认值（%s）：登录失败的 0 会被替换，审计日志失真", m[1])
	}

	// 2. 建表默认值不能是 1（正常走不到，走到了也不该把失败记成成功）
	initSQL := readFile(t, filepath.Join(outDir, "scripts", "sql", "init.sql"))
	d := loginStatusDDLPattern.FindStringSubmatch(initSQL)
	if d == nil {
		t.Fatal("init.sql 里找不到 admin_user_login_logs.login_status 的列定义")
	}
	if d[1] != "0" {
		t.Errorf("init.sql 的 login_status 默认值是 %s：漏写该列时会把失败记成成功，应为 0", d[1])
	}

	// 3. 写日志的那条路径必须原样透传 status，不做零值兜底
	svc := readFile(t, filepath.Join(backend, "app", "service", "login_log_service.go"))
	if !strings.Contains(funcBody(svc, "func newLoginLogRecord("), "LoginStatus:   status,") {
		t.Error("newLoginLogRecord 没有把入参 status 原样写进 LoginStatus")
	}
	for _, forbidden := range []string{"if status == 0", "status = LoginStatusSuccess"} {
		if strings.Contains(svc, forbidden) {
			t.Errorf("login_log_service.go 里出现对状态的零值兜底 %q：失败状态会被改写", forbidden)
		}
	}

	// 4. 必须存在真正跑 GORM Create 的回归测试（只查 tag 字符串拦不住这个缺陷）
	regression := readFile(t, filepath.Join(backend, "app", "repo", "admin_user_login_log_repo_test.go"))
	for _, must := range []string{
		"DryRun",                               // 只构建 SQL、不连数据库
		"mysql.New(",                           // 用真实 MySQL dialector，不用 SQLite 替身
		"NewAdminUserLoginLogRepositoryWithDB", // 走真实仓储 Create，而不是直接调 gorm
		`db.Callback().Create().After("gorm:create")`, // 断言的是真实构建出的语句
		`cols["login_status"]`,
	} {
		if !strings.Contains(regression, must) {
			t.Errorf("生成的登录日志回归测试缺少 %q：失败状态的落库路径没被真正覆盖", must)
		}
	}
}

// loginStatusTagPattern 取出 model 里 LoginStatus 字段的整条 struct tag。
var loginStatusTagPattern = regexp.MustCompile("LoginStatus\\s+int8\\s+`([^`]*)`")

// loginStatusDDLPattern 取出建表语句里 login_status 的默认值。
var loginStatusDDLPattern = regexp.MustCompile("`login_status`\\s+TINYINT\\s+NOT NULL DEFAULT\\s+(\\S+)")

func assertNoDeadRegisterFlow(t *testing.T, outDir string) {
	t.Helper()

	backend := filepath.Join(outDir, "backend")
	for _, tc := range []struct {
		file    string
		needles []string
	}{
		{filepath.Join("app", "controller", "auth_controller.go"), []string{
			"func (c *AuthController) Register(",
			// controller 不得自己写一套 Token 提取优先级：登出必须吊销
			// 「请求真正通过认证的那个 Token」
			"func extractJWTTokenFromRequest(",
		}},
		{filepath.Join("app", "service", "auth_service.go"), []string{"func (s *AuthService) Register("}},
		{filepath.Join("app", "dto", "auth_dto.go"), []string{"type RegisterRequest struct"}},
	} {
		content := readFile(t, filepath.Join(backend, tc.file))
		for _, needle := range tc.needles {
			if strings.Contains(content, needle) {
				t.Errorf("%s 仍残留死代码 %q", tc.file, needle)
			}
		}
	}

	// 登出 / 刷新都必须复用中间件的提取顺序
	auth := readFile(t, filepath.Join(backend, "app", "controller", "auth_controller.go"))
	if strings.Count(auth, "middleware.GetJWTToken(r)") < 2 {
		t.Error("Logout 与 RefreshToken 都应复用 middleware.GetJWTToken(r)")
	}

	if ts := readFile(t, filepath.Join(outDir, "frontend", "src", "types", "auth.ts")); strings.Contains(ts, "RegisterRequest") {
		t.Error("frontend/src/types/auth.ts 仍残留 RegisterRequest 类型")
	}

	// 路由里本来就没挂注册接口，顺手钉住这个前提
	if routes := readFile(t, filepath.Join(backend, "app", "routes", "routes.go")); strings.Contains(routes, "/register") {
		t.Error("routes.go 出现 /register 路由：自助注册不在本底座范围内")
	}
}

// assertSQLLogNoLeak GORM 日志不得把插值后的 SQL 或原始数据库错误写进日志文件。
//
// Pi 在生成项目上实测复现过：慢查询与错误分支会把带明文凭证字段的插值 SQL
// 和原始错误一起落盘。行为由生成项目的 internal/taurus/daily_log_test.go
// 覆盖（含合成敏感 marker），这里只钉住结构，防止改回去。
func assertSQLLogNoLeak(t *testing.T, outDir string) {
	t.Helper()

	// 签名写全：下面要断言「函数体里不再出现原始 err」，参数列表不能混进来
	const traceSig = "func (l *DailyGormLogger) Trace(ctx context.Context, begin time.Time, " +
		"fc func() (sql string, rowsAffected int64), err error) {"
	trace := funcBody(readFile(t, filepath.Join(outDir, "backend", "internal", "taurus", "daily_log.go")), traceSig)
	if trace == "" {
		t.Fatal("daily_log.go 找不到 gorm logger 的 Trace 实现")
	}
	// fc() 的第一个返回值是插值 SQL：只能用来取语句类型，不能进日志行
	for _, line := range strings.Split(trace, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "sqlStr, rows := fc()" || trimmed == "verb := sqlVerb(sqlStr)" {
			continue
		}
		if strings.Contains(trimmed, "sqlStr") {
			t.Errorf("Trace 的 %q 引用了插值 SQL：语句原文不得进日志（会带出明文凭证等字段值）", trimmed)
		}
	}
	// 原始错误同理：只允许判空和过分类函数，err.Error() 会把参数值带出来
	sanitized := strings.NewReplacer("err != nil", "", "safeDBErrorClass(err)", "").Replace(trace)
	if regexp.MustCompile(`\berr\b`).MatchString(sanitized) {
		t.Error("Trace 除判空与 safeDBErrorClass 分类外不得再接触原始数据库错误")
	}
	if !strings.Contains(trace, "safeDBErrorClass(err)") {
		t.Error("Trace 的错误分支应只输出 safeDBErrorClass(err) 这类错误分类")
	}
	// 排障信息要保住：耗时、行数、语句类型
	for _, must := range []string{"[rows:%v]", "[op:%s]", "durationMillis(elapsed)"} {
		if !strings.Contains(trace, must) {
			t.Errorf("Trace 丢掉了排障信息 %q", must)
		}
	}
}

// funcBody 返回以 signature 开头的那个顶层函数的函数体（到行首 "}" 为止）。
func funcBody(src, signature string) string {
	i := strings.Index(src, signature)
	if i < 0 {
		return ""
	}
	rest := src[i+len(signature):]
	end := strings.Index(rest, "\n}")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// blockAfter 返回 marker 之后到下一行 "\t}" 为止的内容（匹配顶层缩进的收尾花括号）。
func blockAfter(src, marker string) string {
	i := strings.Index(src, marker)
	if i < 0 {
		return ""
	}
	rest := src[i+len(marker):]
	end := strings.Index(rest, "\n\t}")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// groupContaining 返回包含 needle 的那个 AddRouterGroup(...) 调用的源码片段。
func groupContaining(src, needle string) string {
	const marker = "taurus.Container.Http.AddRouterGroup(router.RouteGroup{"
	for start := 0; ; {
		i := strings.Index(src[start:], marker)
		if i < 0 {
			return ""
		}
		i += start
		rest := src[i:]
		end := strings.Index(rest, "\n\t})")
		if end < 0 {
			return ""
		}
		group := rest[:end]
		if strings.Contains(group, needle) {
			return group
		}
		start = i + end
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

	// 部门编码可空，但非空值仍受唯一约束。
	// 两半缺一不可：不可空 → 第二个不填编码的部门撞唯一索引建不出来；
	// 去掉唯一索引 → 修好了创建却丢了编码不重复的保证。
	deptCodeCol := regexp.MustCompile("`dept_code`\\s+VARCHAR\\(50\\)\\s+NULL DEFAULT NULL")
	if !deptCodeCol.MatchString(sql) {
		t.Error("admin_depts.dept_code 应为 NULL DEFAULT NULL（可空），否则第二个空编码部门建不出来")
	}
	if !strings.Contains(sql, "UNIQUE KEY `uk_dept_code` (`dept_code`)") {
		t.Error("admin_depts.dept_code 应保留 uk_dept_code 唯一约束")
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

// readTemplateFile 从 embed 的模板树里读一个模板的原文（未渲染）。
// 相对 templateRoot 给路径，例如 "backend/app/helper/crypto.go.tmpl"。
//
// 用来断言「模板里确实写着某个约定」，比渲染一整个项目再去 grep 便宜得多。
func readTemplateFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := fs.ReadFile(templates.FS, path.Join(templateRoot, rel))
	if err != nil {
		t.Fatalf("读取模板 %s 失败: %v", rel, err)
	}
	return string(data)
}

// TestPasswordHash 初始密码散列必须与生成项目 app/helper/crypto.go 的算法一致：
// bcrypt，cost 与两边共同约定的常量相同。
//
// 这里没有「已知答案」可比：bcrypt 自带随机盐，同一密码每次输出都不同。
// 所以断言换成三条等价强度的性质 —— 格式对、cost 对、能校验回原密码。
// 哪天有人把实现悄悄改回 SHA-256，第一条就会失败。
func TestPasswordHash(t *testing.T) {
	const password = "SyntheticPw123" // 合成口令，只用于本测试

	got, err := passwordHash(password)
	if err != nil {
		t.Fatalf("passwordHash 不该失败: %v", err)
	}

	if !strings.HasPrefix(got, "$2a$") && !strings.HasPrefix(got, "$2b$") && !strings.HasPrefix(got, "$2y$") {
		t.Fatalf("散列不是 bcrypt 格式: %q", got)
	}
	if len(got) != 60 {
		t.Fatalf("bcrypt 散列应为 60 字符，实际 %d: %q", len(got), got)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got), []byte(password)); err != nil {
		t.Fatalf("散列校验不过原密码: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got), []byte(password+"x")); err == nil {
		t.Fatal("错误密码不该通过校验")
	}

	// cost 必须就是约定的那个常量：cost 写错（比如被当成 4）散列照样能用，
	// 但抗离线爆破的强度会掉几个数量级，而且从外部完全看不出来。
	cost, err := bcrypt.Cost([]byte(got))
	if err != nil {
		t.Fatalf("读取 bcrypt cost 失败: %v", err)
	}
	if cost != bcryptCost {
		t.Errorf("bcrypt cost = %d，期望 %d", cost, bcryptCost)
	}
	if cost < 10 {
		t.Errorf("bcrypt cost 不得低于 10，实际 %d", cost)
	}

	// 自带随机盐：同一密码两次散列必须不同
	again, err := passwordHash(password)
	if err != nil {
		t.Fatalf("passwordHash 不该失败: %v", err)
	}
	if again == got {
		t.Fatal("两次散列结果相同，bcrypt 的随机盐没生效")
	}

	// 超过 72 字节必须报错而不是被静默截断
	if _, err := passwordHash(strings.Repeat("a", 73)); err == nil {
		t.Fatal("超过 72 字节的密码应当报错")
	}
}

// TestPasswordHashCostMatchesTemplate 生成器与生成项目必须用同一个 cost。
//
// 两边各写一个常量、靠注释约定「保持一致」是会漂的：init.sql 的初始散列
// 由生成器算，之后所有密码由后端算，cost 不一致不会报错，只会让强度悄悄分叉。
// 所以这里直接去模板文件里把那个常量读出来比。
func TestPasswordHashCostMatchesTemplate(t *testing.T) {
	src := readTemplateFile(t, "backend/app/helper/crypto.go.tmpl")
	if !strings.Contains(src, "const bcryptCost = bcrypt.DefaultCost") {
		t.Fatalf("生成项目 crypto.go 里找不到 `const bcryptCost = bcrypt.DefaultCost`：" +
			"两边 cost 的一致性无法再被这条测试保证，改动时请同步更新本断言")
	}
	if bcryptCost != bcrypt.DefaultCost {
		t.Errorf("生成器的 bcryptCost = %d，与模板里的 bcrypt.DefaultCost = %d 不一致",
			bcryptCost, bcrypt.DefaultCost)
	}
}

// TestRandomPassword 随机初始密码必须：每次不同、长度正确、
// 只含字母数字且字母与数字都至少出现一次。
//
// 最后一条是硬约束：生成项目的 helper.ValidatePasswordStrength 要求
// 6-20 位且字母数字齐全，初始密码过不了它自己的校验，用户首次改密就会被挡。
func TestRandomPassword(t *testing.T) {
	const length = 16
	const rounds = 200

	seen := make(map[string]bool, rounds)
	for i := 0; i < rounds; i++ {
		pw, err := randomPassword(length)
		if err != nil {
			t.Fatalf("生成随机密码失败: %v", err)
		}
		if len(pw) != length {
			t.Fatalf("密码长度应为 %d，实际 %d", length, len(pw))
		}
		if seen[pw] {
			t.Fatal("随机密码重复，熵不足或实现有误")
		}
		seen[pw] = true

		hasLetter, hasDigit := false, false
		for _, c := range pw {
			switch {
			case c >= '0' && c <= '9':
				hasDigit = true
			case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
				hasLetter = true
			default:
				t.Fatalf("密码含非字母数字字符: %q", c)
			}
		}
		if !hasLetter || !hasDigit {
			t.Fatal("密码必须同时包含字母和数字（生成项目的密码强度校验要求）")
		}
	}

	for _, bad := range []int{0, 7, 21} {
		if _, err := randomPassword(bad); err == nil {
			t.Errorf("长度 %d 应被拒绝", bad)
		}
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

// assertDockerCrossPlatform Dockerfile 必须按 BuildKit 的目标平台编译，不能写死架构。
// 固定 GOARCH=amd64 时镜像仍能 build 成功（Go 交叉编译），故障推迟到容器启动才暴露
// （arm64 宿主上 exec format error），所以这里按文本钉死四条约束。
func assertDockerCrossPlatform(t *testing.T, outDir string) {
	t.Helper()

	dockerfile := readFile(t, filepath.Join(outDir, "Dockerfile"))

	// 1. 不得写死目标架构：这正是修复前的故障形态
	if regexp.MustCompile(`GOARCH=(amd64|arm64|386|arm)\b`).MatchString(dockerfile) {
		t.Error("Dockerfile 写死了 GOARCH：跨架构宿主上 build 会通过但容器启动即 exec format error")
	}
	// 同理也不能把 GOARCH 的默认值写成某个具体架构（${TARGETARCH:-amd64}）
	if regexp.MustCompile(`TARGETARCH:[-=]\w`).MatchString(dockerfile) {
		t.Error("Dockerfile 给 TARGETARCH 写了具体默认架构：BuildKit 被关掉时会悄悄编出错架构的二进制")
	}

	// 2. 后端编译阶段必须声明并使用 BuildKit 注入的目标平台变量
	for _, want := range []string{"ARG TARGETOS", "ARG TARGETARCH", "GOARCH=${TARGETARCH}"} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile 缺少跨平台编译所需的 %q", want)
		}
	}

	// 3. builder 阶段原生跑（免 QEMU 模拟），前后端两个 builder 都要
	for _, stage := range []string{"AS frontend-builder", "AS backend-builder"} {
		line := dockerfileStageLine(t, dockerfile, stage)
		if !strings.Contains(line, "--platform=$BUILDPLATFORM") {
			t.Errorf("builder 阶段 %q 未使用 --platform=$BUILDPLATFORM：%s", stage, line)
		}
	}

	// 4. 运行时阶段必须跟随目标平台，绝不能钉在 BUILDPLATFORM
	//    （否则交叉构建时 rootfs 跟着构建机，又和二进制架构对不上）
	runtime := dockerfileStageLine(t, dockerfile, "FROM alpine:")
	if strings.Contains(runtime, "--platform") {
		t.Errorf("运行时阶段不应固定 platform，须跟随构建目标：%s", runtime)
	}
}

// dockerfileStageLine 取出包含 marker 的那一行 FROM 指令。
func dockerfileStageLine(t *testing.T, dockerfile, marker string) string {
	t.Helper()

	for _, line := range strings.Split(dockerfile, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "FROM ") && strings.Contains(trimmed, marker) {
			return trimmed
		}
	}
	t.Fatalf("Dockerfile 里找不到 %q 对应的 FROM 阶段", marker)
	return ""
}

// assertComposeNoFixedPlatform Compose 不得用写死 platform 的方式绕过架构问题：
// 架构由 Dockerfile 的 TARGET* 决定，compose 里固定 platform 会掩盖真实不匹配。
func assertComposeNoFixedPlatform(t *testing.T, outDir string) {
	t.Helper()

	compose := readFile(t, filepath.Join(outDir, "docker-compose.yml"))
	if regexp.MustCompile(`(?m)^\s*platform:`).MatchString(compose) {
		t.Error("docker-compose.yml 出现 platform:：不得用强制 platform 绕过架构不匹配")
	}
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

// TestBuildTemplateData_CarriesNoPlaintextPassword 渲染上下文里不能有明文密码。
//
// 这是一道比「检查生成产物」更靠前的防线。assertInitialSecrets 已经在断言
// 明文不出现在任何生成文件里，但那是**事后**检查：只要 templateData 上挂着
// 明文字段，以后任何人往某个 .tmpl 里引用它都会立刻把明文写进生成产物
// （而生成产物通常会被提交进使用者的版本库）。所以明文由 buildTemplateData
// 单独返回，不进渲染上下文 —— 这里用反射把「上下文里一个字段都不等于明文」钉住。
func TestBuildTemplateData_CarriesNoPlaintextPassword(t *testing.T) {
	opts := Options{ProjectName: "demo-ctx", Module: "example.com/demo-ctx"}
	data, plaintext, err := buildTemplateData(opts, normalizeComponents(nil))
	if err != nil {
		t.Fatalf("buildTemplateData 失败: %v", err)
	}
	if plaintext == "" {
		t.Fatal("buildTemplateData 没有返回初始管理员明文密码：CLI 无从提示用户")
	}
	// 散列必须真能校验出这个明文，否则按提示登录会失败
	if err := bcrypt.CompareHashAndPassword([]byte(data.AdminPasswordHash), []byte(plaintext)); err != nil {
		t.Errorf("AdminPasswordHash 校验不过返回的明文: %v", err)
	}

	v := reflect.ValueOf(*data)
	ty := v.Type()
	for i := 0; i < ty.NumField(); i++ {
		if v.Field(i).Kind() != reflect.String {
			continue
		}
		if v.Field(i).String() == plaintext {
			t.Errorf("templateData.%s 带着明文初始密码：模板一旦引用它，明文就进生成文件了", ty.Field(i).Name)
		}
	}
}

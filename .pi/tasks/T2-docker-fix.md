# Docker交付问题调查及窄修

## 授权与基线

用户同意查明MinIO拉取失败、确定方案后由Claude Code修复并重新验证。不含commit/push/部署。基线4031f55+未提交T2；不回滚既有改动。正式编码执行器Claude Code opus，新任务新会话。当前先只读调查，调查结论明确后固化最小修法再编码。

## 已观察

全选生成、Compose config与真实Docker镜像build通过，Compose up因minio/minio:RELEASE.2024-08-17T01-24-54Z pull access denied退出1，未进入启动阶段。Dockerfile硬编码GOARCH=amd64，而Docker Server为arm64。MinIO是否下架/迁移须以公开registry响应确认，不依赖搜索推断。Milvus v2.4.9架构兼容也需核实。

## 调查结论与本轮编码方案

- 官方DockerHub和Quay原MinIO tag匿名认证后manifest均401，无法确认可用来源，不能断定下架，不改镜像引用。官方Milvus发布附件依赖版本不同；任何替代版本仍待公开可用性核实和方案决定。
- Milvus v2.4.9公开manifest支持linux/amd64和linux/arm64。
- Claude Code只读调查成功，实际claude-opus-5，session f2c2bce3-3be2-4689-a820-937e69f57b9e。
- 本轮仅修确定的跨平台问题：frontend/backend builder采用BUILDPLATFORM，backend阶段声明TARGETOS/TARGETARCH并按目标编译；runtime跟随目标，不固定Compose platform。增加三组合生成Dockerfile回归断言；README/部署文档说明原生架构、buildx交叉构建与运行验证区别。保留非root及日志挂载权限既有约束，不扩大配置改造。
- 先测试及独立实际Docker构建/启动验证架构修法；MinIO整套交付阻塞另列，不因架构修复而宣布全选通过。

## 范围

Dockerfile跨平台构建、Compose官方依赖镜像来源及相应生成器回归测试、必要README说明。优先保持依赖版本与业务行为，不用latest/不明第三方镜像，不静默换存储产品或强制platform绕过。若只能升级较大版本或自建依赖镜像，回到用户决策。

## 禁止

不改admin、模块缓存/全局系统；不读取.env/.env.local/account/state或docker认证配置；不泄漏Token/密码。不做此前拦截的并发诊断。故障停止不自动重试/切换执行器。不commit/push/发布。

## 验证

独立检查完整diff，go vet/test（三组合）；新CLI生成、新Docker镜像构建及仅127.0.0.1独立端口的Compose启动验收。变更后旧运行证据不能冒充当前候选。按真实健康/连接判定，未测如实写明；结束清理本轮资源。

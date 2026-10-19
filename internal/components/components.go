// Package components 维护 taurus-x 内置的 Taurus Pro 组件注册表。
//
// 注册表是「组件 → 包路径/版本/是否必选」的唯一数据源：CLI 交互、go.mod
// 渲染、internal/taurus 的 Provide* 生成、docker-compose 服务生成都从这里取值，
// 避免版本号散落在多处模板里。
package components

// Component 一个可接入的 Taurus Pro 组件。
type Component struct {
	// Name 组件短名，同时是模板里的条件判断键（如 "consul"）。
	Name string
	// Package Go module 路径。
	Package string
	// Version 固定版本号，生成的 go.mod 直接使用。
	Version string
	// Required 必选组件，交互时显示为已选且不可取消。
	Required bool
	// Description 交互列表里展示的说明。
	Description string
}

// All 全部组件，顺序即交互列表顺序：先 4 个必选，再 5 个可选。
var All = []Component{
	{
		Name:        "common",
		Package:     "github.com/stones-hub/taurus-pro-common",
		Version:     "v0.2.16",
		Required:    true,
		Description: "通用能力：日志 logx、定时任务 cron、panic recovery",
	},
	{
		Name:        "config",
		Package:     "github.com/stones-hub/taurus-pro-config",
		Version:     "v0.0.4",
		Required:    true,
		Description: "配置加载：config/ 目录 + .env 环境变量占位符",
	},
	{
		Name:        "http",
		Package:     "github.com/stones-hub/taurus-pro-http",
		Version:     "v0.1.6",
		Required:    true,
		Description: "HTTP 服务：路由注册、中间件、优雅关闭",
	},
	{
		Name:        "storage",
		Package:     "github.com/stones-hub/taurus-pro-storage",
		Version:     "v0.1.35",
		Required:    true,
		Description: "存储：GORM 多库 + Redis（后台底座依赖）",
	},
	{
		Name:        "consul",
		Package:     "github.com/stones-hub/taurus-pro-consul",
		Version:     "v0.1.2",
		Description: "Consul：服务注册/注销 + 配置监听",
	},
	{
		Name:        "opentelemetry",
		Package:     "github.com/stones-hub/taurus-pro-opentelemetry",
		Version:     "v0.0.2",
		Description: "OpenTelemetry：Tracer Provider，关闭时 flush",
	},
	{
		Name:        "tcp",
		Package:     "github.com/stones-hub/taurus-pro-tcp",
		Version:     "v0.0.2",
		Description: "TCP 服务端：独立端口，与 HTTP 一起启动和关闭",
	},
	{
		Name:        "grpc",
		Package:     "github.com/stones-hub/taurus-pro-grpc",
		Version:     "v0.0.2",
		Description: "gRPC 服务端：独立端口，与 HTTP 一起启动和关闭",
	},
	{
		Name:        "milvus",
		Package:     "github.com/stones-hub/taurus-pro-milvus",
		Version:     "v0.0.7",
		Description: "Milvus 向量数据库客户端池",
	},
}

// RequiredNames 必选组件名列表。
func RequiredNames() []string {
	names := make([]string, 0, len(All))
	for _, c := range All {
		if c.Required {
			names = append(names, c.Name)
		}
	}
	return names
}

// OptionalNames 可选组件名列表。
func OptionalNames() []string {
	names := make([]string, 0, len(All))
	for _, c := range All {
		if !c.Required {
			names = append(names, c.Name)
		}
	}
	return names
}

// AllNames 全部组件名列表。
func AllNames() []string {
	names := make([]string, 0, len(All))
	for _, c := range All {
		names = append(names, c.Name)
	}
	return names
}

// Find 按名字查组件。
func Find(name string) (Component, bool) {
	for _, c := range All {
		if c.Name == name {
			return c, true
		}
	}
	return Component{}, false
}

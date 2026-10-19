// Package templates 把项目模板目录以 embed FS 的形式提供给生成器。
//
// 必须用 `all:` 前缀，否则 `.gitignore`、`.env.example`、`.dockerignore`
// 这类以点开头的文件不会被打进二进制。
package templates

import "embed"

//go:embed all:project
var FS embed.FS

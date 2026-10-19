// taurus-x 是一个生成 Go + Vue 后台底座项目的命令行脚手架。
//
//	taurus-x new <项目名>   交互式选择组件后生成项目
//	taurus-x version       输出版本号
package main

import (
	"os"

	"github.com/stones-hub/taurus-x/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}

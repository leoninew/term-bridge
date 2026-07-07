package main

import (
	"os"

	"gitee.com/leoninew/TermBridge-go/cmd/termbridge/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	mode := flag.String("mode", "unicode", "output mode: unicode, long-output, slow-output, exit-code")
	lines := flag.Int("lines", 100000, "number of lines for output modes")
	exitCode := flag.Int("code", 0, "exit code for exit-code mode")
	flag.Parse()

	switch *mode {
	case "unicode":
		fmt.Println("TERM_BRIDGE_UNICODE_BEGIN")
		fmt.Println("你好，TermBridge")
		fmt.Println("English ASCII")
		fmt.Println("\x1b[31mANSI_RED_TEXT\x1b[0m")
		fmt.Println("TERM_BRIDGE_UNICODE_END")
	case "long-output":
		fmt.Println("TERM_BRIDGE_LONG_OUTPUT_BEGIN")
		for i := range *lines {
			fmt.Printf("TERM_BRIDGE_LONG_OUTPUT_LINE_%06d\n", i)
		}
		fmt.Println("TERM_BRIDGE_LONG_OUTPUT_END")
	case "slow-output":
		fmt.Println("TERM_BRIDGE_SLOW_OUTPUT_BEGIN")
		for i := range *lines {
			fmt.Printf("TERM_BRIDGE_SLOW_OUTPUT_LINE_%06d\n", i)
			time.Sleep(50 * time.Millisecond)
		}
		fmt.Println("TERM_BRIDGE_SLOW_OUTPUT_END")
	case "exit-code":
		fmt.Println("TERM_BRIDGE_EXIT_CODE_" + strconv.Itoa(*exitCode))
		os.Exit(*exitCode)
	default:
		fmt.Fprintln(os.Stderr, "unknown mode: "+*mode)
		os.Exit(2)
	}
}

//go:build windows && !cli

package main

import "os"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--cli" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		desktopCLI()
		return
	}
	windowsGUIMain()
}

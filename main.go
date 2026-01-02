package main

import "github.com/victor/yaktui/cmd"

var (
	version = "0.1.0"
)

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}


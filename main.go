package main

import "github.com/nlink-jp/ask-gemini-mcp/cmd"

var version = "dev"

func main() {
	cmd.Execute(version)
}

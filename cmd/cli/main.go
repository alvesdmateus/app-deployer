package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("app-deployer CLI v0.1.0")
	fmt.Println("Usage:")
	fmt.Println("  deployer deploy <repo-url>    Deploy an application from repository")
	fmt.Println("  deployer list                 List all deployments")
	fmt.Println("  deployer logs <id>            Stream deployment logs")
	fmt.Println("  deployer destroy <id>         Destroy a deployment")
	fmt.Println("  deployer rollback <id>        Rollback a deployment")
	fmt.Println()
	fmt.Println("Not implemented yet. Coming in Phase 3.")
	os.Exit(0)
}

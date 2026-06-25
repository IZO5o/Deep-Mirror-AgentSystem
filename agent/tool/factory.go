package tool

import (
	"log"
	"os/exec"
)

// checkDockerAvailable checks if docker command is available
func checkDockerAvailable() bool {
	// Use `docker ps` to check if docker daemon is running
	// `docker ps` will fail if docker daemon is not running
	cmd := exec.Command("docker", "ps")
	return cmd.Run() == nil
}

// CreateBashTool creates a bash tool, automatically choosing between
// DockerBashTool (if docker is available) and regular BashTool.
func CreateBashTool(workspaceDir string) Tool {
	if !checkDockerAvailable() {
		log.Printf("Docker not available, using regular bash tool")
		return NewBashTool()
	}
	if workspaceDir == "" {
		log.Printf("Docker available but workspace dir is empty, using regular bash tool")
		return NewBashTool()
	}
	containerName := generateContainerName(workspaceDir)
	log.Printf("Docker available, using DockerBashTool with sandbox container '%s'", containerName)
	return NewDockerBashTool("", workspaceDir)
}

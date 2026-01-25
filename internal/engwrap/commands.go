package engwrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateOptions holds options for the Create command
type CreateOptions struct {
	ConfigPath    string
	Template      string
	ContainerName string
	Interactive   bool
}

func Create(ctx context.Context, dc *DockerClient, opts CreateOptions) error {
	var configPath string

	// determine config path
	if opts.Template != "" {
		// load from template
		templatePath, err := GetTemplatePath(opts.Template)
		if err != nil {
			return err
		}
		configPath = templatePath
	} else if opts.ConfigPath != "" {
		configPath = opts.ConfigPath
	} else {
		return fmt.Errorf("either config path or template must be specified")
	}

	// load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	// override name if specified
	if opts.ContainerName != "" {
		config.Name = opts.ContainerName
	}

	fmt.Printf("Creating container '%s'\n", config.Name)
	fmt.Printf("  Image: %s\n", config.Image)
	workspacePath, err := config.GetWorkspacePath()
	if err != nil {
		return err
	}
	fmt.Printf("  Workspace: %s\n", workspacePath)

	// check if container already exists
	exists, container, err := dc.ContainerExists(ctx, config.Name)
	if err != nil {
		return err
	}

	if exists {
		status, err := dc.GetContainerStatus(ctx, container.ID)
		if err != nil {
			return err
		}

		if status == "paused" {
			fmt.Printf("Container '%s' already exists and is paused. Use 'engwrap enter %s' to resume.\n", config.Name, config.Name)
			return nil
		}
		if status == "running" {
			fmt.Printf("Container '%s' is already running.\n", config.Name)
			return nil
		}
		// if exists but is not running, start it
		fmt.Printf("Container '%s' exists but is not running. Starting.\n", config.Name)
		if err := dc.StartContainer(ctx, container.ID); err != nil {
			return err
		}
		fmt.Printf("Container '%s' started successfully!\n", config.Name)
		return nil
	}

	// get workspace path (always $HOME/.engwrap/work/<name>)
	workspacePath, err = config.GetWorkspacePath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(workspacePath, 0700); err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}

	logsDir := filepath.Join(workspacePath, "logs")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return fmt.Errorf("creating logs directory: %w", err)
	}

	if err := dc.PullImage(ctx, config.Image); err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}

	// create container
	containerID, err := dc.CreateContainer(ctx, config.Name, config.Image, workspacePath, config.Name, config.NetworkMode, config.Mounts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	// create .bashrc with custom prompt
	if err := dc.SetupBashrc(ctx, containerID, config.Name, config.BashrcCustoms); err != nil {
		fmt.Printf("Warning: error configuring prompt: %v\n", err)
	}

	// execute initial commands
	if len(config.InitCommands) > 0 {
		fmt.Println("Executing initial commands")
		for _, cmd := range config.InitCommands {
			parts := strings.Fields(cmd)
			if len(parts) == 0 {
				continue
			}
			fmt.Printf("  Executing: %s\n", cmd)
			if err := dc.ExecCommand(ctx, containerID, parts); err != nil {
				fmt.Printf("Warning: command '%s' exited with error: %v\n", cmd, err)
			}
		}
	}

	// start logging
	logPath := filepath.Join(logsDir, "docker.log")

	_, err = dc.StartLogging(ctx, containerID, logPath)
	if err != nil {
		fmt.Printf("Warning: error starting logging: %v\n", err)
	}

	fmt.Printf("Container '%s' created and started successfully!\n", config.Name)
	fmt.Printf("  ID: %s\n", containerID[:12])
	fmt.Printf("  Logs: %s\n", logsDir)

	// if interactive flag is set, enter the container immediately
	if opts.Interactive {
		fmt.Println()
		return Enter(ctx, dc, config.Name)
	}

	return nil
}

func Enter(ctx context.Context, dc *DockerClient, name string) error {
	// check docker container
	exists, container, err := dc.ContainerExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("environment '%s' not found. Use 'engwrap create' to create it", name)
	}

	// handle container state
	// For Enter, we might need current precise state.
	status, err := dc.GetContainerStatus(ctx, container.ID)
	if err != nil {
		return err
	}

	if status == "paused" {
		fmt.Printf("Resuming container '%s'\n", name)
		if err := dc.UnpauseContainer(ctx, container.ID); err != nil {
			return err
		}
	} else if status != "running" {
		fmt.Printf("Starting container '%s'\n", name)
		if err := dc.StartContainer(ctx, container.ID); err != nil {
			return err
		}
	}

	// get workspace path from container
	workspacePath, err := dc.GetContainerWorkspacePath(ctx, container.ID)
	if err != nil {
		return err
	}

	// create logs directory if it doesn't exist
	logsDir := filepath.Join(workspacePath, "logs")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return fmt.Errorf("creating logs directory: %w", err)
	}

	// start interactive shell
	cmdlogPath := filepath.Join(logsDir, "cmdlog.log")
	sessionPath := filepath.Join(logsDir, fmt.Sprintf("session-%s.typescript", time.Now().Format("20060102-150405")))

	return dc.ExecInteractive(ctx, container.ID, cmdlogPath, sessionPath)
}

func Stop(ctx context.Context, dc *DockerClient, name string) error {
	// check docker container
	exists, container, err := dc.ContainerExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("environment '%s' not found", name)
	}

	// check status
	status, err := dc.GetContainerStatus(ctx, container.ID)
	if err != nil {
		return err
	}

	if status == "paused" {
		fmt.Printf("Container '%s' is already paused.\n", name)
		return nil
	}

	if status != "running" {
		return fmt.Errorf("container '%s' is not running (status: %s)", name, status)
	}

	// pause container
	if err := dc.PauseContainer(ctx, container.ID); err != nil {
		return err
	}

	fmt.Printf("Container '%s' paused.\n", name)
	return nil
}

func List(ctx context.Context, dc *DockerClient) error {
	// list containers with labels
	// ListEngwrapContainers now returns []container.Summary
	containers, err := dc.ListEngwrapContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	if len(containers) == 0 {
		fmt.Println("No environments configured.")
		return nil
	}

	fmt.Printf("%-15s %-30s %-60s %-10s\n", "NAME", "IMAGE", "PATH", "STATUS")

	for _, c := range containers {
		// Calculate fields from summary to avoid N+1 scans

		// Name
		name := c.Labels["engwrap.env"]
		if name == "" {
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			} else {
				name = "unknown"
			}
		}

		// Image
		image := c.Image

		// Workspace Path
		workspacePath := "unknown"
		for _, m := range c.Mounts {
			if m.Destination == "/workspace" {
				workspacePath = m.Source
				break
			}
		}

		// Status
		status := strings.ToLower(c.State) // "running", "paused", "exited"

		var color string
		switch status {
		case "running":
			color = ColorGreen
		case "paused":
			color = ColorYellow
		default:
			color = ColorRed
		}

		fmt.Printf("%-15s %-30s %-60s %s%-10s%s\n",
			name, image, workspacePath,
			color, strings.ToUpper(status), ColorReset)
	}

	return nil
}

func Destroy(ctx context.Context, dc *DockerClient, name string) error {
	// check docker container
	exists, container, err := dc.ContainerExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("environment '%s' not found", name)
	}

	// check status
	status, err := dc.GetContainerStatus(ctx, container.ID)
	if err != nil {
		return err
	}

	// stop container if running or paused
	if status == "running" || status == "paused" {
		if status == "paused" {
			// unpause before stopping
			if err := dc.UnpauseContainer(ctx, container.ID); err != nil {
				return err
			}
		}
		fmt.Printf("Stopping container '%s'\n", name)
		if err := dc.StopContainer(ctx, container.ID); err != nil {
			return err
		}
	}

	// remove container
	fmt.Printf("Removing container '%s'\n", name)
	if err := dc.RemoveContainer(ctx, container.ID, true); err != nil {
		return err
	}

	fmt.Printf("Container '%s' removed successfully.\n", name)
	return nil
}

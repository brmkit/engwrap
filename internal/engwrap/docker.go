package engwrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
)

//go:embed scripts/*
var scriptsFS embed.FS

type DockerClient struct {
	cli *client.Client
}

func NewDockerClient() (*DockerClient, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	if os.Getenv("DOCKER_HOST") == "" {
		if host := resolveDockerContextHost(); host != "" {
			opts = append(opts, client.WithHost(host))
		}
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	return &DockerClient{
		cli: cli,
	}, nil
}

// resolveDockerContextHost reads the active Docker CLI context to find the
// daemon endpoint. Returns "" if the context is "default" or unreadable.
//
// This mirrors docker/cli's on-disk context store (the sha256-of-name meta
// directory and meta.json shape); if that layout changes upstream this falls
// back to "" and the default socket. It also honors the same precedence the
// CLI uses when DOCKER_HOST is unset: DOCKER_CONTEXT > config.json currentContext.
func resolveDockerContextHost() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	ctxName := os.Getenv("DOCKER_CONTEXT")
	if ctxName == "" {
		data, err := os.ReadFile(filepath.Join(home, ".docker", "config.json"))
		if err != nil {
			return ""
		}
		var cfg struct {
			CurrentContext string `json:"currentContext"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return ""
		}
		ctxName = cfg.CurrentContext
	}

	if ctxName == "" || ctxName == "default" {
		return ""
	}

	hash := sha256.Sum256([]byte(ctxName))
	ctxDir := hex.EncodeToString(hash[:])
	metaPath := filepath.Join(home, ".docker", "contexts", "meta", ctxDir, "meta.json")

	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return ""
	}

	var meta struct {
		Endpoints map[string]struct {
			Host string `json:"Host"`
		} `json:"Endpoints"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return ""
	}

	if ep, ok := meta.Endpoints["docker"]; ok && ep.Host != "" {
		return ep.Host
	}
	return ""
}

func (dc *DockerClient) Close() error {
	return dc.cli.Close()
}

func (dc *DockerClient) PullImage(ctx context.Context, imageName string) error {
	// check if the image exists locally using ImageInspect.
	_, err := dc.cli.ImageInspect(ctx, imageName)
	if err == nil {
		fmt.Printf("Image %s already present locally\n", imageName)
		return nil
	}

	if !errdefs.IsNotFound(err) {
		return err
	}

	fmt.Printf("Pulling image %s ", imageName)
	reader, err := dc.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// use jsonmessage to display progress bars
	termFd, isTerm := term.GetFdInfo(os.Stdout)
	stream := &writerAdapter{Writer: os.Stdout, fd: termFd, isTerm: isTerm}
	if err := jsonmessage.DisplayJSONMessagesToStream(reader, stream, nil); err != nil {
		return fmt.Errorf("displaying progress: %w", err)
	}

	return nil
}

type writerAdapter struct {
	io.Writer
	fd     uintptr
	isTerm bool
}

func (w *writerAdapter) FD() uintptr {
	return w.fd
}

func (w *writerAdapter) IsTerminal() bool {
	return w.isTerm
}

func (dc *DockerClient) ContainerExists(ctx context.Context, name string) (bool, *container.Summary, error) {
	// use server-side filtering
	args := filters.NewArgs()
	args.Add("name", fmt.Sprintf("^/%s$", name)) // Exact match anchor

	containers, err := dc.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return false, nil, err
	}

	if len(containers) > 0 {
		return true, &containers[0], nil
	}

	return false, nil, nil
}

// GetContainerStatus uses data from Summary if available, or falls back to Inspect
func (dc *DockerClient) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	info, err := dc.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	if info.State.Paused {
		return "paused", nil
	}
	if info.State.Running {
		return "running", nil
	}
	return info.State.Status, nil
}

func (dc *DockerClient) GetContainerImage(ctx context.Context, containerID string) (string, error) {
	info, err := dc.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	return info.Config.Image, nil
}

func (dc *DockerClient) GetContainerWorkspacePath(ctx context.Context, containerID string) (string, error) {
	info, err := dc.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	for _, mount := range info.Mounts {
		if mount.Destination == "/workspace" {
			return mount.Source, nil
		}
	}

	return "", fmt.Errorf("workspace mount not found for container %s", containerID)
}

func (dc *DockerClient) CreateContainer(ctx context.Context, name, image, workspacePath, envName string, networkMode string, extraMounts []string) (string, error) {
	logsDir := filepath.Join(workspacePath, "logs")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return "", fmt.Errorf("creating logs directory: %w", err)
	}

	envVars := []string{fmt.Sprintf("ENV_NAME=%s", envName)}

	labels := map[string]string{
		"engwrap.managed": "true",
		"engwrap.env":     envName,
	}

	config := &container.Config{
		Image:        image,
		Cmd:          []string{"tail", "-f", "/dev/null"},
		WorkingDir:   "/workspace",
		Tty:          true,
		OpenStdin:    true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          envVars,
		Labels:       labels,
	}

	mountsConfig := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: workspacePath,
			Target: "/workspace",
		},
	}

	for _, m := range extraMounts {
		parts := strings.Split(m, ":")
		if len(parts) != 2 {
			fmt.Printf("Warning: invalid mount format '%s', skipping.\n", m)
			continue
		}
		source, target := parts[0], parts[1]

		absSource, err := filepath.Abs(source)
		if err != nil {
			fmt.Printf("Warning: invalid mount path '%s': %v\n", source, err)
			continue
		}

		mountsConfig = append(mountsConfig, mount.Mount{
			Type:     mount.TypeBind,
			Source:   absSource,
			Target:   target,
			ReadOnly: true,
		})
	}

	hostConfig := &container.HostConfig{
		Mounts:      mountsConfig,
		NetworkMode: container.NetworkMode(networkMode),
	}

	resp, err := dc.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return "", err
	}

	if err := dc.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (dc *DockerClient) StartContainer(ctx context.Context, containerID string) error {
	return dc.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (dc *DockerClient) PauseContainer(ctx context.Context, containerID string) error {
	return dc.cli.ContainerPause(ctx, containerID)
}

func (dc *DockerClient) UnpauseContainer(ctx context.Context, containerID string) error {
	return dc.cli.ContainerUnpause(ctx, containerID)
}

func (dc *DockerClient) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10
	return dc.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (dc *DockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	options := container.RemoveOptions{
		Force: force,
	}
	return dc.cli.ContainerRemove(ctx, containerID, options)
}

func (dc *DockerClient) SetupBashrc(ctx context.Context, containerID, envName string, bashrcCustoms []string) error {
	if len(bashrcCustoms) == 0 {
		return nil
	}

	var bashrcLines []string
	bashrcLines = append(bashrcLines, "# engwrap shell configuration")
	bashrcLines = append(bashrcLines, bashrcCustoms...)
	bashrcContent := strings.Join(bashrcLines, "\n") + "\n"

	heredocDelim := "ENWRAP_BASHRC_EOF"
	bashrcCmd := []string{"sh", "-c", fmt.Sprintf("cat > /root/.bashrc <<'%s'\n%s%s\n", heredocDelim, bashrcContent, heredocDelim)}

	// use simplified internal exec
	return dc.execInternal(ctx, containerID, bashrcCmd, false)
}

// execInternal handles unified execution logic
func (dc *DockerClient) execInternal(ctx context.Context, containerID string, cmd []string, streamOutput bool) error {
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	execResp, err := dc.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}

	attachResp, err := dc.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return err
	}
	defer attachResp.Close()

	if err := dc.cli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{}); err != nil {
		return err
	}

	// writers policy
	var stdout, stderr io.Writer
	if streamOutput {
		stdout = os.Stdout
		stderr = os.Stderr
	} else {
		stdout = io.Discard
		stderr = io.Discard
	}

	// drain exec output - always
	if _, err := stdcopy.StdCopy(stdout, stderr, attachResp.Reader); err != nil {
		return err
	}

	inspect, err := dc.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return err
	}

	if inspect.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}

	return nil
}

func (dc *DockerClient) ExecCommand(ctx context.Context, containerID string, cmd []string) error {
	return dc.execInternal(ctx, containerID, cmd, true)
}

func (dc *DockerClient) execCommandSilent(ctx context.Context, containerID string, cmd []string) error {
	return dc.execInternal(ctx, containerID, cmd, false)
}

// StartLogging starts tailing logs in a goroutine.
func (dc *DockerClient) StartLogging(ctx context.Context, containerID, logPath string) (func(), error) {
	done := make(chan struct{})

	go func() {
		defer close(done)

		reader, err := dc.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
		})
		if err != nil {
			if ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "Error starting logging: %v\n", err)
			}
			return
		}
		defer reader.Close()

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			return
		}
		defer file.Close()

		if _, err := io.Copy(file, reader); err != nil {
			if ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "Error writing log: %v\n", err)
			}
		}
	}()

	return func() {
		<-done
	}, nil
}

func (dc *DockerClient) getAvailableShell(ctx context.Context, containerID string) string {
	checkBash := []string{"sh", "-c", "command -v bash >/dev/null 2>&1"}
	if err := dc.execCommandSilent(ctx, containerID, checkBash); err == nil {
		return "bash"
	}
	return "sh"
}

func (dc *DockerClient) ExecInteractive(ctx context.Context, containerID string, cmdlogPath, sessionPath string) error {
	shell := dc.getAvailableShell(ctx, containerID)

	// Open session log file on HOST
	sessionFile, err := os.OpenFile(sessionPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening session log file: %w", err)
	}
	defer sessionFile.Close()

	cmdlogFileInContainer := "/workspace/logs/cmdlog.log"

	data := struct {
		CmdLogPath string
	}{
		CmdLogPath: cmdlogFileInContainer,
	}

	var tmplPath string
	if shell == "bash" {
		tmplPath = "scripts/wrapper_bash.sh"
	} else {
		tmplPath = "scripts/wrapper_sh.sh"
	}

	tmplContent, err := scriptsFS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("reading embedded script %s: %w", tmplPath, err)
	}

	t, err := template.New("script").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	var scriptBuf bytes.Buffer
	if err := t.Execute(&scriptBuf, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	execCmd := []string{shell, "-c", scriptBuf.String()}

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		Cmd:          execCmd,
	}

	execResp, err := dc.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}

	attachResp, err := dc.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{Tty: true})
	if err != nil {
		return err
	}
	defer attachResp.Close()

	if isTerminal(os.Stdin.Fd()) {
		oldState, err := makeRaw(os.Stdin.Fd())
		if err != nil {
			return fmt.Errorf("configuring terminal: %w", err)
		}
		defer func() {
			restoreTerminal(os.Stdin.Fd(), oldState)
		}()
	}

	if err := dc.cli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{Tty: true}); err != nil {
		return err
	}

	// Ooutput logic: Write to stdout AND session file
	outputWriter := io.MultiWriter(os.Stdout, sessionFile)

	done := make(chan error, 2)
	go func() {
		// capture stdout + stderr (merged by Tty: true) and write to both
		_, err := io.Copy(outputWriter, attachResp.Reader)
		done <- err
	}()
	go func() {
		_, err := io.Copy(attachResp.Conn, os.Stdin)
		done <- err
	}()

	<-done
	time.Sleep(100 * time.Millisecond)

	return nil
}

// ListEngwrapContainers returns full summary of containers
func (dc *DockerClient) ListEngwrapContainers(ctx context.Context) ([]container.Summary, error) {
	args := filters.NewArgs()
	args.Add("label", "engwrap.managed=true")

	containers, err := dc.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	return containers, nil
}

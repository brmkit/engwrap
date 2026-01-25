package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"engwrap/internal/engwrap"
)

var cli struct {
	Debug bool `help:"Enable debug mode."`

	Create struct {
		Template    string `help:"Select template source" short:"t" default:"kali-rolling"`
		Name        string `help:"Set specific container name" short:"n"`
		Interactive bool   `help:"Start in interactive mode" short:"i"`
		ConfigFile  string `arg:"" optional:"" help:"Optional config file path" type:"path"`
	} `cmd:"" help:"Create and start a new environment."`

	Enter struct {
		Name string `arg:"" required:"" help:"Name of the environment"`
	} `cmd:"" help:"Spawn a shell in an existing environment."`

	Stop struct {
		Name string `arg:"" required:"" help:"Name of the environment"`
	} `cmd:"" help:"Stop a running environment."`

	Destroy struct {
		Name string `arg:"" required:"" help:"Name of the environment"`
	} `cmd:"" help:"Permanently remove an environment."`

	Archive struct {
		Name string `arg:"" required:"" help:"Name of the environment"`
		Out  string `arg:"" required:"" help:"Output filename"`
	} `cmd:"" help:"Export workspace to file."`

	List struct {
	} `cmd:"" help:"List all configured environments."`

	Template struct {
		List struct {
		} `cmd:"" help:"List available templates"`

		Add struct {
			Path string `arg:"" required:"" help:"Template path"`
		} `cmd:"" help:"Add a new template"`
	} `cmd:"" help:"Manage templates"`
}

func main() {
	// Create context that cancels on SIGINT or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("   (       (  (  (  (   (      )        ")
	fmt.Println("  ))\\ (    )\\))( )\\))(  )(  ( /( \\`  )   ")
	fmt.Println(" /((_))\\ )((_))\\((_)()\\(()\\ )(_))/(/(   ")
	fmt.Println("(_)) _(_/( (()(_)(()((_)((_|(_)_((_)_\\  ")
	fmt.Println("/ -_) ' \\)\\) _` |\\ V  V / '_/ _` | '_ \\) ")
	fmt.Println("\\___|_||_|\\__, | \\_/\\_/|_| \\__,_| .__/  ")
	fmt.Println("          |___/                 |_|     ")
	fmt.Println("------------------------------------------------")

	// Parse CLI
	kctx := kong.Parse(&cli,
		kong.Name("engwrap"),
		kong.Description("A simple docker wrapper to manage red team environments."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}),
	)

	// Initialize directories
	if err := engwrap.InitializeEngwrapDirs(); err != nil {
		kctx.FatalIfErrorf(err)
	}

	// Commands that don't need Docker client
	switch kctx.Command() {
	case "archive <name> <out>":
		if err := engwrap.Archive(cli.Archive.Name, cli.Archive.Out); err != nil {
			kctx.FatalIfErrorf(err)
		}
		return
	case "template list":
		if err := engwrap.ListTemplates(); err != nil {
			kctx.FatalIfErrorf(err)
		}
		return
	case "template add <path>":
		if err := engwrap.AddTemplate(cli.Template.Add.Path); err != nil {
			kctx.FatalIfErrorf(err)
		}
		return
	}

	// for other commands, we need Docker client
	dc, err := engwrap.NewDockerClient()
	if err != nil {
		kctx.FatalIfErrorf(fmt.Errorf("creating docker client: %w", err))
	}
	defer dc.Close()

	// Dispatch commands requiring docker
	switch kctx.Command() {
	case "create", "create <config-file>":
		opts := engwrap.CreateOptions{
			Template:      cli.Create.Template,
			ContainerName: cli.Create.Name,
			Interactive:   cli.Create.Interactive,
			ConfigPath:    cli.Create.ConfigFile,
		}
		if err := engwrap.Create(ctx, dc, opts); err != nil {
			kctx.FatalIfErrorf(err)
		}
	case "enter <name>":
		if err := engwrap.Enter(ctx, dc, cli.Enter.Name); err != nil {
			kctx.FatalIfErrorf(err)
		}
	case "stop <name>":
		if err := engwrap.Stop(ctx, dc, cli.Stop.Name); err != nil {
			kctx.FatalIfErrorf(err)
		}
	case "list":
		if err := engwrap.List(ctx, dc); err != nil {
			kctx.FatalIfErrorf(err)
		}
	case "destroy <name>":
		if err := engwrap.Destroy(ctx, dc, cli.Destroy.Name); err != nil {
			kctx.FatalIfErrorf(err)
		}
	default:
		kctx.FatalIfErrorf(fmt.Errorf("unknown command %s", kctx.Command()))
	}
}

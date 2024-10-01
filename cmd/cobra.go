package kenginecmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/khulnasoft/kengine/v2"
)

var defaultFactory = newRootCommandFactory(func() *cobra.Command {
	return &cobra.Command{
		Use: "kengine",
		Long: `Kengine is an extensible server platform written in Go.

At its core, Kengine merely manages configuration. Modules are plugged
in statically at compile-time to provide useful functionality. Kengine's
standard distribution includes common modules to serve HTTP, TLS,
and PKI applications, including the automation of certificates.

To run Kengine, use:

	- 'kengine run' to run Kengine in the foreground (recommended).
	- 'kengine start' to start Kengine in the background; only do this
	  if you will be keeping the terminal window open until you run
	  'kengine stop' to close the server.

When Kengine is started, it opens a locally-bound administrative socket
to which configuration can be POSTed via a restful HTTP API (see
https://khulnasoft.com/docs/api).

Kengine's native configuration format is JSON. However, config adapters
can be used to convert other config formats to JSON when Kengine receives
its configuration. The Kenginefile is a built-in config adapter that is
popular for hand-written configurations due to its straightforward
syntax (see https://khulnasoft.com/docs/kenginefile). Many third-party
adapters are available (see https://khulnasoft.com/docs/config-adapters).
Use 'kengine adapt' to see how a config translates to JSON.

For convenience, the CLI can act as an HTTP client to give Kengine its
initial configuration for you. If a file named Kenginefile is in the
current working directory, it will do this automatically. Otherwise,
you can use the --config flag to specify the path to a config file.

Some special-purpose subcommands build and load a configuration file
for you directly from command line input; for example:

	- kengine file-server
	- kengine reverse-proxy
	- kengine respond

These commands disable the administration endpoint because their
configuration is specified solely on the command line.

In general, the most common way to run Kengine is simply:

	$ kengine run

Or, with a configuration file:

	$ kengine run --config kengine.json

If running interactively in a terminal, running Kengine in the
background may be more convenient:

	$ kengine start
	...
	$ kengine stop

This allows you to run other commands while Kengine stays running.
Be sure to stop Kengine before you close the terminal!

Depending on the system, Kengine may need permission to bind to low
ports. One way to do this on Linux is to use setcap:

	$ sudo setcap cap_net_bind_service=+ep $(which kengine)

Remember to run that command again after replacing the binary.

See the Kengine website for tutorials, configuration structure,
syntax, and module documentation: https://khulnasoft.com/docs/

Custom Kengine builds are available on the Kengine download page at:
https://khulnasoft.com/download

The xkengine command can be used to build Kengine from source with or
without additional plugins: https://github.com/khulnasoft/xkengine

Where possible, Kengine should be installed using officially-supported
package installers: https://khulnasoft.com/docs/install

Instructions for running Kengine in production are also available:
https://khulnasoft.com/docs/running
`,
		Example: `  $ kengine run
  $ kengine run --config kengine.json
  $ kengine reload --config kengine.json
  $ kengine stop`,

		// kind of annoying to have all the help text printed out if
		// kengine has an error provisioning its modules, for instance...
		SilenceUsage: true,
		Version:      onlyVersionText(),
	}
})

const fullDocsFooter = `Full documentation is available at:
https://khulnasoft.com/docs/command-line`

func init() {
	defaultFactory.Use(func(rootCmd *cobra.Command) {
		rootCmd.SetVersionTemplate("{{.Version}}\n")
		rootCmd.SetHelpTemplate(rootCmd.HelpTemplate() + "\n" + fullDocsFooter + "\n")
	})
}

func onlyVersionText() string {
	_, f := kengine.Version()
	return f
}

func kengineCmdToCobra(kengineCmd Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   kengineCmd.Name + " " + kengineCmd.Usage,
		Short: kengineCmd.Short,
		Long:  kengineCmd.Long,
	}
	if kengineCmd.CobraFunc != nil {
		kengineCmd.CobraFunc(cmd)
	} else {
		cmd.RunE = WrapCommandFuncForCobra(kengineCmd.Func)
		cmd.Flags().AddGoFlagSet(kengineCmd.Flags)
	}
	return cmd
}

// WrapCommandFuncForCobra wraps a Kengine CommandFunc for use
// in a cobra command's RunE field.
func WrapCommandFuncForCobra(f CommandFunc) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		status, err := f(Flags{cmd.Flags()})
		if status > 1 {
			cmd.SilenceErrors = true
			return &exitError{ExitCode: status, Err: err}
		}
		return err
	}
}

// exitError carries the exit code from CommandFunc to Main()
type exitError struct {
	ExitCode int
	Err      error
}

func (e *exitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exiting with status %d", e.ExitCode)
	}
	return e.Err.Error()
}

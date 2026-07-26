package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/smallstep/truststore"
)

// runCertCommand implements `nullsec-server cert install|uninstall`, mirroring
// Hetty's approach: instead of asking the user to manually import the CA
// certificate into every browser/OS trust store, drive smallstep/truststore
// to do it for them (system store).
func runCertCommand(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: server cert <install|uninstall> [flags]")
		os.Exit(1)
	}

	sub := args[0]
	fs := flag.NewFlagSet("cert "+sub, flag.ExitOnError)
	certPath := fs.String("cert", defaultCACertPath(), "Path to CA certificate")
	skipSystem := fs.Bool("skip-system", false, "Skip the system trust store")
	_ = fs.Parse(args[1:]) // flag.ExitOnError already exits the process on a parse error

	var opts []truststore.Option
	if *skipSystem {
		opts = append(opts, truststore.WithNoSystem())
	}

	switch sub {
	case "install":
		if !*skipSystem {
			fmt.Println("You may be prompted for your password to install the certificate system-wide.")
		}
		if err := truststore.InstallFile(*certPath, opts...); err != nil {
			fmt.Fprintln(os.Stderr, "failed to install certificate:", err)
			os.Exit(1)
		}
		fmt.Println("Certificate installed.")
	case "uninstall":
		if err := truststore.UninstallFile(*certPath, opts...); err != nil {
			fmt.Fprintln(os.Stderr, "failed to uninstall certificate:", err)
			os.Exit(1)
		}
		fmt.Println("Certificate uninstalled.")
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", sub)
		os.Exit(1)
	}
}

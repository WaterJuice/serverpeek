// ---------------------------------------------------------------------------------------
//
//	cli.go
//	------
//
//	CLI argument parsing, help text, and server launch. Provides a single command
//	that starts the monitoring dashboard HTTP server.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created (Python)
//	Mar 2026 - Rewritten in Go
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

const licenceText = `serverpeek — Released under the Unlicense (public domain)

This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

For more information, please refer to <https://unlicense.org/>
`

// ---------------------------------------------------------------------------------------
//
//	Run
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// Run is the main entry point called from main.go.
func Run(version string) {
	host := "0.0.0.0"
	port := 8080

	args := os.Args[1:]

	if len(args) == 0 {
		startServerCmd(version, host, port)
		return
	}

	i := 0
	for i < len(args) {
		switch args[i] {
		case "-h", "--help":
			printUsage()
			return
		case "--version":
			fmt.Printf("serverpeek: %s\n", version)
			return
		case "--license":
			fmt.Print(licenceText)
			return
		case "-p", "--port":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --port requires a value")
				os.Exit(1)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: --port must be a number")
				os.Exit(1)
			}
			port = n
		case "-H", "--host":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --host requires a value")
				os.Exit(1)
			}
			host = args[i]
		default:
			fmt.Fprintf(os.Stderr, "Unknown option: %s\n", args[i])
			fmt.Fprintln(os.Stderr, "Run with --help for usage")
			os.Exit(1)
		}
		i++
	}

	startServerCmd(version, host, port)
}

// ---------------------------------------------------------------------------------------
//
//	Help Text
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// printUsage prints the help text with TTY-aware ANSI colour codes.
func printUsage() {
	tty := isTerminal(os.Stdout)
	if tty {
		h := "\033[1;34m"
		p := "\033[1;35m"
		s := "\033[32m"
		l := "\033[36m"
		m := "\033[33m"
		S := "\033[1;32m"
		L := "\033[1;36m"
		M := "\033[1;33m"
		r := "\033[0m"

		fmt.Printf("%susage: %s%sserverpeek%s [%s-h%s] [%s--version%s] [%s--license%s] [%s-H%s %sHOST%s] [%s-p%s %sPORT%s]\n",
			h, r, p, r, s, r, l, r, l, r, s, r, m, r, s, r, m, r)
		fmt.Println()
		fmt.Println("Live-updating web dashboard for server monitoring.")
		fmt.Println()
		fmt.Printf("%soptions:%s\n", h, r)
		fmt.Printf("  %s-h%s, %s--help%s             show this help message and exit\n", S, r, L, r)
		fmt.Printf("  %s--version%s              show version and exit\n", L, r)
		fmt.Printf("  %s--license%s              show licence information and exit\n", L, r)
		fmt.Printf("  %s-H%s, %s--host%s %sHOST%s       address to bind to (default: 0.0.0.0)\n", S, r, L, r, M, r)
		fmt.Printf("  %s-p%s, %s--port%s %sPORT%s       port to listen on (default: 8080)\n", S, r, L, r, M, r)
	} else {
		fmt.Println("usage: serverpeek [-h] [--version] [--license] [-H HOST] [-p PORT]")
		fmt.Println()
		fmt.Println("Live-updating web dashboard for server monitoring.")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help             show this help message and exit")
		fmt.Println("  --version              show version and exit")
		fmt.Println("  --license              show licence information and exit")
		fmt.Println("  -H, --host HOST        address to bind to (default: 0.0.0.0)")
		fmt.Println("  -p, --port PORT        port to listen on (default: 8080)")
	}
}

// ---------------------------------------------------------------------------------------
//
//	Server Startup
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// startServerCmd prints the startup banner and launches the HTTP server.
func startServerCmd(version string, host string, port int) {
	isTTY := isTerminal(os.Stderr)

	logInfo(isTTY, "serverpeek %s", version)

	if host == "0.0.0.0" || host == "::" {
		logInfo(isTTY, "Listening on all interfaces, port %d", port)
		logInfo(isTTY, "  http://localhost:%d", port)
		hostname, err := os.Hostname()
		if err == nil && hostname != "" && hostname != "localhost" {
			hostname = stripDomain(hostname)
			logInfo(isTTY, "  http://%s:%d", hostname, port)
		}
	} else {
		logInfo(isTTY, "Serving on http://%s:%d", host, port)
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	startServer(addr, isTTY)
}

// ---------------------------------------------------------------------------------------
//
//	Terminal / Colour Helpers
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// isTerminal reports whether the given file descriptor is connected to a terminal (TTY).
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ---------------------------------------------------------------------------------------
// logInfo prints a message to stderr, dimmed when connected to a TTY.
func logInfo(isTTY bool, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if isTTY {
		fmt.Fprintf(os.Stderr, "\033[2m%s\033[0m\n", msg)
	} else {
		fmt.Fprintln(os.Stderr, msg)
	}
}

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yegors/co-atc/internal/auth"
)

// printNewUser reads a password twice without echoing it and prints the TOML block
// to paste into the configuration.
//
// It prints rather than writes. Rewriting the configuration file would mean a
// program editing the document that carries every measured default and the reason
// for it; and a password that has been typed should reach exactly one place, the
// hash, with no copy left in a backup file along the way.
func printNewUser(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a user name is required")
	}

	first, err := readPassword("Password for " + name + ": ")
	if err != nil {
		return err
	}
	again, err := readPassword("Repeat: ")
	if err != nil {
		return err
	}
	if first != again {
		return fmt.Errorf("the two entries differ")
	}

	hash, err := auth.HashPassword(first)
	if err != nil {
		return err
	}

	fmt.Printf("\n# Add this to your configuration, under [auth]:\n\n")
	fmt.Printf("[auth]\nenabled = true\n\n[[auth.users]]\nname = %q\npassword_hash = %q\n\n", name, hash)
	fmt.Printf("# The password itself is not stored anywhere. Losing it means creating\n")
	fmt.Printf("# another account with this command; there is no recovery by design.\n")
	return nil
}

// stdinReader is shared across reads. A fresh bufio.Reader per call would lose
// whatever the previous one had already buffered, which makes the second prompt
// read EOF when the input is piped rather than typed.
var stdinReader = bufio.NewReader(os.Stdin)

var warnedAboutEcho bool

// readPassword turns off terminal echo through stty rather than pulling in a
// dependency for it. If echo cannot be turned off -- a pipe, a terminal that does
// not support it -- the caller is told plainly rather than typing a password into
// a visible line without knowing.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	stty := exec.Command("stty", "-echo")
	stty.Stdin = os.Stdin
	echoOff := stty.Run() == nil
	if !echoOff && !warnedAboutEcho {
		warnedAboutEcho = true
		fmt.Fprint(os.Stderr, "\n[!] this terminal will not hide input; type with that in mind\n"+prompt)
	}

	line, err := stdinReader.ReadString('\n')

	if echoOff {
		restore := exec.Command("stty", "echo")
		restore.Stdin = os.Stdin
		restore.Run()
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

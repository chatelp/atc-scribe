package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yegors/co-atc/internal/auth"
	"golang.org/x/term"
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

	role, err := readRole()
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(first)
	if err != nil {
		return err
	}

	fmt.Printf("\n# Add this to your configuration, under [auth]:\n\n")
	fmt.Printf("[auth]\nenabled = true\n\n[[auth.users]]\nname = %q\npassword_hash = %q\n", name, hash)
	if role != auth.RoleOwner {
		// An owner is what an account without a role is; only the other one
		// is written, so the block for an owner reads as it always did.
		fmt.Printf("role = %q\n", role)
	}
	fmt.Println()
	fmt.Printf("# The password itself is not stored anywhere. Losing it means creating\n")
	fmt.Printf("# another account with this command; there is no recovery by design.\n")
	return nil
}

// readRole asks what the account may do, owner unless told otherwise: a
// viewer reads the map, the aircraft and the transcriptions, changes nothing,
// and is not given the audio unless server.audio_replay says so.
func readRole() (string, error) {
	fmt.Fprint(os.Stderr, "Role, owner or viewer (read-only) [owner]: ")
	line, err := stdinReader.ReadString('\n')
	if err != nil && !(err == io.EOF && line == "") {
		return "", err
	}
	return auth.NormalizeRole(strings.TrimRight(line, "\r\n"))
}

// stdinReader is shared across reads. A fresh bufio.Reader per call would lose
// whatever the previous one had already buffered, which makes the second prompt
// read EOF when the input is piped rather than typed.
var stdinReader = bufio.NewReader(os.Stdin)

var warnedAboutEcho bool

// readPassword reads a line without echoing it when the input is a terminal,
// through golang.org/x/term, which does so on every system the server builds for.
// If it is not -- a pipe -- the caller is told plainly rather than typing a
// password into a visible line without knowing.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		password, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	if !warnedAboutEcho {
		warnedAboutEcho = true
		fmt.Fprint(os.Stderr, "\n[!] this terminal will not hide input; type with that in mind\n"+prompt)
	}
	line, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

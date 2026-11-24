package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/storeurl"
)

// usersCmd adds, lists, removes and resets the local accounts an `agni serve --auth tokens
// --local-accounts` signs people in with (agni issue 1815). Like `agni token`, it works on the
// --store directly rather than through the server, so making an account takes access to the
// server's machine, and nobody can sign themselves up.
func usersCmd() *cobra.Command {
	var storeURL string
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Add, list, remove and reset the local accounts an agni serve --local-accounts signs people in with",
		Long: `users manages the local username and password accounts a shared agni serve keeps in its
--store. Run it on the server's machine, with the same --store the server was started with. A
person signs in at /signin with the username and password, and the server records them as
local:<username> on what they store.

  printf '%s\n' "$PASSWORD" | agni users add alice --store file:///srv/agni/meta --password-stdin
  agni users ls --store file:///srv/agni/meta
  printf '%s\n' "$NEW" | agni users passwd alice --store file:///srv/agni/meta --password-stdin
  agni users rm alice --store file:///srv/agni/meta

A username is 3 to 20 letters, digits, dashes or underscores, and case doesn't matter. A password
needs at least 8 characters. The store keeps only its bcrypt hash. Removing an account or changing
its password leaves the tokens it already signed in for working until they expire; agni token
revoke ends one sooner.`,
	}
	cmd.PersistentFlags().StringVar(&storeURL, "store", "", "the --store of the agni serve these accounts are for, such as file:///srv/agni/meta, sqlite:///srv/agni.db, postgres://agni@db/agni or datastore://project?namespace=agni (required)")
	// store opens --store's accounts, and closes the store when cmd ends, as agni token does.
	store := func(cmd *cobra.Command) (authgate.Accounts, error) {
		if storeURL == "" {
			return nil, fmt.Errorf("agni users needs --store, the same one the server was started with")
		}
		st, _, err := storeurl.Open(storeURL, "")
		if err != nil {
			return nil, err
		}
		cmd.PostRunE = func(*cobra.Command, []string) error { return st.Close() }
		return st.Accounts()
	}

	var addStdin, passwdStdin bool
	add := &cobra.Command{
		Use:   "add <name> --password-stdin",
		Short: "Make a local account, reading its password from stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword(cmd, addStdin)
			if err != nil {
				return err
			}
			s, err := store(cmd)
			if err != nil {
				return err
			}
			name, err := authgate.AddUser(cmd.Context(), s, args[0], password)
			if errors.Is(err, authgate.ErrUserExists) {
				return fmt.Errorf("%w; agni users passwd changes its password", err)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s, who signs in as local:%s\n", name, name)
			return nil
		},
	}
	add.Flags().BoolVar(&addStdin, "password-stdin", false, "read the password from the first line of stdin (required)")

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the local accounts by name, each with when it was made",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := store(cmd)
			if err != nil {
				return err
			}
			users, err := authgate.ListUsers(cmd.Context(), s)
			if err != nil {
				return err
			}
			for _, u := range users {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\tlocal:%s\t%s\n", u.Name, u.Name, u.CreatedAt.UTC().Format(time.RFC3339))
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a local account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store(cmd)
			if err != nil {
				return err
			}
			removed, err := authgate.RemoveUser(cmd.Context(), s, args[0])
			if err != nil {
				return err
			}
			if !removed {
				return fmt.Errorf("rm %s: %w", args[0], authgate.ErrNoSuchUser)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", strings.ToLower(args[0]))
			return nil
		},
	}

	passwd := &cobra.Command{
		Use:   "passwd <name> --password-stdin",
		Short: "Change a local account's password, reading the new one from stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword(cmd, passwdStdin)
			if err != nil {
				return err
			}
			s, err := store(cmd)
			if err != nil {
				return err
			}
			name, err := authgate.SetPassword(cmd.Context(), s, args[0], password)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "changed %s's password\n", name)
			return nil
		},
	}
	passwd.Flags().BoolVar(&passwdStdin, "password-stdin", false, "read the new password from the first line of stdin (required)")

	cmd.AddCommand(add, ls, rm, passwd)
	return cmd
}

// readPassword reads a password from the first line of cmd's stdin. The repo has no terminal
// helper that reads without echo, so a password comes only through --password-stdin, which keeps
// it out of the shell history and the process list.
func readPassword(cmd *cobra.Command, fromStdin bool) (string, error) {
	if !fromStdin {
		usage := "users " + cmd.Name()
		if cmd.Name() == "login" {
			usage = "login"
		}
		return "", fmt.Errorf("pass --password-stdin and give the password on stdin, such as printf '%%s\\n' \"$PASSWORD\" | agni %s ... --password-stdin", usage)
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading the password from stdin: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", errors.New("--password-stdin read an empty password")
	}
	return line, nil
}

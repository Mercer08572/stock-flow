package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Mercer08572/stock-flow/internal/auth"
	"github.com/Mercer08572/stock-flow/internal/shared/config"
	"github.com/Mercer08572/stock-flow/internal/shared/database"
)

const initialPasswordEnv = "ADMIN_INITIAL_PASSWORD"

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("admin command failed: %v", err)
	}
}

func run(args []string) error {
	return runWithDeps(args, os.Stdin, os.Stdout, newAdminService)
}

// adminServiceFactory lazily builds the auth service so that argument and password
// validation still run before any database connection is opened.
type adminServiceFactory func(ctx context.Context) (auth.Service, func(), error)

// runWithDeps parses the flags and reads the password before newService is called,
// so the service factory never has to open a database connection during argument tests.
func runWithDeps(args []string, stdin io.Reader, stdout io.Writer, newService adminServiceFactory) error {
	if len(args) == 0 || args[0] != "init" {
		return errors.New("usage: stock-flow-admin init [--username admin] [--password-stdin]")
	}
	flags := flag.NewFlagSet("admin init", flag.ContinueOnError)
	username := flags.String("username", "admin", "administrator username")
	passwordStdin := flags.Bool("password-stdin", false, "read the initial password from standard input")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	password, err := readInitialPassword(*passwordStdin, stdin)
	if err != nil {
		return err
	}

	ctx := context.Background()
	service, closeService, err := newService(ctx)
	if err != nil {
		return err
	}
	defer closeService()

	if err := service.InitializeAdminPassword(ctx, *username, password); err != nil {
		// The password is overwritten before the error is inspected, so never surface it.
		password = ""
		return describeInitializeError(*username, err)
	}
	fmt.Fprintf(stdout, "administrator %q initialized; password change is required at first login\n", strings.TrimSpace(*username))
	return nil
}

// describeInitializeError turns the repository outcomes into operator-facing guidance.
// It never echoes the password that was read.
func describeInitializeError(username string, err error) error {
	switch {
	case errors.Is(err, auth.ErrAdminAlreadyInitialized):
		return fmt.Errorf("administrator %q is already initialized; refusing to overwrite the existing password", strings.TrimSpace(username))
	case errors.Is(err, auth.ErrAdminNotFound):
		return fmt.Errorf("administrator %q not found; run the database migrations before initializing", strings.TrimSpace(username))
	default:
		return err
	}
}

func newAdminService(ctx context.Context) (auth.Service, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	service := auth.NewService(auth.NewPostgresRepository(db), auth.NewInMemorySessionStore(), auth.NewArgon2idPasswordHasher(auth.Argon2idParams{}), auth.ServiceOptions{})
	return service, db.Close, nil
}

func readInitialPassword(fromStdin bool, stdin io.Reader) (string, error) {
	if value := os.Getenv(initialPasswordEnv); value != "" {
		return value, nil
	}
	if !fromStdin {
		info, err := os.Stdin.Stat()
		if err != nil {
			return "", fmt.Errorf("inspect terminal: %w", err)
		}
		if info.Mode()&os.ModeCharDevice == 0 {
			return "", fmt.Errorf("set %s temporarily or use --password-stdin", initialPasswordEnv)
		}
		fmt.Fprint(os.Stderr, "Initial administrator password: ")
		if err := setTerminalEcho(false); err != nil {
			return "", err
		}
		defer func() { _ = setTerminalEcho(true); fmt.Fprintln(os.Stderr) }()
	}
	value, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read initial password: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r"), nil
}

func setTerminalEcho(enabled bool) error {
	mode := "-echo"
	if enabled {
		mode = "echo"
	}
	cmd := exec.Command("stty", mode)
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("configure terminal input: %w", err)
	}
	return nil
}

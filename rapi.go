package rapi

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rubiojr/rapi/backend"
	"github.com/rubiojr/rapi/backend/azure"
	"github.com/rubiojr/rapi/backend/b2"
	"github.com/rubiojr/rapi/backend/gs"
	"github.com/rubiojr/rapi/backend/local"
	"github.com/rubiojr/rapi/backend/location"
	"github.com/rubiojr/rapi/backend/rclone"
	"github.com/rubiojr/rapi/backend/rest"
	"github.com/rubiojr/rapi/backend/s3"
	"github.com/rubiojr/rapi/backend/sftp"
	"github.com/rubiojr/rapi/backend/swift"
	"github.com/rubiojr/rapi/internal/cache"
	"github.com/rubiojr/rapi/internal/debug"
	"github.com/rubiojr/rapi/internal/fs"
	"github.com/rubiojr/rapi/internal/limiter"
	"github.com/rubiojr/rapi/internal/options"
	"github.com/rubiojr/rapi/internal/textfile"
	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"

	"github.com/rubiojr/rapi/internal/errors"

	"os/exec"

	"golang.org/x/crypto/ssh/terminal"
)

// TimeFormat is the format used for all timestamps printed by restic.
const TimeFormat = "2006-01-02 15:04:05"

type backendWrapper func(r restic.Backend) (restic.Backend, error)

// ResticOptions hold all global options for restic.
type ResticOptions struct {
	Repo            string
	RepositoryFile  string
	PasswordFile    string
	PasswordCommand string
	KeyHint         string
	Quiet           bool
	Verbose         int
	NoLock          bool
	JSON            bool
	CacheDir        string
	NoCache         bool
	CACerts         []string
	TLSClientCert   string
	CleanupCache    bool

	LimitUploadKb   int
	LimitDownloadKb int

	ctx      context.Context
	Password string
	stdout   io.Writer
	stderr   io.Writer

	backendTestHook backendWrapper

	// verbosity is set as follows:
	//  0 means: don't print any messages except errors, this is used when --quiet is specified
	//  1 is the default: print essential messages
	//  2 means: print more messages, report minor things, this is used when --verbose is specified
	//  3 means: print very detailed debug messages, this is used when --verbose=2 is specified
	verbosity uint

	Options []string

	extended options.Options
}

var DefaultOptions = ResticOptions{
	stdout: os.Stdout,
	stderr: os.Stderr,
	ctx:    context.Background(),
}

func stdinIsTerminal() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd()))
}

func stdoutIsTerminal() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func stdoutTerminalWidth() int {
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0
	}
	return w
}

// ClearLine creates a platform dependent string to clear the current
// line, so it can be overwritten. ANSI sequences are not supported on
// current windows cmd shell.
func ClearLine() string {
	if runtime.GOOS == "windows" {
		if w := stdoutTerminalWidth(); w > 0 {
			return strings.Repeat(" ", w-1) + "\r"
		}
		return ""
	}
	return "\x1b[2K"
}

// Printf writes the message to the configured stdout stream.
func Printf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(DefaultOptions.stdout, format, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to stdout: %v\n", err)
	}
}

// Print writes the message to the configured stdout stream.
func Print(args ...interface{}) {
	_, err := fmt.Fprint(DefaultOptions.stdout, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to stdout: %v\n", err)
	}
}

// Println writes the message to the configured stdout stream.
func Println(args ...interface{}) {
	_, err := fmt.Fprintln(DefaultOptions.stdout, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to stdout: %v\n", err)
	}
}

// Verbosef calls Printf to write the message when the verbose flag is set.
func Verbosef(format string, args ...interface{}) {
	if DefaultOptions.verbosity >= 1 {
		Printf(format, args...)
	}
}

// Verboseff calls Printf to write the message when the verbosity is >= 2
func Verboseff(format string, args ...interface{}) {
	if DefaultOptions.verbosity >= 2 {
		Printf(format, args...)
	}
}

// PrintProgress wraps fmt.Printf to handle the difference in writing progress
// information to terminals and non-terminal stdout
func PrintProgress(format string, args ...interface{}) {
	var (
		message         string
		carriageControl string
	)
	message = fmt.Sprintf(format, args...)

	if !(strings.HasSuffix(message, "\r") || strings.HasSuffix(message, "\n")) {
		if stdoutIsTerminal() {
			carriageControl = "\r"
		} else {
			carriageControl = "\n"
		}
		message = fmt.Sprintf("%s%s", message, carriageControl)
	}

	if stdoutIsTerminal() {
		message = fmt.Sprintf("%s%s", ClearLine(), message)
	}

	fmt.Print(message)
}

// Warnf writes the message to the configured stderr stream.
func Warnf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(DefaultOptions.stderr, format, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to stderr: %v\n", err)
	}
}

// resolvePassword determines the password to be used for opening the repository.
func resolvePassword(opts ResticOptions, envStr string) (string, error) {
	if opts.PasswordFile != "" && opts.PasswordCommand != "" {
		return "", errors.Fatalf("Password file and command are mutually exclusive options")
	}
	if opts.PasswordCommand != "" {
		args, err := backend.SplitShellStrings(opts.PasswordCommand)
		if err != nil {
			return "", err
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return (strings.TrimSpace(string(output))), nil
	}
	if opts.PasswordFile != "" {
		s, err := textfile.Read(opts.PasswordFile)
		if os.IsNotExist(errors.Cause(err)) {
			return "", errors.Fatalf("%s does not exist", opts.PasswordFile)
		}
		return strings.TrimSpace(string(s)), errors.Wrap(err, "Readfile")
	}

	if pwd := os.Getenv(envStr); pwd != "" {
		return pwd, nil
	}

	return "", nil
}

// readPassword reads the password from the given reader directly.
func readPassword(in io.Reader) (password string, err error) {
	sc := bufio.NewScanner(in)
	sc.Scan()

	return sc.Text(), errors.Wrap(err, "Scan")
}

// readPasswordTerminal reads the password from the given reader which must be a
// tty. Prompt is printed on the writer out before attempting to read the
// password.
func readPasswordTerminal(in *os.File, out io.Writer, prompt string) (password string, err error) {
	fmt.Fprint(out, prompt)
	buf, err := terminal.ReadPassword(int(in.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", errors.Wrap(err, "ReadPassword")
	}

	password = string(buf)
	return password, nil
}

// ReadPassword reads the password from a password file, the environment
// variable RESTIC_PASSWORD or prompts the user.
func ReadPassword(opts ResticOptions, prompt string) (string, error) {
	if opts.Password != "" {
		return opts.Password, nil
	}

	var (
		password string
		err      error
	)

	if stdinIsTerminal() {
		password, err = readPasswordTerminal(os.Stdin, os.Stderr, prompt)
	} else {
		password, err = readPassword(os.Stdin)
		Verbosef("reading repository password from stdin\n")
	}

	if err != nil {
		return "", errors.Wrap(err, "unable to read password")
	}

	if len(password) == 0 {
		return "", errors.New("an empty password is not a password")
	}

	return password, nil
}

// ReadPasswordTwice calls ReadPassword two times and returns an error when the
// passwords don't match.
func ReadPasswordTwice(gopts ResticOptions, prompt1, prompt2 string) (string, error) {
	pw1, err := ReadPassword(gopts, prompt1)
	if err != nil {
		return "", err
	}
	if stdinIsTerminal() {
		pw2, err := ReadPassword(gopts, prompt2)
		if err != nil {
			return "", err
		}

		if pw1 != pw2 {
			return "", errors.Fatal("passwords do not match")
		}
	}

	return pw1, nil
}

func ReadRepo(opts ResticOptions) (string, error) {
	envRepo := os.Getenv("RESTIC_REPOSITORY")

	if opts.Repo == "" && opts.RepositoryFile == "" && envRepo == "" {
		return "", errors.Fatal("Please specify repository location (-r or --repository-file)")
	}

	if envRepo != "" {
		return envRepo, nil
	}

	repo := opts.Repo
	if opts.RepositoryFile != "" {
		if repo != "" {
			return "", errors.Fatal("Options -r and --repository-file are mutually exclusive, please specify only one")
		}

		s, err := textfile.Read(opts.RepositoryFile)
		if os.IsNotExist(errors.Cause(err)) {
			return "", errors.Fatalf("%s does not exist", opts.RepositoryFile)
		}
		if err != nil {
			return "", err
		}

		repo = strings.TrimSpace(string(s))
	}

	return repo, nil
}

const maxKeys = 20

// OpenRepository reads the password and opens the repository.
func OpenRepository(opts ResticOptions) (*repository.Repository, error) {
	repo, err := ReadRepo(opts)
	if err != nil {
		return nil, err
	}

	be, err := open(repo, opts, opts.extended)
	if err != nil {
		return nil, err
	}

	be = backend.NewRetryBackend(be, 10, func(msg string, err error, d time.Duration) {
		Warnf("%v returned error, retrying after %v: %v\n", msg, d, err)
	})

	// wrap backend if a test specified a hook
	if opts.backendTestHook != nil {
		be, err = opts.backendTestHook(be)
		if err != nil {
			return nil, err
		}
	}

	pwd, err := resolvePassword(opts, "RESTIC_PASSWORD")
	if err != nil {
		return nil, err
	}

	if pwd != "" {
		opts.Password = pwd
	}

	s := repository.New(be)

	passwordTriesLeft := 1
	if stdinIsTerminal() && opts.Password == "" {
		passwordTriesLeft = 3
	}

	for ; passwordTriesLeft > 0; passwordTriesLeft-- {
		opts.Password, err = ReadPassword(opts, "enter password for repository: ")
		if err != nil && passwordTriesLeft > 1 {
			opts.Password = ""
			fmt.Printf("%s. Try again\n", err)
		}
		if err != nil {
			continue
		}

		err = s.SearchKey(opts.ctx, opts.Password, maxKeys, opts.KeyHint)
		if err != nil && passwordTriesLeft > 1 {
			opts.Password = ""
			fmt.Printf("%s. Try again\n", err)
		}
	}
	if err != nil {
		if errors.IsFatal(err) {
			return nil, err
		}
		return nil, errors.Fatalf("%s", err)
	}

	if stdoutIsTerminal() && !opts.JSON {
		id := s.Config().ID
		if len(id) > 8 {
			id = id[:8]
		}
		if !opts.JSON {
			Verbosef("repository %v opened successfully, password is correct\n", id)
		}
	}

	if opts.NoCache {
		return s, nil
	}

	c, err := cache.New(s.Config().ID, opts.CacheDir)
	if err != nil {
		Warnf("unable to open cache: %v\n", err)
		return s, nil
	}

	if c.Created && !opts.JSON {
		Verbosef("created new cache in %v\n", c.Base)
	}

	// start using the cache
	s.UseCache(c)

	oldCacheDirs, err := cache.Old(c.Base)
	if err != nil {
		Warnf("unable to find old cache directories: %v", err)
	}

	// nothing more to do if no old cache dirs could be found
	if len(oldCacheDirs) == 0 {
		return s, nil
	}

	// cleanup old cache dirs if instructed to do so
	if opts.CleanupCache {
		Printf("removing %d old cache dirs from %v\n", len(oldCacheDirs), c.Base)

		for _, item := range oldCacheDirs {
			dir := filepath.Join(c.Base, item.Name())
			err = fs.RemoveAll(dir)
			if err != nil {
				Warnf("unable to remove %v: %v\n", dir, err)
			}
		}
	} else {
		if stdoutIsTerminal() {
			Verbosef("found %d old cache directories in %v, run `restic cache --cleanup` to remove them\n",
				len(oldCacheDirs), c.Base)
		}
	}

	return s, nil
}

func parseConfig(loc location.Location, opts options.Options) (interface{}, error) {
	// only apply options for a particular backend here
	opts = opts.Extract(loc.Scheme)

	switch loc.Scheme {
	case "local":
		cfg := loc.Config.(local.Config)
		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening local repository at %#v", cfg)
		return cfg, nil

	case "sftp":
		cfg := loc.Config.(sftp.Config)
		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening sftp repository at %#v", cfg)
		return cfg, nil

	case "s3":
		cfg := loc.Config.(s3.Config)
		if cfg.KeyID == "" {
			cfg.KeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		}

		if cfg.Secret == "" {
			cfg.Secret = os.Getenv("AWS_SECRET_ACCESS_KEY")
		}

		if cfg.Region == "" {
			cfg.Region = os.Getenv("AWS_DEFAULT_REGION")
		}

		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening s3 repository at %#v", cfg)
		return cfg, nil

	case "gs":
		cfg := loc.Config.(gs.Config)
		if cfg.ProjectID == "" {
			cfg.ProjectID = os.Getenv("GOOGLE_PROJECT_ID")
		}

		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening gs repository at %#v", cfg)
		return cfg, nil

	case "azure":
		cfg := loc.Config.(azure.Config)
		if cfg.AccountName == "" {
			cfg.AccountName = os.Getenv("AZURE_ACCOUNT_NAME")
		}

		if cfg.AccountKey == "" {
			cfg.AccountKey = os.Getenv("AZURE_ACCOUNT_KEY")
		}

		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening gs repository at %#v", cfg)
		return cfg, nil

	case "swift":
		cfg := loc.Config.(swift.Config)

		if err := swift.ApplyEnvironment("", &cfg); err != nil {
			return nil, err
		}

		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening swift repository at %#v", cfg)
		return cfg, nil

	case "b2":
		cfg := loc.Config.(b2.Config)

		if cfg.AccountID == "" {
			cfg.AccountID = os.Getenv("B2_ACCOUNT_ID")
		}

		if cfg.AccountID == "" {
			return nil, errors.Fatalf("unable to open B2 backend: Account ID ($B2_ACCOUNT_ID) is empty")
		}

		if cfg.Key == "" {
			cfg.Key = os.Getenv("B2_ACCOUNT_KEY")
		}

		if cfg.Key == "" {
			return nil, errors.Fatalf("unable to open B2 backend: Key ($B2_ACCOUNT_KEY) is empty")
		}

		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening b2 repository at %#v", cfg)
		return cfg, nil
	case "rest":
		cfg := loc.Config.(rest.Config)
		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening rest repository at %#v", cfg)
		return cfg, nil
	case "rclone":
		cfg := loc.Config.(rclone.Config)
		if err := opts.Apply(loc.Scheme, &cfg); err != nil {
			return nil, err
		}

		debug.Log("opening rest repository at %#v", cfg)
		return cfg, nil
	}

	return nil, errors.Fatalf("invalid backend: %q", loc.Scheme)
}

// Open the backend specified by a location config.
func open(s string, gopts ResticOptions, opts options.Options) (restic.Backend, error) {
	debug.Log("parsing location %v", location.StripPassword(s))
	loc, err := location.Parse(s)
	if err != nil {
		return nil, errors.Fatalf("parsing repository location failed: %v", err)
	}

	var be restic.Backend

	cfg, err := parseConfig(loc, opts)
	if err != nil {
		return nil, err
	}

	tropts := backend.TransportOptions{
		RootCertFilenames:        DefaultOptions.CACerts,
		TLSClientCertKeyFilename: DefaultOptions.TLSClientCert,
	}
	rt, err := backend.Transport(tropts)
	if err != nil {
		return nil, err
	}

	// wrap the transport so that the throughput via HTTP is limited
	lim := limiter.NewStaticLimiter(gopts.LimitUploadKb, gopts.LimitDownloadKb)
	rt = lim.Transport(rt)

	switch loc.Scheme {
	case "local":
		be, err = local.Open(cfg.(local.Config))
		// wrap the backend in a LimitBackend so that the throughput is limited
		be = limiter.LimitBackend(be, lim)
	case "sftp":
		be, err = sftp.Open(cfg.(sftp.Config))
		// wrap the backend in a LimitBackend so that the throughput is limited
		be = limiter.LimitBackend(be, lim)
	case "s3":
		be, err = s3.Open(cfg.(s3.Config), rt)
	case "gs":
		be, err = gs.Open(cfg.(gs.Config), rt)
	case "azure":
		be, err = azure.Open(cfg.(azure.Config), rt)
	case "swift":
		be, err = swift.Open(cfg.(swift.Config), rt)
	case "b2":
		be, err = b2.Open(DefaultOptions.ctx, cfg.(b2.Config), rt)
	case "rest":
		be, err = rest.Open(cfg.(rest.Config), rt)
	case "rclone":
		be, err = rclone.Open(cfg.(rclone.Config), lim)

	default:
		return nil, errors.Fatalf("invalid backend: %q", loc.Scheme)
	}

	if err != nil {
		return nil, errors.Fatalf("unable to open repo at %v: %v", location.StripPassword(s), err)
	}

	// check if config is there
	fi, err := be.Stat(DefaultOptions.ctx, restic.Handle{Type: restic.ConfigFile})
	if err != nil {
		return nil, errors.Fatalf("unable to open config file: %v\nIs there a repository at the following location?\n%v", err, location.StripPassword(s))
	}

	if fi.Size == 0 {
		return nil, errors.New("config file has zero size, invalid repository?")
	}

	return be, nil
}

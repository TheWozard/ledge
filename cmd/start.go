package cmd

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
	"ledge/internal/background"
	"ledge/internal/config"
	"ledge/internal/state"
)

// runFlags are the flags start and restart pass on to `ledge run`.
type runFlags struct {
	config   string
	port     int
	headless bool
}

func (f *runFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.config, "config", "", "window settings `PATH` (JSON; default: built-in defaults)")
	cmd.Flags().IntVar(&f.port, "port", 0, fmt.Sprintf("API port (default: $LEDGE_PORT or %d)", state.DefaultPort))
	cmd.Flags().BoolVar(&f.headless, "headless", false, "run only the API, with no windows")
}

// args is the flags as `ledge run` arguments. The background ledge runs in $HOME, so
// a relative config path is made absolute.
func (f *runFlags) args() ([]string, error) {
	var args []string
	if f.config != "" {
		path, err := filepath.Abs(f.config)
		if err != nil {
			return nil, err
		}
		args = append(args, "--config", path)
	}
	if f.port != 0 {
		args = append(args, "--port", strconv.Itoa(f.port))
	}
	if f.headless {
		args = append(args, "--headless")
	}
	return args, nil
}

var startFlags runFlags

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Run ledge in the background",
	Long: `Run ledge in the background, as ` + "`ledge run`" + ` does in a terminal, and wait for its
API to answer. Its output goes to ledge.log in the state dir. Does nothing if it's
already running; ` + "`ledge stop`" + ` stops it.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		args, err := startFlags.args()
		if err != nil {
			return err
		}
		return startWith(args)
	},
}

// startWith starts `ledge run args...` in the background.
func startWith(args []string) error {
	process, err := backgroundProcess(portIn(args))
	if err != nil {
		return err
	}
	if err := checkConfig(args); err != nil {
		return err
	}
	pid, started, err := process.Start(args)
	if err != nil {
		return err
	}
	if !started {
		fmt.Printf("ledge is already running (pid %d)\n", pid)
		return nil
	}
	fmt.Printf("ledge started (pid %d, port %d, log %s)\n", pid, process.Port, process.LogFile)
	return nil
}

// checkConfig loads the --config in `ledge run` arguments, so a bad config fails
// straight away with its own message, before anything is started or stopped.
func checkConfig(args []string) error {
	if i := slices.Index(args, "--config"); i >= 0 && i+1 < len(args) {
		_, err := config.Load(args[i+1])
		return err
	}
	return nil
}

// portIn is the --port in `ledge run` arguments, or 0 for none.
func portIn(args []string) int {
	if i := slices.Index(args, "--port"); i >= 0 && i+1 < len(args) {
		port, _ := strconv.Atoi(args[i+1])
		return port
	}
	return 0
}

// backgroundProcess is the background ledge on port (0 for $LEDGE_PORT or the default).
func backgroundProcess(port int) (*background.Process, error) {
	if port == 0 {
		var err error
		if port, err = state.Port(); err != nil {
			return nil, err
		}
	}
	dir, err := state.Dir()
	if err != nil {
		return nil, err
	}
	return background.New(dir, port)
}

func init() {
	startFlags.register(startCmd)
	rootCmd.AddCommand(startCmd)
}

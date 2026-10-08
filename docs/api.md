# Go API reference

Generated with `go doc -all .` using the Linux source view.
The exported API is shared across Linux, macOS and Windows.

```text
package graceproc // import "github.com/latticebuild/graceproc"

Package graceproc owns cancellation and cleanup of native tool invocations.

FUNCTIONS

func Run(cmd *exec.Cmd, signals <-chan os.Signal) (code int, err error)
    Run forwards cancellation to the child process group, allowing graceful
    shutdown before forcing termination. Before returning successfully,
    cleanup confirms that the owned Unix process group or Windows job has no
    live members, allowing callers to reuse their ports. Unix descendants that
    create a separate process group or session are outside that ownership.

func RunWithGrace(cmd *exec.Cmd, signals <-chan os.Signal, grace time.Duration) (code int, err error)
    RunWithGrace gives a supervising process more time than its child's own
    cleanup budget. Each of the two escalation stages lasts grace; the default
    Run policy remains three seconds per stage.

func SignalCode(signal os.Signal) int
    SignalCode returns the conventional exit status for cancellation.

func Signals() []os.Signal
    Signals are the supported cancellation signals on this platform.

```

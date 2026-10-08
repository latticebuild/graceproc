# Graceful process supervision

`graceproc` owns cancellation and descendant cleanup for native commands used by
build tools and CI helpers. It accepts an `os/exec.Cmd` and a signal channel, so
it has no Bazel, compiler, package-manager, or application dependency. Its
separate [Go module](go.mod) lets build toolkits share that boundary.

Ordinary command execution does not own a child's descendants. A worker may
retain a listening port after its parent exits, making the next server start
fail. Cleanup runs after ordinary command completion as well as cancellation,
and checks that owned processes have exited before returning without a cleanup
error. A successful child exit cannot hide a cleanup failure.

For cancellation, the supervisor forwards the signal and allows a grace period,
then repeats the signal before forcing cleanup. This gives tools that manage
their own descendants a chance to stop them. The [RunWithGrace API](graceproc.go)
lets an outer supervisor allow more time than its child's cleanup budget; the
caller chooses the allowance for its own workload.
The final cleanup confirmation has a separate bounded wait, independent of
the escalation grace period.

On Linux and macOS, ownership is a process group. A descendant that creates
another group or session escapes that boundary; the invoked tool must stop
those descendants itself. Cleanup needs the supervisor to remain alive and
cannot survive its forced termination. Cleanup ignores fully exited zombies,
but Linux treats a group with surviving threads as live, even after its
thread-group leader exits: those threads can retain ports.

On Windows, ownership is a Job Object with termination on its last handle's
closure. The command starts suspended, enters the job, and only then resumes,
preventing a fast child from spawning descendants before assignment. The job
remains open through command completion and cleanup. Cancellation sends a
console break event to the new process group; failed delivery triggers job
termination. The [Windows implementation](graceproc_windows.go) uses `x/sys/windows`
bindings and keeps the missing completion-port layout local. It associates a
private completion port before assignment and waits for that job's final process
notification under one cleanup deadline. An empty accounting count can precede
kernel resource release. Missing notifications and API failures remain cleanup
errors; startup failures before assignment close the inactive job directly.

The local layer keeps the shutdown policy and resource-release checks. Standard
`os/exec` and Go's `x/sys` bindings supply the process and OS interfaces. This
keeps OS-specific ownership here while callers retain their tool semantics.

## Verification

[Package tests](BUILD.bazel) exercise exit status, startup failures, cancellation
and port release after both normal completion and cancellation. The
[Linux cases](graceproc_linux_test.go) distinguish fully exited groups from an
exited leader with live sibling threads; [Darwin cases](graceproc_darwin_test.go)
accept exited, unreaped groups. The [Windows cases](graceproc_windows_test.go)
exercise job assignment before a fast parent's exit and cleanup of a descendant
holding a port. They require its held process handle to be signaled immediately
when supervision returns, then immediately rebind its port.

The [CI workflow](.github/workflows/ci.yml) executes these tests on native
Linux, macOS, and Windows runners. Native execution establishes OS behavior;
cross-compilation checks source compatibility. See [development](docs/development.md)
for the local build and check commands.

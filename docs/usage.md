# Using graceproc

Prepare an `os/exec.Cmd`, wire its standard streams, and pass a signal channel to
`Run`. The command must not already be started. Use `signal.Notify` with
`graceproc.Signals()...`, then stop the notification when supervision returns.
The [supervisor example](../examples/supervise/main.go) shows the full command.

`Run` allows three seconds for each cancellation stage. `RunWithGrace` accepts a
positive duration for each stage: it forwards cancellation, forwards again after
the first deadline, then forces cleanup after the second deadline. Cleanup also
runs when the child exits normally. Passing a nil channel disables signal input;
closing a channel stops listening without cancelling the command.

Always inspect both returned values. A normal child exit preserves its exit
status, including a nonzero status. An explicit cancellation returns
`SignalCode(signal)`; interrupt conventionally returns130. Start, supervision and
cleanup failures remain errors and turn a zero status into1. Use system signals
from `Signals` or `os.Interrupt` with `SignalCode`.

Unix ownership is a process group. A descendant that creates a different process
group or session needs cleanup from the tool that created it. Windows ownership
is a Job Object assigned before the primary thread starts. Successful return
confirms that the owned group or job has no live members; this lets callers reuse
ports after supervision.

## Bazel

Add `github.com/latticebuild/graceproc v0.1.0` to your caller-owned go.mod.
Then import its checksum-pinned Go dependency with
[Gazelle go_deps](https://github.com/bazel-contrib/bazel-gazelle/blob/master/extensions.md#go_deps)
in MODULE.bazel:

```starlark
bazel_dep(name = "rules_go", version = "0.63.0", repo_name = "io_bazel_rules_go")
bazel_dep(name = "gazelle", version = "0.51.3")
go_sdk = use_extension("@io_bazel_rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(version = "1.27.1")
go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//:go.mod")
use_repo(go_deps, "com_github_latticebuild_graceproc")
```

Run `go mod tidy` to update go.mod and go.sum. Use the imported library in BUILD.bazel:

```starlark
load("@io_bazel_rules_go//go:def.bzl", "go_binary")
go_binary(
    name = "supervise",
    srcs = ["main.go"],
    deps = ["@com_github_latticebuild_graceproc//:graceproc"],
)
```

The [external Go-module caller](../examples/bazel/) builds and executes all four
exported API examples using the public Go release. The [generated Go reference](api.md) lists
the complete API; [runnable examples](../examples/README.md) give the owning gates.

# Runnable examples

Prepare the repository with `mise run bootstrap`, then run:

```sh
bazel build //examples:artifacts
bazel test //examples:test
```

| Feature | Source | Command | Expected result |
| --- | --- | --- | --- |
| Default supervision and exit status | [supervise/main.go](supervise/main.go) | `bazel run //examples/supervise -- COMMAND ARG` | Runs the child and forwards interrupt signals; exits with the supervised status. |
| Custom grace budget and signal status | [api/api_test.go](api/api_test.go) | `bazel test //examples/api:api_test` | Run, RunWithGrace, Signals and SignalCode examples all execute. |
| Bazel consuming the public Go module | [bazel/](bazel/) | `cd examples/bazel && bazel test //:test` | All four exported API examples execute through checksum-pinned go_deps. |
| Descendant cleanup and escalation | [../graceproc_test.go](../graceproc_test.go) | `bazel test //:unit_test` | Existing native tests verify cancellation, cleanup and port reuse. |

The root artifact/test gates include the local examples; CI separately builds and
tests the external Bazel caller. Deliberately invalid subjects
remain in test fixtures; their owner tests require the expected refusals. Fix and
editor-write commands modify the invoking checkout only when run explicitly.
Automated mutation cases use disposable invoking workspaces.

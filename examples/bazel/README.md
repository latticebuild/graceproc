# Bazel Go-module example

This independent caller resolves `github.com/latticebuild/graceproc@v0.1.0`
through Gazelle `go_deps`, using its own go.mod and go.sum. It builds and executes
examples of Run, RunWithGrace, Signals and SignalCode against the public Go release.

```sh
go mod download
bazel build //:artifacts
bazel test //:test
```

The repository CI runs these commands on Linux, macOS27 and Windows.

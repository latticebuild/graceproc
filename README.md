# graceproc

[![CI](https://github.com/latticebuild/graceproc/actions/workflows/ci.yml/badge.svg)](https://github.com/latticebuild/graceproc/actions/workflows/ci.yml)
[![Bazel](https://img.shields.io/badge/Bazel-9.2.0-43A047?logo=bazel&logoColor=white)](MODULE.bazel)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Go process supervision with graceful cancellation and descendant cleanup on
Linux, macOS and Windows. Run accepts an os/exec.Cmd and a signal channel;
RunWithGrace lets the caller choose the cancellation grace period.

## Setup

Add the release to your Go module:

```sh
go get github.com/latticebuild/graceproc@v0.1.0
```

Bazel consumers use the latticebuild_graceproc module and the public
`@latticebuild_graceproc//:graceproc` library; see [MODULE.bazel](MODULE.bazel)
for toolchain and dependency versions.

## Usage

```go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "os/signal"

    "github.com/latticebuild/graceproc"
)

func main() {
    signals := make(chan os.Signal, 2)
    signal.Notify(signals, graceproc.Signals()...)
    cmd := exec.Command("node", "server.mjs")
    cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
    code, err := graceproc.Run(cmd, signals)
    signal.Stop(signals)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        if code == 0 {
            code = 1
        }
    }
    os.Exit(code)
}
```

Supervision returns after bounded cleanup, including when the parent exits
normally. Unix ownership is a process group; descendants that create another
group or session need cleanup by the tool that created them. Windows ownership
is a Job Object assigned before the child starts running. Cleanup failures
remain errors even if the child exits successfully.

<details>
<summary>Repository map</summary>

| Area | Location |
| --- | --- |
| Public package | [graceproc.go](graceproc.go) |
| Process tests | [graceproc_test.go](graceproc_test.go) |
| Development guide | [docs/development.md](docs/development.md) |

</details>

## Documentation and examples

See the [generated API reference](docs/README.md) and [runnable examples](examples/README.md).

## Development

Install [Mise](https://mise.jdx.dev/), then prepare this checkout:

```sh
mise trust
mise run bootstrap
hk validate
hk test
hk check --all --slow
bazel build //:artifacts
bazel test //:test
```

Tools and dependency versions are pinned in [mise.toml](mise.toml) and
[MODULE.bazel](MODULE.bazel). CI runs these gates on native Linux, macOS and
Windows runners. Repositories with a race suite also run it on Linux and macOS.
See [docs/development.md](docs/development.md) for owning checks and platform
constraints, and [ARCHITECTURE.md](ARCHITECTURE.md) for implementation decisions.

## License

[Apache License 2.0](LICENSE).

[Sponsor us](https://github.com/mathematic-inc) · [Discuss questions and ideas](https://github.com/latticebuild/graceproc/discussions)

Pull requests are limited to repository collaborators. Use Discussions for bugs,
feature requests and support. Changes merge as squash commits.

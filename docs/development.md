# Development

Install Mise, then run `mise trust` and `mise run bootstrap` in this checkout.

```sh
hk check --all --slow
bazel build //:artifacts
bazel test //:test
# Linux and macOS:
bazel test //:race_test
cd examples/bazel && bazel build //:artifacts && bazel test //:test
```

CI runs the same checks on Ubuntu 24.04, macOS 27, and Windows 2025 workers. Go race
variants retain their Windows incompatibility. Dependencies belong to the owned
lockfiles and MODULE configuration. The external example resolves Graceproc's
public Go release through go_deps; its Go API has the same version for every build system.

On Windows, CI creates LOCALAPPDATA/Temp/latticebuild before Mise installs
tools. This uses a canonical long path on the installation drive and forwards
TMP/TEMP through Bazel tests. Private runtime trees remain inside that root.

CI uses a short Bazel output root on Windows (`D:/b`) so native linkers can
open deeply nested runfiles. Locally, select a short writable root with
`bazel --output_user_root=C:/b test //:test` when needed. Documentation and
example scripts accept the same root through BAZEL_OUTPUT_USER_ROOT.

# Development

Install Mise, then run `mise trust` and `mise run bootstrap` in this checkout.

```sh
hk check --all --slow
bazel build //:artifacts
bazel test //:test
# Linux and macOS:
bazel test //:race_test
```

CI runs the same checks on Ubuntu 24.04, macOS 27, and Windows 2025 workers. Go race
variants retain their Windows incompatibility. Dependencies belong to the owned
lockfiles and MODULE configuration. Root consumers must declare source overrides
for Latticebuild modules until they are registered in BCR.

On Windows, CI creates LOCALAPPDATA/Temp/latticebuild before Mise installs
tools. This uses a canonical long path on the installation drive and forwards
TMP/TEMP through Bazel tests. Private runtime trees remain inside that root.

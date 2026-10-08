# Development

Install Mise, then run `mise trust` and `mise run bootstrap` in this checkout.

```sh
hk check --all --slow
bazel build //:artifacts
bazel test //:test
# Linux and macOS:
bazel test //:race_test
```

CI runs the same checks on native Linux, macOS, and Windows workers. Go race
variants retain their Windows incompatibility. Dependencies belong to the owned
lockfiles and MODULE configuration. Root consumers must declare source overrides
for Latticebuild modules until they are registered in BCR.

On Windows, use a temporary root with its canonical long path. Vite rejects 8.3
aliases in served paths. CI selects RUNNER_TEMP before dependency preparation and
forwards TMP/TEMP through Bazel tests; private runtime trees remain inside that
root. Keep this path out of installed source and dependency directories.

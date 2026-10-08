# Registry consumer

This separate Bazel module consumes the public `latticebuild_graceproc` API under the
`@subject` alias. Its own-module override resolves to the extracted parent
archive; other Latticebuild modules resolve through the registry.

The BCR presubmit builds `//:artifacts` and runs `//:test`.

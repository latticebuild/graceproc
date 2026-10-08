# Go documentation uses a fixed source platform for identical bytes on every host.
def main [--check] {
  let result = (with-env {GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0"} { ^go doc -all . | complete })
  if $result.exit_code != 0 { error make {msg: $result.stderr} }
  let generated = $"# Go API reference\n\nGenerated with `go doc -all .` using the Linux source view.\nThe exported API is shared across Linux, macOS and Windows.\n\n```text\n($result.stdout)```\n"
  let output = "docs/api.md"
  if $check {
    if not ($output | path exists) { error make {msg: "Missing generated Go API; run mise run docs"} }
    if (open --raw $output) != $generated { error make {msg: "Stale generated Go API; run mise run docs"} }
  } else {
    $generated | save --force $output
  }
}

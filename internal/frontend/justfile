# Shortcuts for working on ledge-frontend.

# List the recipes.
default:
    @just --list

# Compile the .templ files to *_templ.go.
generate:
    templ generate

# Fail if the *_templ.go files weren't what the .templ files compile to (and fix them).
check-generated:
    #!/usr/bin/env sh
    before=$(cat components/*_templ.go | shasum)
    templ generate >/dev/null || exit 1
    if [ "$before" != "$(cat components/*_templ.go | shasum)" ]; then
        echo "the *_templ.go files were stale; they've been regenerated" >&2
        exit 1
    fi

# The tests that don't open windows.
test: generate
    go test ./...

# The tests that open windows: a bar and a report window appear for a few seconds.
gui: generate
    LEDGE_GUI_TEST=1 go test -count=1 ./ui

# Open a bar and a report window in a theme, until ctrl-c.
preview theme="default": generate
    go run ./cmd/preview {{theme}}

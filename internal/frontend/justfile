# Shortcuts for working on ledge-frontend.

# List the recipes.
default:
    @just --list

# Compile the .templ files to *_templ.go.
generate:
    templ generate

# Fail if the committed *_templ.go files aren't what the .templ files compile to.
check-generated: generate
    git diff --exit-code -- '*_templ.go'

# The tests that don't open windows.
test: generate
    go test ./...

# The tests that open windows: a bar and a report window appear for a few seconds.
gui: generate
    LEDGE_GUI_TEST=1 go test -count=1 ./ui

# Open a bar and a report window in a theme, until ctrl-c.
preview theme="default": generate
    go run ./cmd/preview {{theme}}

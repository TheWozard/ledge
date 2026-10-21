# Shortcuts for working on ledge-frontend.

# The tests that open windows and go through each built-in theme: that each loads.
theme_tests := "^(TestThemes|TestReportThemes)$"

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

# Lint the CSS with Biome (brew install biome); see biome.json. Warnings fail it too.
lint:
    biome lint --error-on-warnings

# Clean up the CSS with Biome: format it, sort each rule's properties, apply safe fixes.
fmt:
    biome check --write

# The CSS lint, then the tests that don't open windows.
test: generate lint
    go test ./...

# The tests that open windows, with those that go through each theme only in the
# themes given (comma-separated): a bar and a report window appear for a few seconds.
gui themes="default": generate
    LEDGE_GUI_TEST=1 LEDGE_THEMES={{themes}} go test -count=1 ./ui

# Only the tests that go through each theme, in the themes given (comma-separated).
gui-themes themes: generate
    LEDGE_GUI_TEST=1 LEDGE_THEMES={{themes}} go test -count=1 -run '{{theme_tests}}' ./ui

# Every test that opens windows, in every theme.
gui-all: generate
    LEDGE_GUI_TEST=1 go test -count=1 ./ui

# Open a bar and a report window in a theme, until ctrl-c.
preview theme="default": generate
    go run ./cmd/preview {{theme}}

# Serve the bar and a report in a theme and open them in the browser, to inspect
# them, until ctrl-c. Edits to the theme's stylesheets show on a reload.
inspect theme="default": generate
    go run ./cmd/preview -browser {{theme}}

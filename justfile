# Shortcuts for working on ledge: build it from this repo and run it in the
# background, the example scripts in scripts/, and the frontend's generating, lint,
# window tests and previews.

# Where the build goes: ledge's state dir, as `ledge start` uses it.
bin := env("XDG_STATE_HOME", home_directory() / ".local" / "state") / "ledge" / "ledge"

# List the recipes.
default:
    @just --list

# Build ledge and start it in the background; arguments go to `ledge start`.
start *args: build
    "{{bin}}" start {{args}}

# Stop the ledge started in the background.
stop: build
    "{{bin}}" stop

# Rebuild, then `ledge restart`: reuses the last start's flags unless given new ones.
restart *args: build
    "{{bin}}" restart {{args}}

# Stop every ledge running, however it was started, such as one left by `go run`.
kill:
    scripts/kill.sh

# Build ledge into the state dir.
build:
    @mkdir -p "$(dirname "{{bin}}")"
    @go build -o "{{bin}}" .

# Example cards of every kind, plus extra ones to scroll: `just bar 15`.
bar *args:
    scripts/bar.sh {{args}}

# Example cards with reports.
report *args:
    scripts/report.sh {{args}}

# An example feed section, replaced after a pause.
feed *args:
    scripts/feed.sh {{args}}

# Remove the examples; `just clear --all` empties the bar.
clear *args:
    scripts/clear.sh {{args}}

# The frontend: the bar's and reports' windows, pages and themes, in internal/frontend.

# The tests that open windows and go through each built-in theme: that each loads.
theme_tests := "^(TestThemes|TestReportThemes)$"

# Compile the .templ files to *_templ.go.
generate:
    templ generate

# Fail if the *_templ.go files weren't what the .templ files compile to (and fix them).
check-generated:
    #!/usr/bin/env sh
    before=$(cat internal/frontend/components/*_templ.go | shasum)
    templ generate >/dev/null || exit 1
    if [ "$before" != "$(cat internal/frontend/components/*_templ.go | shasum)" ]; then
        echo "the *_templ.go files were stale; they've been regenerated" >&2
        exit 1
    fi

# Lint the CSS with Biome (brew install biome), see biome.json, warnings failing it too;
# then fail if any Go file isn't gofmt'd.
lint:
    #!/usr/bin/env sh
    biome lint --error-on-warnings || exit 1
    unformatted=$(gofmt -l .)
    if [ -n "$unformatted" ]; then
        echo "not gofmt'd (just fmt fixes them):" >&2
        echo "$unformatted" >&2
        exit 1
    fi

# Clean up the CSS with Biome: format it, sort each rule's properties, apply safe fixes;
# then gofmt the Go files.
fmt:
    biome check --write
    gofmt -w .

# The lint, then the tests that don't open windows.
test: generate lint
    go test ./...

# The tests that open windows: the frontend's, with those that go through each theme
# only in the themes given (comma-separated), then ledge's bar's. Windows appear for a
# few seconds.
gui themes="default": generate
    LEDGE_GUI_TEST=1 LEDGE_THEMES={{themes}} go test -count=1 ./internal/frontend/ui ./internal/bar

# Only the tests that go through each theme, in the themes given (comma-separated).
gui-themes themes: generate
    LEDGE_GUI_TEST=1 LEDGE_THEMES={{themes}} go test -count=1 -run '{{theme_tests}}' ./internal/frontend/ui

# Every test that opens windows, in every theme.
gui-all: generate
    LEDGE_GUI_TEST=1 go test -count=1 ./internal/frontend/ui ./internal/bar

# Open a bar and a report window in a theme, until ctrl-c.
preview theme="default": generate
    go run ./internal/frontend/cmd/preview {{theme}}

# Serve the bar and a report in a theme and open them in the browser, to inspect
# them, until ctrl-c. Edits to the theme's stylesheets show on a reload.
inspect theme="default": generate
    go run ./internal/frontend/cmd/preview -browser {{theme}}

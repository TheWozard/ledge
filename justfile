# Shortcuts for working on ledge: build it from this repo and run it in the
# background, and the example scripts in scripts/.

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

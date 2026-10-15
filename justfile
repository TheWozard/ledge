# Shortcuts for the scripts in scripts/; see each script for details.

# List the recipes.
default:
    @just --list

# Build ledge and start it in the background; arguments go to `ledge run`.
start *args:
    scripts/ledgectl.sh start {{args}}

# Stop the ledge started with `just start`.
stop:
    scripts/ledgectl.sh stop

# Stop, rebuild and start again; arguments go to `ledge run`.
restart *args:
    scripts/ledgectl.sh restart {{args}}

# Example cards and pills, plus extra ones to scroll: `just bar 15 12`.
bar *args:
    scripts/bar.sh {{args}}

# An example card with an HTML report.
report *args:
    scripts/report.sh {{args}}

# An example feed section, replaced after a pause.
feed *args:
    scripts/feed.sh {{args}}

# Remove the examples; `just clear --all` empties the bar.
clear *args:
    scripts/clear.sh {{args}}

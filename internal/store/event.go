package store

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"ledge/internal/schema"
)

//go:embed *.schema.json
var schemaFiles embed.FS

// Schemas is what can be sent: event, status and report (which the event schema
// $refs), plus section and dismiss for the other request bodies.
var Schemas = mustLoad()

func mustLoad() *schema.Set {
	set, err := schema.Load(schemaFiles)
	if err != nil {
		panic(err)
	}
	return set
}

// Kinds are the values of `kind`, each with a schema of the same name. An object
// without `kind` is an event.
var Kinds = []string{"event", "status"}

// Priorities in sort order, highest first.
var Priorities = []string{"high", "medium", "low"}

// TimeLayout is how times are stored: ISO 8601 to the second, with an offset.
const TimeLayout = "2006-01-02T15:04:05-07:00"

var duration = regexp.MustCompile(`^(\d+)([smhd])$`)

var unitSeconds = map[string]int64{"s": 1, "m": 60, "h": 3600, "d": 86400}

// Event is one event or status, as stored and as served.
type Event = map[string]any

// Error is something sent that should be refused, with the reason.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

// Refuse is an *Error with the reason.
func Refuse(format string, args ...any) error {
	return &Error{fmt.Sprintf(format, args...)}
}

// Check validates a request body against the named schema, giving an *Error that
// names the schema if it doesn't match.
func Check(name string, value any) error {
	err := Schemas.Validate(name, value)
	var invalid *schema.Error
	if errors.As(err, &invalid) {
		where := strings.Join(invalid.Path, "/")
		if where == "" {
			where = name
		}
		return Refuse("%s schema: %s: %s", name, where, invalid.Message)
	}
	return err
}

// Normalize validates one event or status against its schema and returns it with an
// `id`, a `source`, a `received` time and its times resolved. A non-empty source
// overrides the event's own.
func Normalize(raw any, source string, received time.Time) (Event, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		text, _ := json.Marshal(raw)
		return nil, Refuse("expected a JSON object, got %.200s", text)
	}
	kind := "event"
	if value, present := obj["kind"]; present {
		name, _ := value.(string)
		if !slices.Contains(Kinds, name) {
			text, _ := json.Marshal(value)
			return nil, Refuse("kind %s isn't one of %s", text, strings.Join(Kinds, ", "))
		}
		kind = name
	}
	if err := Check(kind, obj); err != nil {
		return nil, err
	}

	event := maps.Clone(obj)
	if source == "" {
		source, _ = event["source"].(string)
	}
	if source == "" {
		source = "manual"
	}
	event["source"] = source
	if text, ok := event["expires"].(string); ok {
		moment, err := resolveTime(text, "expires", received)
		if err != nil {
			return nil, err
		}
		event["expires"] = moment.Format(TimeLayout)
	}
	if steps, ok := event["schedule"].([]any); ok {
		type step struct {
			at    time.Time
			color any
		}
		resolved := make([]step, 0, len(steps))
		for _, raw := range steps {
			s := raw.(map[string]any) // the schema has checked the shape
			at, err := resolveTime(s["at"].(string), "schedule time", received)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, step{at, s["color"]})
		}
		// Kept in time order, so the page can walk it front to back.
		slices.SortStableFunc(resolved, func(a, b step) int { return a.at.Compare(b.at) })
		schedule := make([]any, len(resolved))
		for i, s := range resolved {
			schedule[i] = map[string]any{"at": s.at.Format(TimeLayout), "color": s.color}
		}
		event["schedule"] = schedule
	}
	if id, _ := event["id"].(string); id == "" {
		event["id"] = newID()
	}
	event["received"] = received.UTC().Format(TimeLayout)
	return event, nil
}

// newID is 12 random hex characters.
func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// resolveTime reads an ISO 8601 time, or a duration ("30m", "4h", "2d") after
// received.
func resolveTime(text, what string, received time.Time) (time.Time, error) {
	if match := duration.FindStringSubmatch(text); match != nil {
		count, err := strconv.ParseInt(match[1], 10, 64)
		unit := unitSeconds[match[2]]
		if err != nil || count > math.MaxInt64/int64(time.Second)/unit {
			return time.Time{}, Refuse("%s %q is too far away", what, text)
		}
		return received.UTC().Add(time.Duration(count*unit) * time.Second), nil
	}
	return ParseTime(text, what)
}

// isoLayouts are the ISO 8601 forms accepted for a time. One without an offset is
// taken as local time.
var isoLayouts = []struct {
	layout string
	zoned  bool
}{
	{"2006-01-02T15:04:05.999999999Z07:00", true},
	{"2006-01-02T15:04Z07:00", true},
	{"2006-01-02T15:04:05.999999999Z0700", true},
	{"2006-01-02T15:04:05.999999999", false},
	{"2006-01-02T15:04", false},
	{"2006-01-02", false},
}

// ParseTime reads an ISO 8601 time.
func ParseTime(text, what string) (time.Time, error) {
	normal := text
	if len(normal) > 10 && normal[10] == ' ' {
		normal = normal[:10] + "T" + normal[11:]
	}
	for _, form := range isoLayouts {
		var moment time.Time
		var err error
		if form.zoned {
			moment, err = time.Parse(form.layout, normal)
		} else {
			moment, err = time.ParseInLocation(form.layout, normal, time.Local)
		}
		if err == nil {
			return moment, nil
		}
	}
	return time.Time{}, Refuse("%s %q isn't an ISO 8601 time or a duration like \"30m\"", what, text)
}

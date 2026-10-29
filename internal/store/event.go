package store

import (
	"bytes"
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

	"github.com/TheWozard/ledge/internal/schema"
)

//go:embed *.schema.json
var schemaFiles embed.FS

// Schemas is what can be sent: an event, or a section of a source's events.
var Schemas = schema.MustLoad(schemaFiles)

// Priorities in sort order, highest first.
var Priorities = []string{"high", "medium", "low"}

// Event is one event, as stored and as served: what was sent, as event.schema.json
// describes it, with an id, a source, when it was received, and its times resolved.
type Event struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Priority    string    `json:"priority,omitempty"`
	Color       string    `json:"color,omitempty"`
	Schedule    []Step    `json:"schedule,omitempty"` // in time order
	URL         string    `json:"url,omitempty"`
	Report      []Block   `json:"report,omitempty"`
	Command     string    `json:"command,omitempty"`
	Source      string    `json:"source"`
	Received    time.Time `json:"received"`
	Expires     time.Time `json:"expires,omitzero"`

	// A timer's.
	Start time.Time `json:"start,omitzero"`
	End   time.Time `json:"end,omitzero"`
	// A progress's (a number from 0 to 1) or a stat's.
	Value Scalar `json:"value,omitzero"`
	// A progress's.
	Label string `json:"label,omitempty"`
	// A stat's.
	Change      string    `json:"change,omitempty"`
	ChangeColor string    `json:"change_color,omitempty"`
	History     []float64 `json:"history,omitempty"`
	// A checks card's.
	Checks []CheckResult `json:"checks,omitempty"`
	// A message's.
	From   string    `json:"from,omitempty"`
	Sent   time.Time `json:"sent,omitzero"`
	Avatar string    `json:"avatar,omitempty"`
}

// Step is a colour change in an event's schedule.
type Step struct {
	At    time.Time `json:"at"`
	Color string    `json:"color"`
}

// CheckResult is one of a checks card's checks.
type CheckResult struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// Scalar is a JSON string or number, kept as it was sent, such as a stat's value.
type Scalar struct {
	Text   string // the string, or the number as it was written
	Number bool
}

// Float is the number, or 0 for a string.
func (s Scalar) Float() float64 {
	if !s.Number {
		return 0
	}
	f, _ := strconv.ParseFloat(s.Text, 64)
	return f
}

func (s Scalar) IsZero() bool { return s == Scalar{} }

func (s Scalar) MarshalJSON() ([]byte, error) {
	if s.Number {
		return []byte(s.Text), nil
	}
	return marshal(s.Text)
}

func (s *Scalar) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(data, []byte(`"`)) {
		*s = Scalar{}
		return json.Unmarshal(data, &s.Text)
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*s = Scalar{Text: n.String(), Number: true}
	return nil
}

// Error is something sent that should be refused, with the reason.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

// refuse is an *Error with the reason.
func refuse(format string, args ...any) error {
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
		return refuse("%s schema: %s: %s", name, where, invalid.Message)
	}
	return err
}

// Normalize validates one event against the event schema and returns it as an Event,
// with an id, a source, a received time and its times resolved. A non-empty source
// overrides the event's own.
func Normalize(raw any, source string, received time.Time) (Event, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		text, _ := json.Marshal(raw)
		return Event{}, refuse("expected a JSON object, got %.200s", text)
	}
	if err := Check("event", obj); err != nil {
		return Event{}, err
	}

	// The schema has checked the shape. Only the times, which can be durations, need
	// reading before the rest decodes into an Event.
	resolved := maps.Clone(obj)
	for _, key := range []string{"expires", "start", "end", "sent"} {
		if text, ok := resolved[key].(string); ok {
			moment, err := resolveTime(text, key, received)
			if err != nil {
				return Event{}, err
			}
			resolved[key] = moment
		}
	}
	if steps, ok := resolved["schedule"].([]any); ok {
		schedule := make([]any, len(steps))
		for i, raw := range steps {
			step := maps.Clone(raw.(map[string]any))
			at, err := resolveTime(step["at"].(string), "schedule time", received)
			if err != nil {
				return Event{}, err
			}
			step["at"] = at
			schedule[i] = step
		}
		resolved["schedule"] = schedule
	}
	data, err := json.Marshal(resolved)
	if err != nil {
		return Event{}, err
	}
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, err
	}

	// Kept in time order, so the bar can walk it front to back.
	slices.SortStableFunc(event.Schedule, func(a, b Step) int { return a.At.Compare(b.At) })
	if source != "" {
		event.Source = source
	} else if event.Source == "" {
		event.Source = "manual"
	}
	if event.Kind == "" {
		event.Kind = "basic"
	}
	if event.ID == "" {
		event.ID = newID()
	}
	event.Received = received.UTC().Truncate(time.Second)
	return event, nil
}

// newID is 12 random hex characters.
func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

var duration = regexp.MustCompile(`^(\d+)([smhd])$`)

var unitSeconds = map[string]int64{"s": 1, "m": 60, "h": 3600, "d": 86400}

// resolveTime reads an ISO 8601 time, or a duration ("30m", "4h", "2d") after
// received, to the second.
func resolveTime(text, what string, received time.Time) (time.Time, error) {
	if match := duration.FindStringSubmatch(text); match != nil {
		count, err := strconv.ParseInt(match[1], 10, 64)
		unit := unitSeconds[match[2]]
		if err != nil || count > math.MaxInt64/int64(time.Second)/unit {
			return time.Time{}, refuse("%s %q is too far away", what, text)
		}
		return received.UTC().Truncate(time.Second).Add(time.Duration(count*unit) * time.Second), nil
	}
	moment, err := parseTime(text, what)
	return moment.Truncate(time.Second), err
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

// parseTime reads an ISO 8601 time.
func parseTime(text, what string) (time.Time, error) {
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
	return time.Time{}, refuse("%s %q isn't an ISO 8601 time or a duration like \"30m\"", what, text)
}

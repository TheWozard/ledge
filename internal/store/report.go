package store

import (
	"bytes"
	"encoding/json"
	"slices"
)

// Block is one part of a report. Type says which, and so which other fields it has.
type Block struct {
	Type    string     `json:"type"`
	Text    Inline     `json:"text,omitzero"`     // heading, text, code, note, quote, link
	Level   int        `json:"level,omitempty"`   // heading
	Color   string     `json:"color,omitempty"`   // note
	Command string     `json:"command,omitempty"` // run
	Title   string     `json:"title,omitempty"`   // run
	URL     string     `json:"url,omitempty"`     // link
	Fields  []Field    `json:"-"`                 // fields, sent as "items"
	Items   []Item     `json:"-"`                 // list, sent as "items"
	Buttons []Button   `json:"-"`                 // buttons, sent as "items"
	Ordered bool       `json:"ordered,omitempty"` // list
	Columns []string   `json:"columns,omitempty"` // table
	Rows    [][]Inline `json:"rows,omitzero"`     // table
	Summary Inline     `json:"summary,omitzero"`  // details
	Blocks  []Block    `json:"blocks,omitempty"`  // details
	Src     string     `json:"src,omitempty"`     // image
	Alt     string     `json:"alt,omitempty"`     // image
}

// Field is one row of a fields block.
type Field struct {
	Label string `json:"label"`
	Value Inline `json:"value"`
}

// block is Block without its JSON methods, for them to use.
type block Block

// UnmarshalJSON reads "items" as a fields block's Fields, a buttons block's Buttons,
// or a list's Items.
func (b *Block) UnmarshalJSON(data []byte) error {
	var raw struct {
		block
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*b = Block(raw.block)
	switch {
	case raw.Items == nil:
		return nil
	case b.Type == "fields":
		return json.Unmarshal(raw.Items, &b.Fields)
	case b.Type == "buttons":
		return json.Unmarshal(raw.Items, &b.Buttons)
	default:
		return json.Unmarshal(raw.Items, &b.Items)
	}
}

// MarshalJSON writes a fields block's Fields, a buttons block's Buttons, or a list's
// Items, as "items".
func (b Block) MarshalJSON() ([]byte, error) {
	raw := struct {
		block
		Items any `json:"items,omitempty"`
	}{block: block(b)}
	switch {
	case b.Fields != nil:
		raw.Items = b.Fields
	case b.Buttons != nil:
		raw.Items = b.Buttons
	case b.Items != nil:
		raw.Items = b.Items
	}
	return marshal(raw)
}

// Inline is text in a block, as it was sent: a string; a number, kept as it was
// written, in a cell or a field's value; a span of strong, em, mono or linked text;
// an array of those, run together; or, in a cell or a field's value, a row of buttons,
// a select or a checkbox.
type Inline struct {
	Type    string // strong, em, mono, link, buttons, select or checkbox; "" for a string, a number or an array
	Text    string // the string, or the number as it was written
	Number  bool
	Parts   []Inline // an array's
	Content *Inline  // a span's text
	URL     string   // a link's
	Buttons []Button // buttons'
	ID      string   // a select's or a checkbox's
	Options []string // a select's
	Value   string   // a select's, shown until one's chosen
	Label   string   // a checkbox's
	Checked bool     // a checkbox's, shown until it's clicked
}

func (in Inline) IsZero() bool {
	return in.Type == "" && in.Text == "" && !in.Number && in.Parts == nil
}

// span is how an Inline with a Type is sent.
type span struct {
	Type    string   `json:"type"`
	Text    *Inline  `json:"text,omitempty"`
	URL     string   `json:"url,omitempty"`
	Items   []Button `json:"items,omitempty"`
	ID      string   `json:"id,omitempty"`
	Options []string `json:"options,omitempty"`
	Value   string   `json:"value,omitempty"`
	Label   string   `json:"label,omitempty"`
	Checked bool     `json:"checked,omitempty"`
}

func (in Inline) MarshalJSON() ([]byte, error) {
	switch {
	case in.Type != "":
		return marshal(span{in.Type, in.Content, in.URL, in.Buttons, in.ID, in.Options, in.Value, in.Label, in.Checked})
	case in.Parts != nil:
		return marshal(in.Parts)
	default:
		return Scalar{in.Text, in.Number}.MarshalJSON()
	}
}

func (in *Inline) UnmarshalJSON(data []byte) error {
	*in = Inline{}
	switch {
	case bytes.HasPrefix(data, []byte("[")):
		in.Parts = []Inline{}
		return json.Unmarshal(data, &in.Parts)
	case bytes.HasPrefix(data, []byte("{")):
		var s span
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		in.Type, in.Content, in.URL, in.Buttons = s.Type, s.Text, s.URL, s.Items
		in.ID, in.Options, in.Value, in.Label, in.Checked = s.ID, s.Options, s.Value, s.Label, s.Checked
		return nil
	default:
		var s Scalar
		err := json.Unmarshal(data, &s)
		in.Text, in.Number = s.Text, s.Number
		return err
	}
}

// Item is one of a list's items: its text, and the list under it, if it has Items,
// numbered if Ordered. One with no list under it is sent as its text alone.
type Item struct {
	Text    Inline `json:"text"`
	Ordered bool   `json:"ordered,omitempty"`
	Items   []Item `json:"items,omitempty"`
}

// item is Item without its JSON methods, for them to use.
type item Item

func (it Item) MarshalJSON() ([]byte, error) {
	if it.Items == nil && !it.Ordered {
		return marshal(it.Text)
	}
	return marshal(item(it))
}

// UnmarshalJSON reads an object without a "type" as an item with a list under it, and
// anything else as an item's text.
func (it *Item) UnmarshalJSON(data []byte) error {
	*it = Item{}
	if bytes.HasPrefix(data, []byte("{")) {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(data, &keys); err != nil {
			return err
		}
		if _, ok := keys["type"]; !ok {
			return json.Unmarshal(data, (*item)(it))
		}
	}
	return json.Unmarshal(data, &it.Text)
}

// Button is a button in a report. Clicking it opens its URL, runs its Command or
// dismisses its event; with none of them, it does nothing.
type Button struct {
	Title   string `json:"title"`
	Color   string `json:"color,omitempty"`
	URL     string `json:"url,omitempty"`
	Command string `json:"command,omitempty"`
	Dismiss bool   `json:"dismiss,omitempty"`
}

// Inputs is the selects and checkboxes in blocks, in a table's cells or a field's
// value, in details too, in the order they're drawn.
func Inputs(blocks []Block) []Inline {
	var out []Inline
	for _, block := range blocks {
		for _, field := range block.Fields {
			if field.Value.IsInput() {
				out = append(out, field.Value)
			}
		}
		for _, row := range block.Rows {
			for _, cell := range row {
				if cell.IsInput() {
					out = append(out, cell)
				}
			}
		}
		out = append(out, Inputs(block.Blocks)...)
	}
	return out
}

// IsInput is whether in is a select or a checkbox, whose choice is kept.
func (in Inline) IsInput() bool { return in.Type == "select" || in.Type == "checkbox" }

// Allows is whether value can be chosen in in: one of a select's options, or true or
// false for a checkbox.
func (in Inline) Allows(value any) bool {
	switch value := value.(type) {
	case string:
		return in.Type == "select" && slices.Contains(in.Options, value)
	case bool:
		return in.Type == "checkbox"
	}
	return false
}

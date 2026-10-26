package components

import (
	"context"
	"io"
	"slices"

	"github.com/a-h/templ"
)

// Block is block with an id, unique among its report's blocks, so ShowReportBlock can
// replace it later and leave the rest of the report as it is. It's drawn as block
// alone; the ids of the Clicks in it are its own, so two Blocks can use the same.
func Block(id string, block templ.Component) templ.Component { return idBlock{id, block} }

// idBlock is a Block: block, with its id.
type idBlock struct {
	id    string
	block templ.Component
}

func (b idBlock) Render(ctx context.Context, w io.Writer) error {
	return inScope(scope{"block", b.id}, b.block).Render(ctx, w)
}

// ReplaceBlock is blocks with the Block among them that has id replaced by a Block of
// block with that id, and whether there was one. blocks itself is left as it is.
func ReplaceBlock(blocks []templ.Component, id string, block templ.Component) ([]templ.Component, bool) {
	i := slices.IndexFunc(blocks, func(c templ.Component) bool {
		b, ok := c.(idBlock)
		return ok && b.id == id
	})
	if i < 0 {
		return blocks, false
	}
	blocks = slices.Clone(blocks)
	blocks[i] = Block(id, block)
	return blocks, true
}

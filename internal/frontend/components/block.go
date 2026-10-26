package components

import (
	"context"
	"io"
	"slices"

	"github.com/a-h/templ"
)

// Block is a report's block with an id, unique among its report's blocks, so
// ShowReportBlock can replace it later and leave the rest of the report as it is.
// It's drawn as its Content alone; the ids of the Clicks in it are its own, so two
// Blocks can use the same.
type Block struct {
	ID      string
	Content templ.Component
}

func (b Block) Render(ctx context.Context, w io.Writer) error {
	return inScope(scope{"block", b.ID}, b.Content).Render(ctx, w)
}

// ReplaceBlock is blocks with the Block among them that has block's ID replaced by
// block, and whether there was one. blocks itself is left as it is.
func ReplaceBlock(blocks []templ.Component, block Block) ([]templ.Component, bool) {
	i := slices.IndexFunc(blocks, func(c templ.Component) bool {
		b, ok := c.(Block)
		return ok && b.ID == block.ID
	})
	if i < 0 {
		return blocks, false
	}
	blocks = slices.Clone(blocks)
	blocks[i] = block
	return blocks, true
}

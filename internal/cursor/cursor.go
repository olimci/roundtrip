package cursor

// NewCursor returns a cursor positioned at line 1, column 1, offset 0.
func NewCursor() *Cursor {
	return &Cursor{
		Position: Position{
			Line:   1,
			Column: 1,
		},
	}
}

// Cursor tracks the current input position and one previous position.
//
// Cursor methods require a non-nil *Cursor.
type Cursor struct {
	Position
	prev *Position
}

// Next advances c by r and size bytes.
func (c *Cursor) Next(r rune, size int) {
	if c.prev == nil {
		c.prev = new(Position)
	}
	*c.prev = c.Position
	c.Offset += size

	switch r {

	case '\r':
		c.Column = 1
	case '\n':
		c.Line++
		c.Column = 1
	default:
		c.Column++
	}
}

// Prev returns c to the previous position, if any.
func (c *Cursor) Prev() {
	if c.prev == nil {
		return
	}
	c.Position = *c.prev
	c.prev = nil
}

// Position is a one-based line and column with a zero-based byte offset.
type Position struct {
	Line   int
	Column int
	Offset int
}

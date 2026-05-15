package cursor

func NewCursor() *Cursor {
	return &Cursor{
		Position: Position{
			Line:   1,
			Column: 1,
		},
	}
}

type Cursor struct {
	Position
	prev *Position
}

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

func (c *Cursor) Prev() {
	if c.prev == nil {
		return
	}
	c.Position = *c.prev
	c.prev = nil
}

type Position struct {
	Line   int
	Column int
	Offset int
}

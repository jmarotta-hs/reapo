package completion

type CompletionItem struct {
	Text        string
	Description string
	Score       int
}

type CompletionType int

const (
	SlashCommand CompletionType = iota
	FileFolder
)

type CompletionState struct {
	Active   bool
	Trigger  rune // '/' or '@'
	Query    string
	Items    []CompletionItem
	Selected int
	Type     CompletionType
}

func (c *CompletionState) Reset() {
	c.Active = false
	c.Trigger = 0
	c.Query = ""
	c.Items = nil
	c.Selected = 0
	c.Type = 0
}

func (c *CompletionState) SelectNext() {
	if len(c.Items) > 0 {
		c.Selected = (c.Selected + 1) % len(c.Items)
	}
}

func (c *CompletionState) SelectPrev() {
	if len(c.Items) > 0 {
		c.Selected = (c.Selected - 1 + len(c.Items)) % len(c.Items)
	}
}

func (c *CompletionState) GetSelectedItem() *CompletionItem {
	if len(c.Items) > 0 && c.Selected >= 0 && c.Selected < len(c.Items) {
		return &c.Items[c.Selected]
	}
	return nil
}

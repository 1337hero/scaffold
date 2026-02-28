package agents

// Step is one agent subprocess in a chain.
type Step struct {
	Name       string
	PromptFile string   // filename in prompts dir
	Tools      []string // nil = default tool set
	Thinking   string   // pi --thinking level: off, minimal, low, medium, high, xhigh
}

// Chain is an ordered sequence of steps.
type Chain struct {
	Steps []Step
}

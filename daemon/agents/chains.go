package agents

// Step is one agent subprocess in a chain.
type Step struct {
	Name       string
	PromptFile string   // filename in prompts dir
	Tools      []string // nil = default tool set
}

// Chain is an ordered sequence of steps.
type Chain struct {
	Steps []Step
}

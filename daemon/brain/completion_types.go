package brain

import "scaffold/llm"

type CompletionClient = llm.CompletionClient

type Dependencies struct {
	Responder            ToolUseResponder
	TriageCompletion     llm.CompletionClient
	PrioritizeCompletion llm.CompletionClient
}

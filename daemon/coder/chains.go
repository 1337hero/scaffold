package coder

// Step is one agent subprocess in a chain.
type Step struct {
	Name   string
	Prompt string // template: {task}, {previous}, {chain_dir}
}

// Chain is an ordered sequence of steps.
type Chain struct {
	Steps []Step
}

var builtinChains = map[string]Chain{
	"implement": {Steps: []Step{
		{Name: "scout", Prompt: scoutPrompt},
		{Name: "planner", Prompt: plannerPrompt},
		{Name: "worker", Prompt: workerPrompt},
		{Name: "reviewer", Prompt: reviewerPrompt},
	}},
	"fix": {Steps: []Step{
		{Name: "worker", Prompt: workerPrompt},
		{Name: "verify", Prompt: verifyPrompt},
	}},
	"spec": {Steps: []Step{
		{Name: "scout", Prompt: scoutPrompt},
		{Name: "planner", Prompt: plannerPrompt},
	}},
	"single": {Steps: []Step{
		{Name: "worker", Prompt: workerPrompt},
	}},
}

const scoutPrompt = `You are scouting a coding task. Do NOT write any code yet.

Task: {task}

Read the relevant issue/ticket/description. Map the codebase structure
relevant to this task. Identify the key files, patterns, and constraints.
Write a clear summary in {chain_dir}/steps/1-scout/output.md.`

const plannerPrompt = `You are planning an implementation. Do NOT write any code yet.

Task: {task}

Scout output:
{previous}

Based on the scout's findings, write a concrete step-by-step implementation
plan. Be specific about which files to edit, what to add/change, and in what
order. Write the plan to {chain_dir}/steps/2-planner/output.md.`

const workerPrompt = `You are implementing a planned feature.

Task: {task}

Implementation plan:
{previous}

Follow the plan. Edit files, run tests after each change, fix failures.
Use ` + "`committer`" + ` (not git commit) for commits. Branch: feature/description.
Write a brief summary of what you did to {chain_dir}/steps/3-worker/output.md.`

const reviewerPrompt = `You are reviewing and shipping an implementation.

Task: {task}

What was implemented:
{previous}

Review the diff (` + "`git diff main`" + `). Verify tests pass. If anything looks wrong,
fix it. Then open a PR with ` + "`gh pr create`" + `. Write the PR URL to
{chain_dir}/steps/4-reviewer/output.md.`

const verifyPrompt = `You are verifying an implementation.

Task: {task}

What was done:
{previous}

Run the test suite. Check for regressions. Verify the fix works end-to-end.
Write your findings to {chain_dir}/steps/2-verify/output.md.`

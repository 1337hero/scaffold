package cortex

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"scaffold/brain"
	appconfig "scaffold/config"
	"scaffold/db"
)

const (
	schedulerTickInterval  = time.Minute
	bulletinMaxInputItems  = 12
	bulletinMaxTokensFloor = 256
	bulletinMaxTokensCeil  = 2048
)

type CortexTask struct {
	Name     string
	Interval time.Duration
	Timeout  time.Duration
	Fn       func(ctx context.Context) error
	mu       sync.Mutex
	lastRun  atomic.Int64
}

func (t *CortexTask) ShouldRun(now time.Time) bool {
	if t == nil || t.Interval <= 0 || t.Fn == nil {
		return false
	}
	last := t.lastRun.Load()
	if last == 0 {
		return true
	}
	return now.Sub(time.Unix(last, 0)) >= t.Interval
}

type BulletinCache struct {
	content       atomic.Value
	generatedAt   atomic.Int64
	maxStaleAfter time.Duration
}

func newBulletinCache(maxStaleAfter time.Duration) *BulletinCache {
	cache := &BulletinCache{maxStaleAfter: maxStaleAfter}
	cache.content.Store("")
	return cache
}

func (b *BulletinCache) Get() (content string, fresh bool) {
	content, _ = b.content.Load().(string)
	generated := b.generatedAt.Load()
	if generated == 0 {
		return content, true
	}
	if b.maxStaleAfter <= 0 {
		return content, true
	}
	age := time.Since(time.Unix(generated, 0))
	return content, age <= b.maxStaleAfter
}

func (b *BulletinCache) Set(content string) {
	b.content.Store(strings.TrimSpace(content))
	b.generatedAt.Store(time.Now().Unix())
}

type llmClient interface {
	SynthesizeBulletin(ctx context.Context, model string, maxWords int, sections bulletinSections) (string, error)
}

type anthropicLLMClient struct {
	client anthropic.Client
}

func newAnthropicLLMClient(apiKey string) llmClient {
	return &anthropicLLMClient{client: anthropic.NewClient(option.WithAPIKey(apiKey))}
}

func (c *anthropicLLMClient) SynthesizeBulletin(ctx context.Context, model string, maxWords int, sections bulletinSections) (string, error) {
	systemPrompt := `You are the cortex's memory bulletin synthesizer.
You receive pre-gathered memory data organized by category and synthesize it into one concise, contextualized briefing.
The bulletin is injected into conversation so the agent has ambient awareness.

Do:
- Prioritize recent and high-importance information
- Connect related facts into coherent narratives
- Keep it scannable and actionable
- Stay under the provided word limit

Do not:
- List IDs or metadata
- Repeat section headers
- Repeat the same information in multiple phrasings
- Output markdown code fences`

	payload, err := json.Marshal(sections)
	if err != nil {
		return "", fmt.Errorf("marshal sections: %w", err)
	}

	maxTokens := int64(maxWords * 3)
	if maxTokens < bulletinMaxTokensFloor {
		maxTokens = bulletinMaxTokensFloor
	}
	if maxTokens > bulletinMaxTokensCeil {
		maxTokens = bulletinMaxTokensCeil
	}

	userPrompt := fmt.Sprintf("Max words: %d\n\nMemory sections (JSON):\n%s", maxWords, string(payload))

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty bulletin response")
	}

	text := strings.TrimSpace(resp.Content[0].Text)
	if text == "" {
		return "", fmt.Errorf("blank bulletin response")
	}
	return truncateWords(text, maxWords), nil
}

type memorySnippet struct {
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Importance float64 `json:"importance"`
	Tags       string  `json:"tags,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type bulletinSections struct {
	Identity     []memorySnippet `json:"identity"`
	Goals        []memorySnippet `json:"goals"`
	Decisions    []memorySnippet `json:"decisions"`
	Todos        []memorySnippet `json:"todos"`
	Preferences  []memorySnippet `json:"preferences"`
	Events       []memorySnippet `json:"events"`
	Observations []memorySnippet `json:"observations"`
	Recent       []memorySnippet `json:"recent"`
}

func (s bulletinSections) Empty() bool {
	return len(s.Identity) == 0 &&
		len(s.Goals) == 0 &&
		len(s.Decisions) == 0 &&
		len(s.Todos) == 0 &&
		len(s.Preferences) == 0 &&
		len(s.Events) == 0 &&
		len(s.Observations) == 0 &&
		len(s.Recent) == 0
}

type Cortex struct {
	db       *db.DB
	brain    *brain.Brain
	llm      llmClient
	cfg      appconfig.CortexConfig
	bulletin *BulletinCache
	tasks    []*CortexTask
	once     sync.Once
}

func New(database *db.DB, b *brain.Brain, apiKey string, cfg appconfig.CortexConfig) *Cortex {
	if cfg.Bulletin.IntervalMinutes <= 0 {
		cfg.Bulletin.IntervalMinutes = 60
	}
	if cfg.Bulletin.MaxWords <= 0 {
		cfg.Bulletin.MaxWords = 500
	}
	if cfg.Bulletin.MaxStaleMultiplier <= 0 {
		cfg.Bulletin.MaxStaleMultiplier = 3
	}
	if strings.TrimSpace(cfg.Bulletin.Model) == "" {
		cfg.Bulletin.Model = "claude-haiku-4-5"
	}

	maxStale := time.Duration(cfg.Bulletin.IntervalMinutes*cfg.Bulletin.MaxStaleMultiplier) * time.Minute
	if maxStale <= 0 {
		maxStale = 3 * time.Hour
	}

	c := &Cortex{
		db:       database,
		brain:    b,
		llm:      newAnthropicLLMClient(apiKey),
		cfg:      cfg,
		bulletin: newBulletinCache(maxStale),
	}

	c.tasks = c.buildTasks()
	return c
}

func (c *Cortex) Start(ctx context.Context) {
	if c == nil {
		return
	}
	c.once.Do(func() {
		go c.run(ctx)
	})
}

func (c *Cortex) CurrentBulletin() (string, bool) {
	if c == nil || c.bulletin == nil {
		return "", true
	}
	return c.bulletin.Get()
}

func (c *Cortex) run(ctx context.Context) {
	if task := c.taskByName("bulletin"); task != nil {
		c.runTask(ctx, task)
	}

	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			for _, task := range c.tasks {
				if task.ShouldRun(now) {
					c.runTask(ctx, task)
				}
			}
		}
	}
}

func (c *Cortex) runTask(ctx context.Context, task *CortexTask) {
	if task == nil || task.Fn == nil {
		return
	}

	if !task.mu.TryLock() {
		log.Printf("cortex: %s still running, skipping", task.Name)
		return
	}

	go func() {
		defer task.mu.Unlock()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("cortex: %s panicked: %v", task.Name, r)
			}
		}()

		timeout := task.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		taskCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		start := time.Now()
		if err := task.Fn(taskCtx); err != nil {
			log.Printf("cortex: %s failed (%v): %v", task.Name, time.Since(start), err)
			return
		}

		task.lastRun.Store(time.Now().Unix())
		log.Printf("cortex: %s completed (%v)", task.Name, time.Since(start))
	}()
}

func (c *Cortex) buildTasks() []*CortexTask {
	tasks := make([]*CortexTask, 0, 1+len(c.cfg.Tasks))
	tasks = append(tasks, &CortexTask{
		Name:     "bulletin",
		Interval: time.Duration(c.cfg.Bulletin.IntervalMinutes) * time.Minute,
		Timeout:  30 * time.Second,
		Fn:       c.generateBulletin,
	})

	if len(c.cfg.Tasks) > 0 {
		names := make([]string, 0, len(c.cfg.Tasks))
		for name := range c.cfg.Tasks {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			taskCfg := c.cfg.Tasks[name]
			taskFn := c.taskFnForName(name)
			if taskFn == nil {
				log.Printf("cortex: no handler for task %q; skipping", name)
				continue
			}
			tasks = append(tasks, &CortexTask{
				Name:     name,
				Interval: time.Duration(taskCfg.IntervalHours) * time.Hour,
				Timeout:  time.Duration(taskCfg.TimeoutSeconds) * time.Second,
				Fn:       taskFn,
			})
		}
	}

	return tasks
}

func (c *Cortex) taskFnForName(name string) func(context.Context) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "consolidation":
		return c.runConsolidation
	case "decay":
		return c.runDecay
	case "prioritize":
		return c.runPrioritization
	case "prune":
		return c.runPrune
	case "reindex":
		return c.runReindex
	case "session_cleanup":
		return c.runSessionCleanup
	default:
		return nil
	}
}

func (c *Cortex) taskByName(name string) *CortexTask {
	for _, task := range c.tasks {
		if task.Name == name {
			return task
		}
	}
	return nil
}

func (c *Cortex) generateBulletin(ctx context.Context) error {
	sections, err := c.loadSections()
	if err != nil {
		return err
	}

	if sections.Empty() {
		log.Printf("cortex: bulletin skipped (no active memories)")
		return nil
	}

	content, err := c.llm.SynthesizeBulletin(
		ctx,
		c.cfg.Bulletin.Model,
		c.cfg.Bulletin.MaxWords,
		sections,
	)
	if err != nil {
		return fmt.Errorf("synthesize bulletin: %w", err)
	}

	if c.bulletin != nil {
		c.bulletin.Set(content)
	}
	return nil
}

func (c *Cortex) loadSections() (bulletinSections, error) {
	sections := bulletinSections{}

	var err error
	sections.Identity, err = c.listType("Identity")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load identity: %w", err)
	}
	sections.Goals, err = c.listType("Goal")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load goals: %w", err)
	}
	sections.Decisions, err = c.listType("Decision")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load decisions: %w", err)
	}
	sections.Todos, err = c.listType("Todo")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load todos: %w", err)
	}
	sections.Preferences, err = c.listType("Preference")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load preferences: %w", err)
	}
	sections.Events, err = c.listType("Event")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load events: %w", err)
	}
	sections.Observations, err = c.listType("Observation")
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load observations: %w", err)
	}

	recentMemories, err := c.db.ListRecentMemories(bulletinMaxInputItems)
	if err != nil {
		return bulletinSections{}, fmt.Errorf("load recent: %w", err)
	}
	sections.Recent = toSnippets(recentMemories)

	return sections, nil
}

func (c *Cortex) listType(memoryType string) ([]memorySnippet, error) {
	memories, err := c.db.ListByType(memoryType, bulletinMaxInputItems)
	if err != nil {
		return nil, err
	}
	return toSnippets(memories), nil
}

func (c *Cortex) runSessionCleanup(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := c.db.CleanExpiredSessions(); err != nil {
		return fmt.Errorf("clean expired sessions: %w", err)
	}
	return nil
}

func (c *Cortex) runPrioritization(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}
	if c.brain == nil {
		return fmt.Errorf("brain is nil")
	}

	todos, err := c.db.ListTodosByImportance(0.5, 20)
	if err != nil {
		return fmt.Errorf("list todos: %w", err)
	}

	yesterdayDesk, err := c.db.YesterdaysDesk()
	if err != nil {
		return fmt.Errorf("yesterday's desk: %w", err)
	}

	tasks, err := c.brain.Prioritize(ctx, todos, yesterdayDesk)
	if err != nil {
		return fmt.Errorf("prioritize: %w", err)
	}

	todayDate := time.Now().Format("2006-01-02")
	for i, task := range tasks {
		stepsJSON, err := json.Marshal(task.MicroSteps)
		if err != nil {
			return fmt.Errorf("marshal micro_steps for %q: %w", task.Title, err)
		}

		item := db.DeskItem{
			Title:    task.Title,
			Position: i + 1,
			Status:   "active",
			MicroSteps: sql.NullString{
				String: string(stepsJSON),
				Valid:  true,
			},
			Date: todayDate,
		}
		if task.SourceMemoryID != "" {
			mem, err := c.db.GetMemory(task.SourceMemoryID)
			if err != nil {
				return fmt.Errorf("load source memory %q: %w", task.SourceMemoryID, err)
			}
			if mem != nil {
				item.MemoryID = sql.NullString{
					String: task.SourceMemoryID,
					Valid:  true,
				}
			} else {
				log.Printf("cortex: prioritize source memory %q missing; inserting desk item without memory_id", task.SourceMemoryID)
			}
		}

		if err := c.db.InsertDeskItem(item); err != nil {
			return fmt.Errorf("insert desk item %q: %w", task.Title, err)
		}
	}

	return nil
}

func (c *Cortex) runPrune(ctx context.Context) error {
	_ = ctx
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	suppressedDays := 30
	if pruneCfg, ok := c.cfg.Tasks["prune"]; ok && pruneCfg.SuppressedDays > 0 {
		suppressedDays = pruneCfg.SuppressedDays
	}

	report, err := c.db.PruneSuppressedMemories(suppressedDays)
	if err != nil {
		return fmt.Errorf("prune suppressed memories: %w", err)
	}

	log.Printf(
		"cortex: prune candidates=%d deleted=%d skipped_active_edges=%d skipped_references=%d edge_rows_deleted=%d",
		report.Candidates,
		report.Deleted,
		report.SkippedActiveEdges,
		report.SkippedReferences,
		report.EdgeRowsDeleted,
	)
	return nil
}

func (c *Cortex) runDecay(ctx context.Context) error {
	_ = ctx
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	taskCfg, ok := c.cfg.Tasks["decay"]
	if !ok {
		return fmt.Errorf("decay task config missing")
	}
	factor := taskCfg.Factor
	if factor <= 0 || factor >= 1 {
		factor = 0.95
	}
	floor := taskCfg.ImportanceFloor
	if floor <= 0 {
		floor = 0.1
	}

	report, err := c.db.DecayMemories(factor, taskCfg.ExemptTypes, floor, 30)
	if err != nil {
		return fmt.Errorf("decay memories: %w", err)
	}
	log.Printf("cortex: decay updated=%d factor=%.4f floor=%.2f", report.Updated, factor, floor)
	return nil
}

func (c *Cortex) runConsolidation(ctx context.Context) error {
	_ = ctx
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	report, err := c.db.ConsolidateMemories()
	if err != nil {
		return fmt.Errorf("consolidate memories: %w", err)
	}
	log.Printf(
		"cortex: consolidation groups=%d duplicates=%d edges_created=%d suppressed=%d skipped_referenced=%d",
		report.GroupsFound,
		report.DuplicatesFound,
		report.EdgesCreated,
		report.MemoriesSuppressed,
		report.SkippedReferenced,
	)
	return nil
}

func (c *Cortex) runReindex(ctx context.Context) error {
	_ = ctx
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	report, err := c.db.ReindexMemoryCentrality()
	if err != nil {
		return fmt.Errorf("reindex centrality: %w", err)
	}
	log.Printf("cortex: reindex indexed=%d max_degree=%d", report.MemoriesIndexed, report.MaxDegree)
	return nil
}

func toSnippets(memories []db.Memory) []memorySnippet {
	out := make([]memorySnippet, 0, len(memories))
	for _, memory := range memories {
		title := strings.TrimSpace(memory.Title)
		content := strings.TrimSpace(memory.Content)
		if title == "" {
			title = content
		}
		if len(content) > 220 {
			content = content[:220] + "..."
		}

		out = append(out, memorySnippet{
			Type:       memory.Type,
			Title:      title,
			Content:    content,
			Importance: memory.Importance,
			Tags:       strings.TrimSpace(memory.Tags),
			CreatedAt:  memory.CreatedAt,
		})
	}
	return out
}

func truncateWords(text string, maxWords int) string {
	if maxWords <= 0 {
		return strings.TrimSpace(text)
	}
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return strings.TrimSpace(text)
	}
	return strings.Join(words[:maxWords], " ")
}

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

	"scaffold/brain"
	appconfig "scaffold/config"
	"scaffold/db"
	"scaffold/embedding"
	googlemail "scaffold/google"
	"scaffold/llm"
	"scaffold/sessionbus"
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

type CompletionClient = llm.CompletionClient

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
	db                    *db.DB
	brain                 *brain.Brain
	llm                   CompletionClient
	semanticLLM           CompletionClient
	observationsLLM       CompletionClient
	semanticModelName     string
	observationsModelName string
	cfg                   appconfig.CortexConfig
	embedder              embedding.Embedder
	bulletin              *BulletinCache
	tasks                 []*CortexTask
	once                  sync.Once
	gmailClient           *googlemail.GmailClient
	gmailCfg              *googlemail.GmailConfig
	sessionBus            *sessionbus.Bus
}

type LLMRoute struct {
	Client CompletionClient
	Model  string
}

type LLMRoutes struct {
	Bulletin     LLMRoute
	Semantic     LLMRoute
	Observations LLMRoute
}

func NewWithLLM(database *db.DB, b *brain.Brain, cfg appconfig.CortexConfig, embedder embedding.Embedder, routes LLMRoutes) *Cortex {
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

	if routes.Bulletin.Client == nil {
		routes.Bulletin.Client = &llm.UnconfiguredCompletionClient{}
	}
	if strings.TrimSpace(routes.Bulletin.Model) != "" {
		cfg.Bulletin.Model = strings.TrimSpace(routes.Bulletin.Model)
	}
	if routes.Semantic.Client == nil {
		routes.Semantic.Client = routes.Bulletin.Client
	}
	if strings.TrimSpace(routes.Semantic.Model) == "" {
		routes.Semantic.Model = cfg.Bulletin.Model
	}
	if routes.Observations.Client == nil {
		routes.Observations.Client = routes.Semantic.Client
	}
	if strings.TrimSpace(routes.Observations.Model) == "" {
		routes.Observations.Model = strings.TrimSpace(routes.Semantic.Model)
	}

	c := &Cortex{
		db:                    database,
		brain:                 b,
		llm:                   routes.Bulletin.Client,
		semanticLLM:           routes.Semantic.Client,
		observationsLLM:       routes.Observations.Client,
		semanticModelName:     strings.TrimSpace(routes.Semantic.Model),
		observationsModelName: strings.TrimSpace(routes.Observations.Model),
		cfg:                   cfg,
		embedder:              embedder,
		bulletin:              newBulletinCache(maxStale),
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
			interval := time.Duration(taskCfg.IntervalHours) * time.Hour
			if interval <= 0 && taskCfg.IntervalMinutes > 0 {
				interval = time.Duration(taskCfg.IntervalMinutes) * time.Minute
			}
			tasks = append(tasks, &CortexTask{
				Name:     name,
				Interval: interval,
				Timeout:  time.Duration(taskCfg.TimeoutSeconds) * time.Second,
				Fn:       taskFn,
			})
		}
	}

	return tasks
}

func (c *Cortex) SetGmailClient(client *googlemail.GmailClient) {
	c.gmailClient = client
	if c.gmailCfg != nil {
		go c.validateGmailLabels(context.Background())
	}
}

func (c *Cortex) SetGmailConfig(cfg *googlemail.GmailConfig) {
	c.gmailCfg = cfg
	if c.gmailClient != nil {
		go c.validateGmailLabels(context.Background())
	}
}

func (c *Cortex) validateGmailLabels(ctx context.Context) {
	if c.gmailClient == nil || c.gmailCfg == nil {
		return
	}
	labelMap, err := c.gmailClient.ListLabels(ctx)
	if err != nil {
		log.Printf("ERROR: gmail label validation failed: %v", err)
		return
	}
	for _, name := range c.gmailCfg.Labels.Status {
		if _, ok := labelMap[name]; !ok {
			log.Printf("ERROR: gmail status label %q not found in Gmail account — triage will skip it", name)
		}
	}
	for _, name := range c.gmailCfg.Labels.Domain {
		if _, ok := labelMap[name]; !ok {
			log.Printf("ERROR: gmail domain label %q not found in Gmail — triage cannot apply it", name)
		}
	}
	if sys := c.gmailCfg.SystemLabel; sys != "" {
		if _, ok := labelMap[sys]; !ok {
			log.Printf("ERROR: gmail system label %q not found in Gmail — create it manually in the Gmail web UI", sys)
		}
	}
}

func (c *Cortex) SetSessionBus(bus *sessionbus.Bus) {
	c.sessionBus = bus
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
	case "embedding_backfill":
		return c.runEmbeddingBackfill
	case "observations":
		return c.runObservations
	case "drift":
		return c.runDrift
	case "gmail_triage":
		return c.runGmailTriage
	case "waiting_check":
		return c.runWaitingCheck
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

	systemPrompt, userPrompt, maxTokens, err := buildBulletinPrompt(c.cfg.Bulletin.MaxWords, sections)
	if err != nil {
		return err
	}

	content, err := c.llm.CompletionText(ctx, c.cfg.Bulletin.Model, systemPrompt, userPrompt, maxTokens)
	if err != nil {
		return fmt.Errorf("synthesize bulletin: %w", err)
	}
	content = truncateWords(content, c.cfg.Bulletin.MaxWords)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("blank bulletin response")
	}

	if c.bulletin != nil {
		c.bulletin.Set(content)
	}
	return nil
}

func buildBulletinPrompt(maxWords int, sections bulletinSections) (systemPrompt string, userPrompt string, maxTokens int64, err error) {
	systemPrompt = `You are the cortex's memory bulletin synthesizer.
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
		return "", "", 0, fmt.Errorf("marshal sections: %w", err)
	}

	maxTokens = int64(maxWords * 3)
	if maxTokens < bulletinMaxTokensFloor {
		maxTokens = bulletinMaxTokensFloor
	}
	if maxTokens > bulletinMaxTokensCeil {
		maxTokens = bulletinMaxTokensCeil
	}

	userPrompt = fmt.Sprintf("Max words: %d\n\nMemory sections (JSON):\n%s", maxWords, string(payload))
	return systemPrompt, userPrompt, maxTokens, nil
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
				item.DomainID = mem.DomainID
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
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	report, err := c.db.ConsolidateMemories()
	if err != nil {
		return fmt.Errorf("consolidate memories (exact): %w", err)
	}
	log.Printf(
		"cortex: consolidation (exact) groups=%d duplicates=%d edges=%d suppressed=%d",
		report.GroupsFound, report.DuplicatesFound, report.EdgesCreated, report.MemoriesSuppressed,
	)

	if c.embedder == nil || !c.embedder.Available(ctx) {
		log.Printf("cortex: consolidation semantic path skipped (embedder unavailable)")
		return nil
	}
	embeddingModel := strings.TrimSpace(c.embedder.ModelName())
	if embeddingModel == "" {
		log.Printf("cortex: consolidation semantic path skipped (embedder model unavailable)")
		return nil
	}

	const similarityThreshold = 0.85
	const maxLLMCalls = 20
	candidates, err := c.db.FindConsolidationCandidates(similarityThreshold, maxLLMCalls*2, embeddingModel)
	if err != nil {
		log.Printf("cortex: find consolidation candidates: %v", err)
		return nil
	}

	llmCalls := 0
	for _, candidate := range candidates {
		if llmCalls >= maxLLMCalls {
			break
		}

		decision, err := c.consolidationDecision(ctx, candidate)
		if err != nil {
			log.Printf("cortex: consolidation LLM decision failed: %v", err)
			continue
		}
		llmCalls++

		switch decision.Action {
		case "merge":
			keepID := strings.TrimSpace(decision.KeepID)
			if keepID != candidate.MemoryA.ID && keepID != candidate.MemoryB.ID {
				log.Printf(
					"cortex: consolidation merge skipped (invalid keep_id=%q for pair %s,%s)",
					decision.KeepID,
					candidate.MemoryA.ID,
					candidate.MemoryB.ID,
				)
				continue
			}

			loserID := candidate.MemoryA.ID
			winnerID := candidate.MemoryB.ID
			if keepID == candidate.MemoryA.ID {
				loserID = candidate.MemoryB.ID
				winnerID = candidate.MemoryA.ID
			}
			refs, err := c.db.CountMemoryReferences(loserID)
			if err != nil || refs > 0 {
				continue
			}
			if _, err := c.db.EnsureUndirectedEdge(loserID, winnerID, "RelatedTo", 0.9); err != nil {
				log.Printf("cortex: consolidation merge edge: %v", err)
			}
			if err := c.db.SuppressMemory(loserID); err != nil {
				log.Printf("cortex: consolidation suppress: %v", err)
			} else {
				report.MemoriesSuppressed++
			}
		case "relate":
			if _, err := c.db.EnsureUndirectedEdge(candidate.MemoryA.ID, candidate.MemoryB.ID, "RelatedTo", 0.8); err != nil {
				log.Printf("cortex: consolidation relate edge: %v", err)
			} else {
				report.EdgesCreated++
			}
		case "keep_separate":
			// Explicit no-op: this pair is distinct but was worth evaluating.
		default:
			log.Printf(
				"cortex: consolidation skipped unknown decision action %q for pair %s,%s",
				decision.Action,
				candidate.MemoryA.ID,
				candidate.MemoryB.ID,
			)
			continue
		}
		report.PairsEvaluated++
		report.LLMDecisions++
	}

	log.Printf(
		"cortex: consolidation (semantic) pairs_evaluated=%d llm_decisions=%d",
		report.PairsEvaluated, report.LLMDecisions,
	)
	return nil
}

type consolidationDecision struct {
	Action string
	KeepID string
	Reason string
}

func (c *Cortex) consolidationDecision(ctx context.Context, candidate db.ConsolidationCandidate) (consolidationDecision, error) {
	snippetA := candidate.MemoryA.Title
	if len(candidate.MemoryA.Content) < 200 {
		snippetA += " — " + candidate.MemoryA.Content
	} else {
		snippetA += " — " + candidate.MemoryA.Content[:200] + "..."
	}
	snippetB := candidate.MemoryB.Title
	if len(candidate.MemoryB.Content) < 200 {
		snippetB += " — " + candidate.MemoryB.Content
	} else {
		snippetB += " — " + candidate.MemoryB.Content[:200] + "..."
	}

	system := `You compare memory pairs and decide their relationship. Respond with valid JSON only.`
	user := fmt.Sprintf(`Compare these memories and decide the relationship:
A (id=%s): %s
B (id=%s): %s

Options:
- merge: These are duplicates or one supersedes the other. Keep the better one.
- relate: These are related but distinct. Create an edge.
- keep_separate: No meaningful relationship.

Respond with JSON: {"decision": "merge|relate|keep_separate", "keep_id": "%s or %s or empty", "reason": "brief reason"}`,
		candidate.MemoryA.ID, snippetA,
		candidate.MemoryB.ID, snippetB,
		candidate.MemoryA.ID, candidate.MemoryB.ID,
	)

	raw, err := c.semanticClient().CompletionJSON(ctx, c.semanticModel(), system, user, 150)
	if err != nil {
		return consolidationDecision{}, err
	}

	var result struct {
		Decision string `json:"decision"`
		KeepID   string `json:"keep_id"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return consolidationDecision{}, fmt.Errorf("parse decision: %w (raw: %s)", err, raw)
	}

	return consolidationDecision{
		Action: strings.ToLower(strings.TrimSpace(result.Decision)),
		KeepID: strings.TrimSpace(result.KeepID),
		Reason: strings.TrimSpace(result.Reason),
	}, nil
}

func (c *Cortex) semanticModel() string {
	if c == nil {
		return "claude-haiku-4-5"
	}
	if model := strings.TrimSpace(c.semanticModelName); model != "" {
		return model
	}
	model := strings.TrimSpace(c.cfg.Bulletin.Model)
	if model == "" {
		return "claude-haiku-4-5"
	}
	return model
}

func (c *Cortex) observationsModel() string {
	if c == nil {
		return "claude-haiku-4-5"
	}
	if model := strings.TrimSpace(c.observationsModelName); model != "" {
		return model
	}
	return c.semanticModel()
}

func (c *Cortex) semanticClient() CompletionClient {
	if c == nil {
		return &llm.UnconfiguredCompletionClient{}
	}
	if c.semanticLLM != nil {
		return c.semanticLLM
	}
	if c.llm != nil {
		return c.llm
	}
	return &llm.UnconfiguredCompletionClient{}
}

func (c *Cortex) observationsClient() CompletionClient {
	if c == nil {
		return &llm.UnconfiguredCompletionClient{}
	}
	if c.observationsLLM != nil {
		return c.observationsLLM
	}
	return c.semanticClient()
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

func (c *Cortex) runDrift(ctx context.Context) error {
	_ = ctx
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	drifts, err := c.db.ComputeDriftStates()
	if err != nil {
		return fmt.Errorf("drift: %w", err)
	}

	counts := map[string]int{}
	for _, d := range drifts {
		counts[d.State]++
	}
	log.Printf("cortex: drift domains=%d active=%d drifting=%d neglected=%d cold=%d overactive=%d",
		len(drifts), counts["active"], counts["drifting"], counts["neglected"], counts["cold"], counts["overactive"])
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

func (c *Cortex) runEmbeddingBackfill(ctx context.Context) error {
	if c.embedder == nil {
		return fmt.Errorf("embedder not configured")
	}
	if !c.embedder.Available(ctx) {
		log.Printf("cortex: embedding_backfill skipped (embedder unavailable)")
		return nil
	}
	modelName := strings.TrimSpace(c.embedder.ModelName())
	if modelName == "" {
		return fmt.Errorf("embedder model name is empty")
	}

	const batchSize = 50
	processed := 0

	jobs, err := c.db.DequeueEmbeddingJobs(batchSize)
	if err != nil {
		return fmt.Errorf("dequeue embedding jobs: %w", err)
	}
	if len(jobs) > 0 {
		type jobMem struct {
			job db.EmbeddingJob
			mem *db.Memory
		}
		var valid []jobMem
		for _, j := range jobs {
			mem, err := c.db.GetMemory(j.MemoryID)
			if err != nil || mem == nil {
				_ = c.db.DeleteEmbeddingJob(j.MemoryID)
				continue
			}
			valid = append(valid, jobMem{j, mem})
		}
		if len(valid) > 0 {
			texts := make([]string, len(valid))
			for i, v := range valid {
				texts[i] = v.mem.Title + " " + v.mem.Content
			}
			embeddings, err := c.embedder.EmbedBatch(ctx, texts)
			if err != nil {
				for _, v := range valid {
					_ = c.db.IncrementEmbeddingJobAttempts(v.job.MemoryID)
				}
				log.Printf("cortex: embedding_backfill batch failed: %v", err)
			} else {
				for i, v := range valid {
					if i >= len(embeddings) {
						break
					}
					if err := c.db.UpsertEmbedding(v.job.MemoryID, embeddings[i], modelName); err != nil {
						log.Printf("cortex: upsert embedding %s: %v", v.job.MemoryID, err)
						_ = c.db.IncrementEmbeddingJobAttempts(v.job.MemoryID)
					} else {
						_ = c.db.DeleteEmbeddingJob(v.job.MemoryID)
						processed++
					}
				}
			}
		}
	}

	ids, err := c.db.ListMemoriesWithoutEmbedding(batchSize)
	if err != nil {
		return fmt.Errorf("list memories without embedding: %w", err)
	}
	if len(ids) > 0 {
		texts := make([]string, 0, len(ids))
		mems := make([]*db.Memory, 0, len(ids))
		for _, id := range ids {
			mem, err := c.db.GetMemory(id)
			if err != nil || mem == nil {
				continue
			}
			texts = append(texts, mem.Title+" "+mem.Content)
			mems = append(mems, mem)
		}
		if len(texts) > 0 {
			embeddings, err := c.embedder.EmbedBatch(ctx, texts)
			if err != nil {
				log.Printf("cortex: embedding_backfill backfill batch failed: %v", err)
			} else {
				for i, mem := range mems {
					if i >= len(embeddings) {
						break
					}
					if err := c.db.UpsertEmbedding(mem.ID, embeddings[i], modelName); err != nil {
						log.Printf("cortex: upsert embedding %s: %v", mem.ID, err)
					} else {
						processed++
					}
				}
			}
		}
	}

	log.Printf("cortex: embedding_backfill processed=%d", processed)
	return nil
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

package ingestion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"scaffold/brain"
	"scaffold/db"
)

const (
	defaultPollInterval = 30 * time.Second
	maxChunkChars       = 4000
)

var supportedExtensions = map[string]struct{}{
	".txt":      {},
	".md":       {},
	".markdown": {},
	".json":     {},
	".yaml":     {},
	".yml":      {},
	".csv":      {},
	".log":      {},
	".pdf":      {},
}

type Service struct {
	db           *db.DB
	brain        *brain.Brain
	dir          string
	pollInterval time.Duration
	ingestMu     sync.Mutex
}

func New(database *db.DB, b *brain.Brain, dir string, pollInterval time.Duration) (*Service, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("ingestion directory is required")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve ingestion directory: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, fmt.Errorf("create ingestion directory: %w", err)
	}
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	return &Service{
		db:           database,
		brain:        b,
		dir:          absDir,
		pollInterval: pollInterval,
	}, nil
}

func (s *Service) Directory() string {
	return s.dir
}

func (s *Service) Start(ctx context.Context) {
	go func() {
		s.run(ctx)
	}()
}

func (s *Service) Upload(_ context.Context, filename string, r io.Reader) (string, error) {
	if r == nil {
		return "", fmt.Errorf("upload reader is required")
	}
	base := sanitizeUploadFilename(filename)
	if base == "" {
		return "", fmt.Errorf("filename is required")
	}
	ext := strings.ToLower(filepath.Ext(base))
	if _, ok := supportedExtensions[ext]; !ok {
		return "", fmt.Errorf("unsupported file extension %q", ext)
	}

	destName := fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102-150405"), base)
	destPath := filepath.Join(s.dir, destName)
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("create upload file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write upload file: %w", err)
	}
	return destPath, nil
}

func (s *Service) IngestNow(ctx context.Context) error {
	if !s.ingestMu.TryLock() {
		return nil
	}
	defer s.ingestMu.Unlock()

	files, err := s.scanFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := s.processFile(ctx, file.path); err != nil {
			log.Printf("ingestion: %s failed: %v", file.path, err)
		}
	}
	return nil
}

func (s *Service) run(ctx context.Context) {
	if err := s.IngestNow(ctx); err != nil {
		log.Printf("ingestion: initial scan failed: %v", err)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.IngestNow(ctx); err != nil {
				log.Printf("ingestion: scan failed: %v", err)
			}
		}
	}
}

type fileCandidate struct {
	path    string
	modTime time.Time
}

func (s *Service) scanFiles() ([]fileCandidate, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read ingestion directory: %w", err)
	}

	out := make([]fileCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := supportedExtensions[ext]; !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, fileCandidate{
			path:    filepath.Join(s.dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].modTime.Before(out[j].modTime)
	})
	return out, nil
}

func (s *Service) processFile(ctx context.Context, path string) error {
	text, rawBytes, err := s.loadText(ctx, path)
	if err != nil {
		_ = s.db.UpsertIngestionFile(db.IngestionFile{
			Path:      path,
			FileHash:  "unavailable",
			Status:    "failed",
			LastError: sql.NullString{String: err.Error(), Valid: true},
		})
		return err
	}

	fileHash := sha256Hex(rawBytes)
	chunks := chunkDocument(path, text, maxChunkChars)

	if err := s.db.UpsertIngestionFile(db.IngestionFile{
		Path:            path,
		FileHash:        fileHash,
		Status:          "processing",
		TotalChunks:     len(chunks),
		ProcessedChunks: 0,
	}); err != nil {
		return fmt.Errorf("mark file processing: %w", err)
	}

	processedChunks := 0
	for idx, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			processedChunks++
			continue
		}

		chunkHash := sha256Hex([]byte(chunk))
		chunkKey := scopedChunkHash(fileHash, chunkHash)
		done, err := s.db.IsIngestionChunkCompleted(chunkKey)
		if err != nil {
			return fmt.Errorf("check chunk completion: %w", err)
		}
		if done {
			processedChunks++
			continue
		}

		memoryID, ingestErr := s.ingestChunk(ctx, filepath.Base(path), idx, chunk)
		status := "completed"
		var errField sql.NullString
		var memoryIDField sql.NullString
		if strings.TrimSpace(memoryID) != "" {
			memoryIDField = sql.NullString{String: memoryID, Valid: true}
		}
		if ingestErr != nil {
			status = "failed"
			errField = sql.NullString{String: ingestErr.Error(), Valid: true}
		}
		if err := s.db.UpsertIngestionChunk(db.IngestionChunk{
			ChunkHash:  chunkKey,
			FilePath:   path,
			FileHash:   fileHash,
			ChunkIndex: idx,
			Status:     status,
			MemoryID:   memoryIDField,
			Error:      errField,
		}); err != nil {
			return fmt.Errorf("record chunk progress: %w", err)
		}

		if ingestErr != nil {
			_ = s.db.UpsertIngestionFile(db.IngestionFile{
				Path:            path,
				FileHash:        fileHash,
				Status:          "failed",
				TotalChunks:     len(chunks),
				ProcessedChunks: processedChunks,
				LastError:       sql.NullString{String: ingestErr.Error(), Valid: true},
			})
			return fmt.Errorf("ingest chunk %d: %w", idx, ingestErr)
		}

		processedChunks++
		if err := s.db.UpsertIngestionFile(db.IngestionFile{
			Path:            path,
			FileHash:        fileHash,
			Status:          "processing",
			TotalChunks:     len(chunks),
			ProcessedChunks: processedChunks,
		}); err != nil {
			return fmt.Errorf("update file progress: %w", err)
		}
	}

	if err := os.Remove(path); err != nil {
		_ = s.db.UpsertIngestionFile(db.IngestionFile{
			Path:            path,
			FileHash:        fileHash,
			Status:          "failed",
			TotalChunks:     len(chunks),
			ProcessedChunks: processedChunks,
			LastError:       sql.NullString{String: err.Error(), Valid: true},
		})
		return fmt.Errorf("delete processed file: %w", err)
	}

	if err := s.db.UpsertIngestionFile(db.IngestionFile{
		Path:            path,
		FileHash:        fileHash,
		Status:          "completed",
		TotalChunks:     len(chunks),
		ProcessedChunks: processedChunks,
	}); err != nil {
		return fmt.Errorf("mark file completed: %w", err)
	}

	return nil
}

func scopedChunkHash(fileHash, chunkHash string) string {
	return fileHash + ":" + chunkHash
}

func (s *Service) loadText(ctx context.Context, path string) (string, []byte, error) {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		text, err := extractPDFText(ctx, path)
		if err != nil {
			return "", nil, err
		}
		return text, rawBytes, nil
	}

	if !utf8.Valid(rawBytes) {
		return "", nil, fmt.Errorf("file is not valid UTF-8")
	}
	return string(rawBytes), rawBytes, nil
}

func extractPDFText(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-nopgbrk", path, "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("extract PDF text (requires pdftotext): %w", err)
	}
	return string(out), nil
}

func (s *Service) ingestChunk(ctx context.Context, fileName string, chunkIndex int, chunk string) (string, error) {
	memType := "Observation"
	title := deriveTitle(chunk, fileName, chunkIndex)
	importance := 0.45
	var tags []string
	var domainID sql.NullInt64

	if s.brain != nil {
		triage, err := s.brain.Triage(ctx, chunk)
		if err != nil {
			log.Printf("ingestion: triage degraded for %s chunk %d: %v", fileName, chunkIndex+1, err)
		}
		if triage != nil {
			if t := canonicalMemoryType(triage.Type); t != "" {
				memType = t
			}
			if strings.TrimSpace(triage.Title) != "" {
				title = strings.TrimSpace(triage.Title)
			}
			if triage.Importance > 0 {
				importance = triage.Importance
			}
			if len(triage.Tags) > 0 {
				tags = append(tags, triage.Tags...)
			}
			resolvedName := strings.TrimSpace(triage.Domain)
			if resolvedName == "" {
				resolvedName = "Personal Development"
			}
			resolved, resolveErr := s.db.ResolveDomainID(resolvedName)
			if resolveErr != nil {
				log.Printf("ingestion: resolve domain %q failed: %v", resolvedName, resolveErr)
			} else if resolved != nil {
				domainID = sql.NullInt64{Int64: int64(*resolved), Valid: true}
			}
		}
	}

	importance = clampImportance(importance)
	tags = normalizeTags(append(tags, "ingest"))

	memoryID := uuid.New().String()
	mem := db.Memory{
		ID:         memoryID,
		Type:       memType,
		Content:    chunk,
		Title:      title,
		Importance: importance,
		Source:     fmt.Sprintf("ingest:%s#%d", fileName, chunkIndex+1),
		Tags:       strings.Join(tags, ","),
		DomainID:   domainID,
	}
	if err := s.db.InsertMemory(mem); err != nil {
		return "", fmt.Errorf("insert memory: %w", err)
	}
	return memoryID, nil
}

func chunkByLines(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = maxChunkChars
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if normalized == "" {
		return nil
	}

	lines := strings.SplitAfter(normalized, "\n")
	out := make([]string, 0, len(lines))
	var b strings.Builder

	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		if b.Len() == 0 {
			if len(line) > maxChars {
				out = append(out, line)
				continue
			}
			b.WriteString(line)
			continue
		}
		if b.Len()+len(line) > maxChars {
			flush()
			if len(line) > maxChars {
				out = append(out, line)
				continue
			}
		}
		b.WriteString(line)
	}
	flush()
	return out
}

func chunkDocument(path, text string, maxChars int) []string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return chunkByLines(text, maxChars)
	}

	sections := splitMarkdownSections(text)
	if len(sections) <= 1 {
		return chunkByLines(text, maxChars)
	}

	out := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		sectionChunks := chunkByLines(section+"\n", maxChars)
		for _, chunk := range sectionChunks {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			out = append(out, chunk)
		}
	}
	if len(out) == 0 {
		return chunkByLines(text, maxChars)
	}
	return out
}

func splitMarkdownSections(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return nil
	}

	lines := strings.SplitAfter(normalized, "\n")
	out := make([]string, 0, 8)
	var b strings.Builder
	seenBreak := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isSectionHeading := strings.HasPrefix(trimmed, "## ")
		if isSectionHeading && b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
			seenBreak = true
		}
		b.WriteString(line)
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}

	if !seenBreak {
		return []string{normalized}
	}
	return out
}

func deriveTitle(chunk, fileName string, chunkIndex int) string {
	for _, line := range strings.Split(chunk, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 96 {
			return trimmed[:96]
		}
		return trimmed
	}

	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Ingested Document"
	}
	return fmt.Sprintf("%s (chunk %d)", base, chunkIndex+1)
}

func canonicalMemoryType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "identity":
		return "Identity"
	case "goal":
		return "Goal"
	case "decision":
		return "Decision"
	case "todo":
		return "Todo"
	case "preference":
		return "Preference"
	case "fact":
		return "Fact"
	case "event":
		return "Event"
	case "observation":
		return "Observation"
	case "idea":
		return "Observation"
	default:
		return ""
	}
}

func clampImportance(v float64) float64 {
	if v < 0.05 {
		return 0.05
	}
	if v > 1 {
		return 1
	}
	return v
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func sanitizeUploadFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

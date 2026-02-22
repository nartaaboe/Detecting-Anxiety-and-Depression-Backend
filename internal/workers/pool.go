package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/ai"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/repositories"
)

var ErrQueueFull = errors.New("queue is full")
var ErrQueueClosed = errors.New("queue is closed")

type Pool struct {
	jobs chan uuid.UUID

	analyses *repositories.AnalysesRepo
	results  *repositories.ResultsRepo
	ai       *ai.Client
	logger   *slog.Logger

	wg sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

func NewPool(workersCount int, queueSize int, analyses *repositories.AnalysesRepo, results *repositories.ResultsRepo, aiClient *ai.Client, logger *slog.Logger) *Pool {
	if queueSize <= 0 {
		queueSize = 1024
	}
	if workersCount <= 0 {
		workersCount = 1
	}

	return &Pool{
		jobs:     make(chan uuid.UUID, queueSize),
		analyses: analyses,
		results:  results,
		ai:       aiClient,
		logger:   logger,
	}
}

func (p *Pool) Start(ctx context.Context, workersCount int) {
	if workersCount <= 0 {
		workersCount = 1
	}

	for i := 0; i < workersCount; i++ {
		p.wg.Add(1)
		go func(workerID int) {
			defer p.wg.Done()
			p.runWorker(ctx, workerID)
		}(i + 1)
	}
}

func (p *Pool) Enqueue(analysisID uuid.UUID) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()

	if closed {
		return ErrQueueClosed
	}

	select {
	case p.jobs <- analysisID:
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *Pool) Stop() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.jobs)
	p.mu.Unlock()
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) runWorker(ctx context.Context, workerID int) {
	logger := p.logger
	if logger != nil {
		logger = logger.With(slog.Int("worker_id", workerID))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case analysisID, ok := <-p.jobs:
			if !ok {
				return
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						if logger != nil {
							logger.Error("worker panic", slog.Any("recover", r), slog.String("analysis_id", analysisID.String()))
						}
						_ = p.analyses.MarkFailed(context.Background(), analysisID, time.Now().UTC(), "worker panic")
					}
				}()

				p.process(ctx, logger, analysisID)
			}()
		}
	}
}

func (p *Pool) process(ctx context.Context, logger *slog.Logger, analysisID uuid.UUID) {
	now := time.Now().UTC()

	analysis, ok, err := p.analyses.MarkRunning(ctx, analysisID, now)
	if err != nil {
		if logger != nil {
			logger.Error("mark analysis running failed", slog.String("analysis_id", analysisID.String()), slog.Any("err", err))
		}
		return
	}
	if !ok {
		return
	}

	content, err := p.analyses.GetTextContentForAnalysis(ctx, analysisID)
	if err != nil {
		_ = p.analyses.MarkFailed(ctx, analysisID, time.Now().UTC(), clipErr(err))
		if logger != nil {
			logger.Error("load text failed", slog.String("analysis_id", analysisID.String()), slog.Any("err", err))
		}
		return
	}

	aiResp, err := p.ai.Infer(ctx, ai.InferRequest{
		Text:         content,
		ModelVersion: analysis.ModelVersion,
		Threshold:    analysis.Threshold,
	})
	if err != nil {
		_ = p.analyses.MarkFailed(ctx, analysisID, time.Now().UTC(), clipErr(err))
		if logger != nil {
			logger.Error("ai infer failed", slog.String("analysis_id", analysisID.String()), slog.Any("err", err))
		}
		return
	}

	explJSON, err := json.Marshal(aiResp.Explanation)
	if err != nil {
		_ = p.analyses.MarkFailed(ctx, analysisID, time.Now().UTC(), clipErr(err))
		if logger != nil {
			logger.Error("marshal explanation failed", slog.String("analysis_id", analysisID.String()), slog.Any("err", err))
		}
		return
	}

	if _, err := p.results.Upsert(ctx, analysisID, aiResp.Label, aiResp.Score, aiResp.Confidence, explJSON); err != nil {
		_ = p.analyses.MarkFailed(ctx, analysisID, time.Now().UTC(), clipErr(err))
		if logger != nil {
			logger.Error("save result failed", slog.String("analysis_id", analysisID.String()), slog.Any("err", err))
		}
		return
	}

	if err := p.analyses.MarkDone(ctx, analysisID, time.Now().UTC()); err != nil {
		if logger != nil {
			logger.Error("mark done failed", slog.String("analysis_id", analysisID.String()), slog.Any("err", err))
		}
		return
	}

	if logger != nil {
		logger.Info("analysis done", slog.String("analysis_id", analysisID.String()))
	}
}

func clipErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	const max = 1000
	if len(s) <= max {
		return s
	}
	return fmt.Sprintf("%s...(%d bytes)", s[:max], len(s))
}

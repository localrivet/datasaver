package backup

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	engine   *Engine
	cron     *cron.Cron
	schedule string
	logger   *slog.Logger
	mu       sync.RWMutex
	running  bool
	nextRun  time.Time
}

func NewScheduler(engine *Engine, schedule string, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		engine:   engine,
		schedule: schedule,
		logger:   logger,
		cron:     cron.New(cron.WithSeconds()),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	schedule := "0 " + s.schedule

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.runBackup(ctx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()

	entry := s.cron.Entry(entryID)
	s.mu.Lock()
	s.nextRun = entry.Next
	s.mu.Unlock()

	s.logger.Info("scheduler started",
		"schedule", s.schedule,
		"next_run", s.nextRun,
	)

	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.running = false
	s.logger.Info("scheduler stopped")
}

func (s *Scheduler) RunNow(ctx context.Context) (*BackupResult, error) {
	return s.engine.Run(ctx)
}

func (s *Scheduler) NextRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextRun
}

func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Scheduler) runBackup(ctx context.Context) {
	s.logger.Info("scheduled backup starting")

	result, err := s.engine.Run(ctx)
	if err != nil {
		s.logger.Error("scheduled backup failed", "error", err)
	} else {
		s.logger.Info("scheduled backup completed", "id", result.ID)
	}

	_, err = s.engine.Cleanup(ctx)
	if err != nil {
		s.logger.Error("cleanup after backup failed", "error", err)
	}

	entries := s.cron.Entries()
	if len(entries) > 0 {
		s.mu.Lock()
		s.nextRun = entries[0].Next
		s.mu.Unlock()
	}
}

func (s *Scheduler) Engine() *Engine {
	return s.engine
}

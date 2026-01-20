package agent

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

var (
	// ErrPoolFull is returned when the task queue is full
	ErrPoolFull = errors.New("agent worker pool is full, please try again later")
	// ErrPoolShutdown is returned when submitting to a shutdown pool
	ErrPoolShutdown = errors.New("agent worker pool is shutting down")
)

// AgentTask represents a task to be executed by the worker pool
type AgentTask struct {
	SessionID   string
	Prompt      string
	Attachments []message.Attachment
	// ResultChan receives the result or error when task completes
	ResultChan chan AgentTaskResult
	// CreatedAt is when the task was created
	CreatedAt time.Time
}

// AgentTaskResult holds the result of an agent task execution
type AgentTaskResult struct {
	Error error
}

// PoolStats holds statistics about the worker pool
type PoolStats struct {
	ActiveWorkers  int64
	QueuedTasks    int
	TotalTasks     int64
	CompletedTasks int64
	FailedTasks    int64
}

// AgentWorkerPool manages a pool of workers for executing agent tasks
type AgentWorkerPool interface {
	// Submit submits a task to the pool. Returns ErrPoolFull if queue is full.
	Submit(ctx context.Context, task AgentTask) error
	// Shutdown gracefully shuts down the pool, waiting for running tasks to complete
	Shutdown(ctx context.Context) error
	// Stats returns current pool statistics
	Stats() PoolStats
	// IsShutdown returns true if the pool is shutting down or shut down
	IsShutdown() bool
}

// TaskExecutor is the function type for executing agent tasks
type TaskExecutor func(ctx context.Context, task AgentTask) error

// TaskLifecycleCallback is called when task starts or completes
// For OnComplete: err is the error from task execution (nil if success), reason is "completed", "error", "timeout", "cancelled", "shutdown"
type TaskLifecycleCallback func(sessionID string, err error, reason string)

// agentWorkerPool implements AgentWorkerPool
type agentWorkerPool struct {
	cfg      *config.AgentConfig
	executor TaskExecutor

	// Lifecycle callbacks
	onTaskStart    TaskLifecycleCallback // Called when worker starts executing a task
	onTaskComplete TaskLifecycleCallback // Called when worker finishes executing a task

	// Task queue - buffered channel
	taskQueue chan AgentTask

	// Semaphore for worker count control
	workerSem chan struct{}

	// Statistics
	activeWorkers  atomic.Int64
	totalTasks     atomic.Int64
	completedTasks atomic.Int64
	failedTasks    atomic.Int64

	// Shutdown control
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
	isShutdown   atomic.Bool

	// WaitGroup for tracking active workers
	wg sync.WaitGroup

	// Worker ID counter for logging
	workerIDCounter atomic.Int64
}

// NewAgentWorkerPool creates a new agent worker pool
// onTaskStart is called when a worker starts executing a task (can be nil)
// onTaskComplete is called when a worker finishes executing a task (can be nil)
func NewAgentWorkerPool(cfg *config.AgentConfig, executor TaskExecutor, onTaskStart, onTaskComplete TaskLifecycleCallback) AgentWorkerPool {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 100 // default
	}
	if cfg.TaskQueueSize <= 0 {
		cfg.TaskQueueSize = 1000 // default
	}

	pool := &agentWorkerPool{
		cfg:            cfg,
		executor:       executor,
		onTaskStart:    onTaskStart,
		onTaskComplete: onTaskComplete,
		taskQueue:      make(chan AgentTask, cfg.TaskQueueSize),
		workerSem:      make(chan struct{}, cfg.MaxWorkers),
		shutdownCh:     make(chan struct{}),
	}

	// Start the dispatcher goroutine
	go pool.dispatcher()

	slog.Info("[GOROUTINE] Agent worker pool initialized",
		"max_workers", cfg.MaxWorkers,
		"queue_size", cfg.TaskQueueSize,
		"permission_timeout_sec", cfg.PermissionTimeout,
		"task_timeout_sec", cfg.TaskTimeout,
	)

	return pool
}

// Submit submits a task to the pool
func (p *agentWorkerPool) Submit(ctx context.Context, task AgentTask) error {
	if p.isShutdown.Load() {
		return ErrPoolShutdown
	}

	task.CreatedAt = time.Now()
	p.totalTasks.Add(1)

	// Try to submit without blocking
	select {
	case p.taskQueue <- task:
		slog.Info("[GOROUTINE] Task submitted to queue",
			"session_id", task.SessionID,
			"queue_size", len(p.taskQueue),
			"active_workers", p.activeWorkers.Load(),
		)
		return nil
	default:
		// Queue is full
		p.failedTasks.Add(1)
		slog.Warn("[GOROUTINE] Task rejected - queue full",
			"session_id", task.SessionID,
			"queue_size", len(p.taskQueue),
			"max_queue_size", p.cfg.TaskQueueSize,
		)
		return ErrPoolFull
	}
}

// dispatcher runs in a goroutine and dispatches tasks to workers
func (p *agentWorkerPool) dispatcher() {
	slog.Info("[GOROUTINE] Worker pool dispatcher started")

	for {
		select {
		case <-p.shutdownCh:
			slog.Info("[GOROUTINE] Worker pool dispatcher shutting down")
			return
		case task := <-p.taskQueue:
			// Acquire worker slot (blocks if all workers are busy)
			select {
			case <-p.shutdownCh:
				// Return task result with shutdown error
				if task.ResultChan != nil {
					task.ResultChan <- AgentTaskResult{Error: ErrPoolShutdown}
				}
				return
			case p.workerSem <- struct{}{}:
				// Got a worker slot, start worker goroutine
				p.wg.Add(1)
				workerID := p.workerIDCounter.Add(1)
				go p.worker(workerID, task)
			}
		}
	}
}

// worker executes a single task
func (p *agentWorkerPool) worker(workerID int64, task AgentTask) {
	startTime := time.Now()
	p.activeWorkers.Add(1)

	slog.Info("[GOROUTINE] 🚀 Agent worker started",
		"worker_id", workerID,
		"session_id", task.SessionID,
		"queue_wait_ms", startTime.Sub(task.CreatedAt).Milliseconds(),
		"active_workers", p.activeWorkers.Load(),
	)

	// Call onTaskStart callback - this is where session status should be set to "running"
	if p.onTaskStart != nil {
		p.onTaskStart(task.SessionID, nil, "started")
	}

	// Create context with task timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.cfg.TaskTimeout)*time.Second)

	// Execute the task
	var err error
	var reason string

	// Check for shutdown before executing
	select {
	case <-p.shutdownCh:
		err = ErrPoolShutdown
		reason = "shutdown"
	default:
		err = p.executor(ctx, task)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				reason = "timeout"
			} else if errors.Is(err, context.Canceled) {
				reason = "cancelled"
			} else {
				reason = "error"
			}
			p.failedTasks.Add(1)
		} else {
			reason = "completed"
			p.completedTasks.Add(1)
		}
	}

	// Cancel context
	cancel()

	// Call onTaskComplete callback - this is where session status should be set to final status
	if p.onTaskComplete != nil {
		p.onTaskComplete(task.SessionID, err, reason)
	}

	// Send result back
	if task.ResultChan != nil {
		select {
		case task.ResultChan <- AgentTaskResult{Error: err}:
		default:
			// Result channel full or closed, log and continue
			slog.Warn("[GOROUTINE] Failed to send task result",
				"worker_id", workerID,
				"session_id", task.SessionID,
				"reason", reason,
			)
		}
	}

	slog.Info("[GOROUTINE] Task execution finished",
		"worker_id", workerID,
		"session_id", task.SessionID,
		"reason", reason,
		"error", err,
	)

	// Release worker slot and update stats
	<-p.workerSem
	p.activeWorkers.Add(-1)
	p.wg.Done()

	duration := time.Since(startTime)
	slog.Info("[GOROUTINE] 🛑 Agent worker exited",
		"worker_id", workerID,
		"session_id", task.SessionID,
		"duration_ms", duration.Milliseconds(),
		"active_workers", p.activeWorkers.Load(),
	)
}

// Shutdown gracefully shuts down the pool
func (p *agentWorkerPool) Shutdown(ctx context.Context) error {
	p.shutdownOnce.Do(func() {
		slog.Info("[GOROUTINE] Agent worker pool shutdown initiated")
		p.isShutdown.Store(true)
		close(p.shutdownCh)
	})

	// Wait for all workers to complete or context to be cancelled
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("[GOROUTINE] Agent worker pool shutdown complete",
			"completed_tasks", p.completedTasks.Load(),
			"failed_tasks", p.failedTasks.Load(),
		)
		return nil
	case <-ctx.Done():
		slog.Warn("[GOROUTINE] Agent worker pool shutdown timeout, some workers may still be running",
			"active_workers", p.activeWorkers.Load(),
		)
		return ctx.Err()
	}
}

// Stats returns current pool statistics
func (p *agentWorkerPool) Stats() PoolStats {
	return PoolStats{
		ActiveWorkers:  p.activeWorkers.Load(),
		QueuedTasks:    len(p.taskQueue),
		TotalTasks:     p.totalTasks.Load(),
		CompletedTasks: p.completedTasks.Load(),
		FailedTasks:    p.failedTasks.Load(),
	}
}

// IsShutdown returns true if the pool is shutting down
func (p *agentWorkerPool) IsShutdown() bool {
	return p.isShutdown.Load()
}

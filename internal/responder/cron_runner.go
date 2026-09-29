package responder

import (
	"context"
	"fmt"
	"time"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
)

// CronRunner executes scheduled responder jobs
type CronRunner struct {
	db          *queries.Base
	xmppWorker  *XMPPWorker
	checkTicker time.Duration
}

// NewCronRunner creates a new cron runner
func NewCronRunner(db *queries.Base, xmppWorker *XMPPWorker) *CronRunner {
	return &CronRunner{
		db:          db,
		xmppWorker:  xmppWorker,
		checkTicker: 1 * time.Minute,
	}
}

// Start begins the cron runner loop
func (r *CronRunner) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.checkTicker)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.checkAndRunJobs(ctx); err != nil {
				return fmt.Errorf("check and run jobs: %w", err)
			}
		}
	}
}

// checkAndRunJobs checks all crontab jobs and runs those that are due
func (r *CronRunner) checkAndRunJobs(ctx context.Context) error {
	jobs, err := r.db.ListCrontabJobs(ctx)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	now := time.Now()
	for _, job := range jobs {
		if !r.shouldRun(&job, now) {
			continue
		}

		if err := r.executeJob(ctx, job.JobType); err != nil {
			// ponytail: log error but continue - one job failure shouldn't stop all jobs
			continue
		}

		// Update last run
		// ponytail: direct SQL - add UpdateLastRun method if this pattern repeats
		_, err := r.db.ResponderQueries.DB.ExecContext(ctx, `UPDATE crontab_job SET last_run = ? WHERE id = ?`, now.Unix(), job.ID)
		if err != nil {
			return fmt.Errorf("update job last run: %w", err)
		}
	}

	return nil
}

// shouldRun determines if a job should run based on its schedule
func (r *CronRunner) shouldRun(job *models.CrontabJob, now time.Time) bool {
	if !job.Enabled {
		return false
	}

	if job.LastRun == nil {
		return true // Never run before
	}

	lastRun := time.Unix(*job.LastRun, 0)
	interval := r.parseInterval(job.Interval)
	nextRun := lastRun.Add(interval)

	return now.After(nextRun) || now.Equal(nextRun)
}

// parseInterval converts interval string to duration
func (r *CronRunner) parseInterval(interval string) time.Duration {
	switch interval {
	case "5min":
		return 5 * time.Minute
	case "15min":
		return 15 * time.Minute
	case "1hr":
		return 1 * time.Hour
	case "6hr":
		return 6 * time.Hour
	case "daily":
		return 24 * time.Hour
	default:
		return 5 * time.Minute // fallback
	}
}

// executeJob runs a specific job type
func (r *CronRunner) executeJob(ctx context.Context, jobType string) error {
	switch jobType {
	case "xmpp_check":
		if r.xmppWorker == nil {
			return nil // Worker not configured
		}
		return r.xmppWorker.fetchMessages(ctx)

	case "cleanup_inactive":
		cutoff := time.Now().Add(-30 * 24 * time.Hour) // 30 days
		_, err := r.db.ArchiveInactiveCustomers(ctx, cutoff)
		return err

	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}
}

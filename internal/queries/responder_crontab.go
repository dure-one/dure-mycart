package queries

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
)

// ListCrontabJobs returns all crontab jobs
func (r *ResponderQueries) ListCrontabJobs(ctx context.Context) ([]models.CrontabJob, error) {
	query := fmt.Sprintf(`
		SELECT id, job_type, interval, enabled, last_run, next_run, %s, %s
		FROM crontab_job
		ORDER BY created DESC
	`, r.DB.Dialect().Epoch("created"), r.DB.Dialect().Epoch("updated"))

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query crontab jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.CrontabJob
	for rows.Next() {
		var j models.CrontabJob
		var lastRun, nextRun sql.NullInt64
		var created, updated sql.NullInt64

		err := rows.Scan(
			&j.ID, &j.JobType, &j.Interval, &j.Enabled,
			&lastRun, &nextRun, &created, &updated,
		)
		if err != nil {
			return nil, fmt.Errorf("scan crontab job: %w", err)
		}

		j.Created = created.Int64
		j.Updated = updated.Int64

		if lastRun.Valid {
			j.LastRun = &lastRun.Int64
		}
		if nextRun.Valid {
			j.NextRun = &nextRun.Int64
		}

		jobs = append(jobs, j)
	}

	return jobs, nil
}

// UpdateCrontabJob updates a crontab job's settings
func (r *ResponderQueries) UpdateCrontabJob(ctx context.Context, jobID string, enabled bool, interval string) error {
	// Validate interval
	validIntervals := []string{"5min", "15min", "1hr", "6hr", "daily"}
	valid := false
	for _, v := range validIntervals {
		if interval == v {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid interval: must be one of 5min, 15min, 1hr, 6hr, daily")
	}

	result, err := r.DB.ExecContext(ctx, `
		UPDATE crontab_job
		SET enabled = ?, interval = ?, updated = CURRENT_TIMESTAMP
		WHERE id = ?
	`, enabled, interval, jobID)

	if err != nil {
		return fmt.Errorf("update crontab job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return errors.ErrNotFound
	}

	return nil
}

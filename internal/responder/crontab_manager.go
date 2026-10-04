package responder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dure-one/dure-mycart/internal/models"
)

const (
	crontabMarkerStart = "# mycart-responder-start"
	crontabMarkerEnd   = "# mycart-responder-end"
)

// CrontabManager manages system crontab entries
type CrontabManager struct {
	binaryPath string
	crontabCmd string // "busybox" or "crontab"
}

// NewCrontabManager creates a new crontab manager
func NewCrontabManager(binaryPath string) *CrontabManager {
	crontabCmd := detectCrontabCommand()

	// Ensure crontab directories exist (busybox needs this)
	if crontabCmd == "busybox" {
		os.MkdirAll("/var/spool/cron/crontabs", 0755)
	}

	return &CrontabManager{
		binaryPath: binaryPath,
		crontabCmd: crontabCmd,
	}
}

// detectCrontabCommand returns the crontab command to use
func detectCrontabCommand() string {
	// Try busybox first (distroless environment)
	if _, err := exec.LookPath("busybox"); err == nil {
		return "busybox"
	}
	// Fall back to standard crontab
	return "crontab"
}

// IsInstalled checks if mycart crontab entries exist
func (m *CrontabManager) IsInstalled(ctx context.Context) (bool, error) {
	current, err := m.getCurrentCrontab(ctx)
	if err != nil {
		// No crontab is not an error, just means not installed
		if strings.Contains(err.Error(), "no crontab") {
			return false, nil
		}
		return false, fmt.Errorf("get current crontab: %w", err)
	}

	return strings.Contains(current, crontabMarkerStart), nil
}

// Install adds mycart crontab entries for enabled jobs
func (m *CrontabManager) Install(ctx context.Context, jobs []models.CrontabJob) error {
	current, err := m.getCurrentCrontab(ctx)
	if err != nil && !strings.Contains(err.Error(), "no crontab") {
		return fmt.Errorf("get current crontab: %w", err)
	}

	// Remove existing mycart entries
	cleaned := m.removeMycartEntries(current)

	// Generate new entries
	entries := m.generateEntries(jobs)
	if len(entries) == 0 {
		// No enabled jobs, just remove existing entries
		return m.writeCrontab(ctx, cleaned)
	}

	// Add mycart entries
	var buf bytes.Buffer
	buf.WriteString(cleaned)
	if len(cleaned) > 0 && !strings.HasSuffix(cleaned, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString(crontabMarkerStart + "\n")
	buf.WriteString(strings.Join(entries, "\n"))
	buf.WriteString("\n" + crontabMarkerEnd + "\n")

	return m.writeCrontab(ctx, buf.String())
}

// Uninstall removes mycart crontab entries
func (m *CrontabManager) Uninstall(ctx context.Context) error {
	current, err := m.getCurrentCrontab(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "no crontab") {
			return nil // Already removed
		}
		return fmt.Errorf("get current crontab: %w", err)
	}

	cleaned := m.removeMycartEntries(current)
	return m.writeCrontab(ctx, cleaned)
}

// getCurrentCrontab reads current crontab
func (m *CrontabManager) getCurrentCrontab(ctx context.Context) (string, error) {
	var cmd *exec.Cmd
	if m.crontabCmd == "busybox" {
		cmd = exec.CommandContext(ctx, "busybox", "crontab", "-l")
	} else {
		cmd = exec.CommandContext(ctx, "crontab", "-l")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("crontab -l failed: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// writeCrontab writes new crontab content
func (m *CrontabManager) writeCrontab(ctx context.Context, content string) error {
	var cmd *exec.Cmd
	if m.crontabCmd == "busybox" {
		cmd = exec.CommandContext(ctx, "busybox", "crontab", "-")
	} else {
		cmd = exec.CommandContext(ctx, "crontab", "-")
	}

	cmd.Stdin = strings.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("crontab write failed: %w: %s", err, stderr.String())
	}

	return nil
}

// removeMycartEntries strips out mycart-managed entries
func (m *CrontabManager) removeMycartEntries(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false

	for _, line := range lines {
		if strings.TrimSpace(line) == crontabMarkerStart {
			skip = true
			continue
		}
		if strings.TrimSpace(line) == crontabMarkerEnd {
			skip = false
			continue
		}
		if !skip {
			result = append(result, line)
		}
	}

	// Clean trailing empty lines
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	if len(result) == 0 {
		return ""
	}

	return strings.Join(result, "\n") + "\n"
}

// generateEntries creates cron entries for enabled jobs
func (m *CrontabManager) generateEntries(jobs []models.CrontabJob) []string {
	var entries []string

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}

		cronExpr := m.intervalToCron(job.Interval)
		comment := fmt.Sprintf("# mycart job: %s", job.JobType)
		command := fmt.Sprintf("%s cron-run %s", m.binaryPath, job.JobType)

		entries = append(entries, comment)
		entries = append(entries, fmt.Sprintf("%s %s", cronExpr, command))
	}

	return entries
}

// intervalToCron converts interval string to cron expression
func (m *CrontabManager) intervalToCron(interval string) string {
	switch interval {
	case "5min":
		return "*/5 * * * *"
	case "15min":
		return "*/15 * * * *"
	case "1hr":
		return "0 * * * *"
	case "6hr":
		return "0 */6 * * *"
	case "daily":
		return "0 0 * * *"
	default:
		return "*/5 * * * *" // fallback
	}
}

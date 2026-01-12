package rotation

import (
	"sort"
	"time"

	"github.com/localrivet/datasaver/pkg/postgres"
)

type GFSRotator struct {
	policy *Policy
}

func NewGFSRotator(policy *Policy) *GFSRotator {
	return &GFSRotator{
		policy: policy,
	}
}

type BackupEntry struct {
	Metadata *postgres.BackupMetadata
	Types    []BackupType
}

func (g *GFSRotator) DetermineBackupsToDelete(backups []*postgres.BackupMetadata) []*postgres.BackupMetadata {
	if len(backups) == 0 {
		return nil
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	entries := make([]BackupEntry, len(backups))
	for i, b := range backups {
		entries[i] = BackupEntry{
			Metadata: b,
			Types:    ClassifyBackup(b.Timestamp),
		}
	}

	keep := make(map[string]bool)

	dailyCount := 0
	weeklyCount := 0
	monthlyCount := 0

	for _, entry := range entries {
		shouldKeep := false

		for _, t := range entry.Types {
			switch t {
			case BackupTypeMonthly:
				if monthlyCount < g.policy.KeepMonthly {
					monthlyCount++
					shouldKeep = true
				}
			case BackupTypeWeekly:
				if weeklyCount < g.policy.KeepWeekly {
					weeklyCount++
					shouldKeep = true
				}
			case BackupTypeDaily:
				if dailyCount < g.policy.KeepDaily {
					dailyCount++
					shouldKeep = true
				}
			}
		}

		if shouldKeep {
			keep[entry.Metadata.ID] = true
		}
	}

	now := time.Now()
	maxAge := time.Duration(g.policy.MaxAgeDays) * 24 * time.Hour

	var toDelete []*postgres.BackupMetadata
	for _, entry := range entries {
		if !keep[entry.Metadata.ID] {
			toDelete = append(toDelete, entry.Metadata)
			continue
		}

		if g.policy.MaxAgeDays > 0 {
			age := now.Sub(entry.Metadata.Timestamp)
			if age > maxAge {
				toDelete = append(toDelete, entry.Metadata)
			}
		}
	}

	return toDelete
}

func (g *GFSRotator) GetRetentionInfo(backupTime time.Time) (time.Time, string) {
	primaryType := GetPrimaryType(backupTime)
	keepUntil := g.policy.CalculateRetentionDate(backupTime, primaryType)
	return keepUntil, string(primaryType)
}

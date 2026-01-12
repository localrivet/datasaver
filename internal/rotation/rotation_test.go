package rotation

import (
	"testing"
	"time"

	"github.com/localrivet/datasaver/pkg/postgres"
)

func TestNewPolicy(t *testing.T) {
	policy := NewPolicy(7, 4, 12, 365)

	if policy.KeepDaily != 7 {
		t.Errorf("KeepDaily = %v, want 7", policy.KeepDaily)
	}
	if policy.KeepWeekly != 4 {
		t.Errorf("KeepWeekly = %v, want 4", policy.KeepWeekly)
	}
	if policy.KeepMonthly != 12 {
		t.Errorf("KeepMonthly = %v, want 12", policy.KeepMonthly)
	}
	if policy.MaxAgeDays != 365 {
		t.Errorf("MaxAgeDays = %v, want 365", policy.MaxAgeDays)
	}
}

func TestBackupType_Constants(t *testing.T) {
	if BackupTypeDaily != "daily" {
		t.Errorf("BackupTypeDaily = %v, want daily", BackupTypeDaily)
	}
	if BackupTypeWeekly != "weekly" {
		t.Errorf("BackupTypeWeekly = %v, want weekly", BackupTypeWeekly)
	}
	if BackupTypeMonthly != "monthly" {
		t.Errorf("BackupTypeMonthly = %v, want monthly", BackupTypeMonthly)
	}
}

func TestClassifyBackup(t *testing.T) {
	tests := []struct {
		name       string
		time       time.Time
		wantDaily  bool
		wantWeekly bool
		wantMonthly bool
	}{
		{
			name:       "regular weekday",
			time:       time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), // Monday
			wantDaily:  true,
			wantWeekly: false,
			wantMonthly: false,
		},
		{
			name:       "sunday (weekly)",
			time:       time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC), // Sunday
			wantDaily:  true,
			wantWeekly: true,
			wantMonthly: false,
		},
		{
			name:       "first of month (monthly)",
			time:       time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC), // Thursday, Feb 1
			wantDaily:  true,
			wantWeekly: false,
			wantMonthly: true,
		},
		{
			name:       "first of month on sunday (weekly + monthly)",
			time:       time.Date(2024, 9, 1, 12, 0, 0, 0, time.UTC), // Sunday, Sep 1
			wantDaily:  true,
			wantWeekly: true,
			wantMonthly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types := ClassifyBackup(tt.time)

			hasDaily := containsType(types, BackupTypeDaily)
			hasWeekly := containsType(types, BackupTypeWeekly)
			hasMonthly := containsType(types, BackupTypeMonthly)

			if hasDaily != tt.wantDaily {
				t.Errorf("daily = %v, want %v", hasDaily, tt.wantDaily)
			}
			if hasWeekly != tt.wantWeekly {
				t.Errorf("weekly = %v, want %v", hasWeekly, tt.wantWeekly)
			}
			if hasMonthly != tt.wantMonthly {
				t.Errorf("monthly = %v, want %v", hasMonthly, tt.wantMonthly)
			}
		})
	}
}

func TestGetPrimaryType(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		want BackupType
	}{
		{
			name: "regular weekday is daily",
			time: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), // Monday
			want: BackupTypeDaily,
		},
		{
			name: "sunday is weekly",
			time: time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC), // Sunday
			want: BackupTypeWeekly,
		},
		{
			name: "first of month is monthly",
			time: time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC), // Feb 1
			want: BackupTypeMonthly,
		},
		{
			name: "first of month on sunday is monthly (monthly takes precedence)",
			time: time.Date(2024, 9, 1, 12, 0, 0, 0, time.UTC), // Sunday, Sep 1
			want: BackupTypeMonthly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPrimaryType(tt.time)
			if got != tt.want {
				t.Errorf("GetPrimaryType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicy_CalculateRetentionDate(t *testing.T) {
	policy := NewPolicy(7, 4, 12, 365)
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		backupType BackupType
		wantDays   int
	}{
		{
			name:       "daily retention",
			backupType: BackupTypeDaily,
			wantDays:   7,
		},
		{
			name:       "weekly retention",
			backupType: BackupTypeWeekly,
			wantDays:   28, // 4 weeks * 7 days
		},
		{
			name:       "monthly retention",
			backupType: BackupTypeMonthly,
			wantDays:   360, // 12 months * 30 days
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.CalculateRetentionDate(baseTime, tt.backupType)
			expectedDate := baseTime.AddDate(0, 0, tt.wantDays)

			if !result.Equal(expectedDate) {
				t.Errorf("CalculateRetentionDate() = %v, want %v", result, expectedDate)
			}
		})
	}
}

func TestPolicy_CalculateRetentionDate_MaxAgeLimit(t *testing.T) {
	// Policy with short max age
	policy := NewPolicy(7, 4, 12, 30) // Max 30 days
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Monthly would be 360 days, but max age is 30
	result := policy.CalculateRetentionDate(baseTime, BackupTypeMonthly)
	expectedDate := baseTime.AddDate(0, 0, 30)

	if !result.Equal(expectedDate) {
		t.Errorf("CalculateRetentionDate() with max age = %v, want %v", result, expectedDate)
	}
}

func TestPolicy_CalculateRetentionDate_NoMaxAge(t *testing.T) {
	// Policy with no max age limit
	policy := NewPolicy(7, 4, 12, 0)
	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	result := policy.CalculateRetentionDate(baseTime, BackupTypeMonthly)
	expectedDate := baseTime.AddDate(0, 0, 360) // Full 12 months

	if !result.Equal(expectedDate) {
		t.Errorf("CalculateRetentionDate() without max age = %v, want %v", result, expectedDate)
	}
}

func TestNewGFSRotator(t *testing.T) {
	policy := NewPolicy(7, 4, 12, 365)
	rotator := NewGFSRotator(policy)

	if rotator == nil {
		t.Fatal("NewGFSRotator() returned nil")
	}
	if rotator.policy != policy {
		t.Error("NewGFSRotator() policy mismatch")
	}
}

func TestGFSRotator_DetermineBackupsToDelete_Empty(t *testing.T) {
	policy := NewPolicy(7, 4, 12, 365)
	rotator := NewGFSRotator(policy)

	result := rotator.DetermineBackupsToDelete(nil)
	if result != nil {
		t.Errorf("DetermineBackupsToDelete(nil) = %v, want nil", result)
	}

	result = rotator.DetermineBackupsToDelete([]*postgres.BackupMetadata{})
	if result != nil {
		t.Errorf("DetermineBackupsToDelete([]) = %v, want nil", result)
	}
}

func TestGFSRotator_DetermineBackupsToDelete_KeepRecent(t *testing.T) {
	policy := NewPolicy(3, 2, 1, 0) // Keep 3 daily, 2 weekly, 1 monthly
	rotator := NewGFSRotator(policy)

	now := time.Now()
	backups := []*postgres.BackupMetadata{
		{ID: "backup-1", Timestamp: now.AddDate(0, 0, -1)},
		{ID: "backup-2", Timestamp: now.AddDate(0, 0, -2)},
		{ID: "backup-3", Timestamp: now.AddDate(0, 0, -3)},
		{ID: "backup-4", Timestamp: now.AddDate(0, 0, -4)},
		{ID: "backup-5", Timestamp: now.AddDate(0, 0, -5)},
	}

	toDelete := rotator.DetermineBackupsToDelete(backups)

	// Should delete backups beyond retention policy
	if len(toDelete) != 2 {
		t.Errorf("DetermineBackupsToDelete() deleted %d, want 2", len(toDelete))
	}
}

func TestGFSRotator_DetermineBackupsToDelete_KeepWeekly(t *testing.T) {
	policy := NewPolicy(0, 2, 0, 0) // Keep only 2 weekly
	rotator := NewGFSRotator(policy)

	// Create backups on Sundays (weekly backups)
	backups := []*postgres.BackupMetadata{
		{ID: "backup-1", Timestamp: time.Date(2024, 1, 7, 12, 0, 0, 0, time.UTC)},  // Sunday
		{ID: "backup-2", Timestamp: time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC)}, // Sunday
		{ID: "backup-3", Timestamp: time.Date(2024, 1, 21, 12, 0, 0, 0, time.UTC)}, // Sunday
	}

	toDelete := rotator.DetermineBackupsToDelete(backups)

	// Should keep 2 most recent Sundays, delete 1
	if len(toDelete) != 1 {
		t.Errorf("DetermineBackupsToDelete() deleted %d, want 1", len(toDelete))
	}
}

func TestGFSRotator_DetermineBackupsToDelete_KeepMonthly(t *testing.T) {
	policy := NewPolicy(0, 0, 2, 0) // Keep only 2 monthly
	rotator := NewGFSRotator(policy)

	// Create backups on first of month (monthly backups)
	backups := []*postgres.BackupMetadata{
		{ID: "backup-1", Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
		{ID: "backup-2", Timestamp: time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)},
		{ID: "backup-3", Timestamp: time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)},
	}

	toDelete := rotator.DetermineBackupsToDelete(backups)

	// Should keep 2 most recent, delete 1
	if len(toDelete) != 1 {
		t.Errorf("DetermineBackupsToDelete() deleted %d, want 1", len(toDelete))
	}
}

func TestGFSRotator_DetermineBackupsToDelete_MaxAge(t *testing.T) {
	policy := NewPolicy(100, 100, 100, 7) // Keep lots, but max 7 days
	rotator := NewGFSRotator(policy)

	now := time.Now()
	backups := []*postgres.BackupMetadata{
		{ID: "backup-recent", Timestamp: now.AddDate(0, 0, -1)},
		{ID: "backup-old", Timestamp: now.AddDate(0, 0, -30)}, // Older than 7 days
	}

	toDelete := rotator.DetermineBackupsToDelete(backups)

	// Old backup should be deleted due to max age
	if len(toDelete) != 1 {
		t.Errorf("DetermineBackupsToDelete() deleted %d, want 1", len(toDelete))
	}
	if len(toDelete) > 0 && toDelete[0].ID != "backup-old" {
		t.Errorf("DetermineBackupsToDelete() deleted wrong backup")
	}
}

func TestGFSRotator_GetRetentionInfo(t *testing.T) {
	policy := NewPolicy(7, 4, 12, 365)
	rotator := NewGFSRotator(policy)

	tests := []struct {
		name         string
		time         time.Time
		wantType     string
	}{
		{
			name:         "regular day is daily",
			time:         time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), // Monday
			wantType:     "daily",
		},
		{
			name:         "sunday is weekly",
			time:         time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC), // Sunday
			wantType:     "weekly",
		},
		{
			name:         "first of month is monthly",
			time:         time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC),
			wantType:     "monthly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keepUntil, backupType := rotator.GetRetentionInfo(tt.time)

			if backupType != tt.wantType {
				t.Errorf("GetRetentionInfo() type = %v, want %v", backupType, tt.wantType)
			}

			if keepUntil.Before(tt.time) {
				t.Errorf("GetRetentionInfo() keepUntil is before backup time")
			}
		})
	}
}

func TestBackupEntry(t *testing.T) {
	metadata := &postgres.BackupMetadata{
		ID:        "test-backup",
		Timestamp: time.Now(),
	}

	entry := BackupEntry{
		Metadata: metadata,
		Types:    []BackupType{BackupTypeDaily, BackupTypeWeekly},
	}

	if entry.Metadata != metadata {
		t.Error("BackupEntry Metadata mismatch")
	}
	if len(entry.Types) != 2 {
		t.Errorf("BackupEntry Types length = %d, want 2", len(entry.Types))
	}
}

func TestGFSRotator_DetermineBackupsToDelete_MixedTypes(t *testing.T) {
	policy := NewPolicy(2, 1, 1, 0) // 2 daily, 1 weekly, 1 monthly
	rotator := NewGFSRotator(policy)

	// Mix of different backup types
	backups := []*postgres.BackupMetadata{
		// Recent dailies
		{ID: "daily-1", Timestamp: time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)}, // Tuesday
		{ID: "daily-2", Timestamp: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)}, // Monday
		{ID: "daily-3", Timestamp: time.Date(2024, 1, 12, 12, 0, 0, 0, time.UTC)}, // Friday (should be deleted)
		// Weekly
		{ID: "weekly-1", Timestamp: time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC)}, // Sunday
		{ID: "weekly-2", Timestamp: time.Date(2024, 1, 7, 12, 0, 0, 0, time.UTC)},  // Sunday (should be deleted)
		// Monthly
		{ID: "monthly-1", Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)}, // Jan 1 (monthly)
	}

	toDelete := rotator.DetermineBackupsToDelete(backups)

	// Should delete: daily-3 and weekly-2
	deletedIDs := make(map[string]bool)
	for _, b := range toDelete {
		deletedIDs[b.ID] = true
	}

	if len(toDelete) != 2 {
		t.Errorf("DetermineBackupsToDelete() deleted %d, want 2", len(toDelete))
		for _, b := range toDelete {
			t.Logf("  deleted: %s", b.ID)
		}
	}
}

// Helper function
func containsType(types []BackupType, target BackupType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

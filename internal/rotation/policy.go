package rotation

import (
	"time"
)

type Policy struct {
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	MaxAgeDays  int
}

func NewPolicy(daily, weekly, monthly, maxAgeDays int) *Policy {
	return &Policy{
		KeepDaily:   daily,
		KeepWeekly:  weekly,
		KeepMonthly: monthly,
		MaxAgeDays:  maxAgeDays,
	}
}

type BackupType string

const (
	BackupTypeDaily   BackupType = "daily"
	BackupTypeWeekly  BackupType = "weekly"
	BackupTypeMonthly BackupType = "monthly"
)

func ClassifyBackup(t time.Time) []BackupType {
	types := []BackupType{BackupTypeDaily}

	if t.Weekday() == time.Sunday {
		types = append(types, BackupTypeWeekly)
	}

	if t.Day() == 1 {
		types = append(types, BackupTypeMonthly)
	}

	return types
}

func GetPrimaryType(t time.Time) BackupType {
	if t.Day() == 1 {
		return BackupTypeMonthly
	}
	if t.Weekday() == time.Sunday {
		return BackupTypeWeekly
	}
	return BackupTypeDaily
}

func (p *Policy) CalculateRetentionDate(backupTime time.Time, backupType BackupType) time.Time {
	var retentionDays int

	switch backupType {
	case BackupTypeMonthly:
		retentionDays = p.KeepMonthly * 30
	case BackupTypeWeekly:
		retentionDays = p.KeepWeekly * 7
	case BackupTypeDaily:
		retentionDays = p.KeepDaily
	}

	if p.MaxAgeDays > 0 && p.MaxAgeDays < retentionDays {
		retentionDays = p.MaxAgeDays
	}

	return backupTime.AddDate(0, 0, retentionDays)
}

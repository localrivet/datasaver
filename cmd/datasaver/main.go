package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/almatuck/datasaver/internal/backup"
	"github.com/almatuck/datasaver/internal/config"
	"github.com/almatuck/datasaver/internal/mcp"
	"github.com/almatuck/datasaver/internal/metrics"
	"github.com/almatuck/datasaver/internal/notify"
	"github.com/almatuck/datasaver/internal/restore"
	"github.com/almatuck/datasaver/internal/storage"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	cfgFile   string
	logger    *slog.Logger
	cfg       *config.Config
	store     storage.Backend
	notifier  *notify.Notifier
)

func main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	rootCmd := &cobra.Command{
		Use:     "datasaver",
		Short:   "Database backup utility",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "help" || cmd.Name() == "version" {
				return nil
			}

			var err error
			cfg, err = config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			factory := storage.NewFactory()
			var s3Cfg *storage.S3Config
			if cfg.Storage.Backend == "s3" {
				s3Cfg = &storage.S3Config{
					Bucket:    cfg.Storage.S3.Bucket,
					Endpoint:  cfg.Storage.S3.Endpoint,
					Region:    cfg.Storage.S3.Region,
					AccessKey: cfg.Storage.S3.AccessKey,
					SecretKey: cfg.Storage.S3.SecretKey,
					UseSSL:    cfg.Storage.S3.UseSSL,
				}
			}

			store, err = factory.Create(cfg.Storage.Backend, cfg.Storage.Path, s3Cfg)
			if err != nil {
				return fmt.Errorf("failed to create storage backend: %w", err)
			}

			notifier = notify.NewNotifier(cfg.Monitoring.WebhookURL, logger)

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	rootCmd.AddCommand(daemonCmd())
	rootCmd.AddCommand(backupCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(cleanupCmd())
	rootCmd.AddCommand(healthCmd())
	rootCmd.AddCommand(verifyCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run as scheduled backup daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			m := metrics.New("datasaver")

			engine := backup.NewEngine(cfg, store, notifier, logger)
			scheduler := backup.NewScheduler(engine, cfg.Schedule, logger)

			if err := scheduler.Start(ctx); err != nil {
				return fmt.Errorf("failed to start scheduler: %w", err)
			}

			mux := http.NewServeMux()
			mux.Handle("/metrics", metrics.Handler())
			mux.HandleFunc("/health", healthHandler(scheduler))

			// Add MCP endpoint if API key is configured
			mcpHandler := mcp.NewHandler(cfg, store, notifier, logger)
			if mcpHandler.Enabled() {
				mux.Handle("/mcp", mcpHandler)
				logger.Info("MCP endpoint enabled", "path", "/mcp")
			}

			healthServer := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.Monitoring.HealthPort),
				Handler: mux,
			}

			go func() {
				logger.Info("health server starting", "port", cfg.Monitoring.HealthPort)
				if err := healthServer.ListenAndServe(); err != http.ErrServerClosed {
					logger.Error("health server error", "error", err)
				}
			}()

			metricsServer := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.Monitoring.MetricsPort),
				Handler: metrics.Handler(),
			}

			go func() {
				logger.Info("metrics server starting", "port", cfg.Monitoring.MetricsPort)
				if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
					logger.Error("metrics server error", "error", err)
				}
			}()

			go alertMonitor(ctx, scheduler, cfg, m)

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			logger.Info("shutting down")

			scheduler.Stop()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			healthServer.Shutdown(shutdownCtx)
			metricsServer.Shutdown(shutdownCtx)

			return nil
		},
	}
}

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Perform immediate backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			engine := backup.NewEngine(cfg, store, notifier, logger)

			result, err := engine.Run(ctx)
			if err != nil {
				return err
			}

			fmt.Printf("Backup completed successfully\n")
			fmt.Printf("  ID: %s\n", result.ID)
			fmt.Printf("  Size: %s\n", formatBytes(result.Size))
			fmt.Printf("  Compressed: %s\n", formatBytes(result.CompressedSize))
			fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Millisecond))

			return nil
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			engine := backup.NewEngine(cfg, store, notifier, logger)

			backups, err := engine.ListBackups(ctx)
			if err != nil {
				return err
			}

			if len(backups) == 0 {
				fmt.Println("No backups found")
				return nil
			}

			sort.Slice(backups, func(i, j int) bool {
				return backups[i].Timestamp.After(backups[j].Timestamp)
			})

			fmt.Printf("%-26s %-20s %-12s %-8s\n", "ID", "DATE", "SIZE", "TYPE")
			for _, b := range backups {
				fmt.Printf("%-26s %-20s %-12s %-8s\n",
					b.ID,
					b.Timestamp.Format("2006-01-02 15:04"),
					formatBytes(b.Backup.CompressedSize),
					b.Type,
				)
			}

			return nil
		},
	}
}

func restoreCmd() *cobra.Command {
	var targetDB string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "restore <backup-id>",
		Short: "Restore from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			restoreEngine := restore.NewEngine(cfg, store, logger)

			result, err := restoreEngine.Restore(ctx, restore.RestoreOptions{
				BackupID: args[0],
				TargetDB: targetDB,
				DryRun:   dryRun,
			})
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Println("Dry run completed - no changes made")
			} else {
				fmt.Printf("Restore completed successfully\n")
				fmt.Printf("  Backup: %s\n", result.BackupID)
				fmt.Printf("  Target database: %s\n", result.TargetDB)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&targetDB, "target-db", "", "restore to different database")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "test restore without applying")

	return cmd
}

func cleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old backups manually",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			engine := backup.NewEngine(cfg, store, notifier, logger)

			count, err := engine.Cleanup(ctx)
			if err != nil {
				return err
			}

			fmt.Printf("Cleanup completed: %d backups deleted\n", count)
			return nil
		},
	}
}

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check backup system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			engine := backup.NewEngine(cfg, store, notifier, logger)

			backups, err := engine.ListBackups(ctx)
			if err != nil {
				return err
			}

			var totalSize int64
			var lastBackup time.Time

			for _, b := range backups {
				totalSize += b.Backup.CompressedSize
				if b.Timestamp.After(lastBackup) {
					lastBackup = b.Timestamp
				}
			}

			status := "healthy"
			if len(backups) == 0 {
				status = "warning: no backups found"
			} else if time.Since(lastBackup) > cfg.AlertDuration() {
				status = "warning: backup overdue"
			}

			fmt.Printf("Status: %s\n", status)
			if !lastBackup.IsZero() {
				fmt.Printf("Last backup: %s\n", lastBackup.Format("2006-01-02 15:04:05"))
			}
			fmt.Printf("Total backups: %d\n", len(backups))
			fmt.Printf("Storage used: %s\n", formatBytes(totalSize))

			return nil
		},
	}
}

func verifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <backup-id>",
		Short: "Validate backup integrity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			engine := backup.NewEngine(cfg, store, notifier, logger)
			validator := backup.NewValidator(store, logger)

			meta, err := engine.GetBackup(ctx, args[0])
			if err != nil {
				return err
			}

			result, err := validator.Validate(ctx, meta)
			if err != nil {
				return err
			}

			if result.Valid {
				fmt.Printf("Backup %s is valid\n", args[0])
				fmt.Printf("  File exists: %v\n", result.FileExists)
				fmt.Printf("  Size match: %v\n", result.SizeMatch)
				fmt.Printf("  Checksum OK: %v\n", result.ChecksumOK)
			} else {
				fmt.Printf("Backup %s is INVALID\n", args[0])
				for _, e := range result.Errors {
					fmt.Printf("  - %s\n", e)
				}
				return fmt.Errorf("backup validation failed")
			}

			return nil
		},
	}
}

func healthHandler(scheduler *backup.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		engine := scheduler.Engine()

		status := "healthy"
		lastRun := engine.LastRun()
		lastErr := engine.LastError()
		nextRun := scheduler.NextRun()

		if lastErr != nil {
			status = "unhealthy"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		fmt.Fprintf(w, "status: %s\n", status)
		if !lastRun.IsZero() {
			fmt.Fprintf(w, "last_backup: %s\n", lastRun.Format(time.RFC3339))
		}
		if lastErr != nil {
			fmt.Fprintf(w, "last_error: %s\n", lastErr.Error())
		}
		if !nextRun.IsZero() {
			fmt.Fprintf(w, "next_backup: %s\n", nextRun.Format(time.RFC3339))
		}
	}
}

func alertMonitor(ctx context.Context, scheduler *backup.Scheduler, cfg *config.Config, m *metrics.Metrics) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			engine := scheduler.Engine()
			lastRun := engine.LastRun()

			backups, _ := engine.ListBackups(ctx)
			var totalSize int64
			for _, b := range backups {
				totalSize += b.Backup.CompressedSize
			}
			m.SetStorageUsed(totalSize)

			if !lastRun.IsZero() && time.Since(lastRun) > cfg.AlertDuration() {
				if notifier != nil {
					notifier.NotifyAlert(fmt.Sprintf(
						"No backup in %d hours. Last backup: %s",
						cfg.Monitoring.AlertAfterHours,
						lastRun.Format(time.RFC3339),
					))
				}
			}
		}
	}
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

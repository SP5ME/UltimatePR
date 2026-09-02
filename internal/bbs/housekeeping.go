package bbs

import (
	"context"
	"log/slog"
	"time"
)

// RunHousekeeping removes expired mail at startup and once per day.
func RunHousekeeping(ctx context.Context, store *Store, bulletinDays, personalDays int, log *slog.Logger) {
	run := func() {
		bulletins, personal, err := store.RemoveExpired(time.Now(), bulletinDays, personalDays)
		if err != nil {
			if log != nil {
				log.Warn("BBS housekeeping failed", "error", err)
			}
			return
		}
		if log != nil && (bulletins > 0 || personal > 0) {
			log.Info("BBS housekeeping removed expired messages", "bulletins", bulletins, "personal", personal)
		}
	}
	run()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			run()
		case <-ctx.Done():
			return
		}
	}
}

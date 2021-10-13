package store

import (
	"fmt"
	"time"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"go.uber.org/zap"
)

const (
	gcInterval = time.Second * 60
)

func (s *ProfileStorage) doGCLoop() {
	ticker := time.NewTicker(gcInterval)
	for {
		select {
		case <-ticker.C:
			s.runGC()
		}
	}
}

func (s *ProfileStorage) runGC() {
	start := time.Now()
	targets := s.getAllTargets()
	safePointTs := s.getLastSafePointTs()
	for _, target := range targets {
		info := s.getTargetInfoFromCache(target)
		if info == nil {
			continue
		}
		sql := fmt.Sprintf("DELETE FROM %v WHERE ts <= ?", s.getProfileTableName(info))
		err := s.db.Exec(sql, safePointTs)
		if err != nil {
			logutil.BgLogger().Error("gc delete target data failed", zap.Error(err))
		}
	}
	logutil.BgLogger().Info("gc finished",
		zap.Int("total-targets", len(targets)),
		zap.Int64("safepoint", safePointTs),
		zap.Duration("cost", time.Since(start)))
}

func (s *ProfileStorage) getLastSafePointTs() int64 {
	cfg := config.GetGlobalConfig()
	safePoint := time.Now().Add(time.Duration(-cfg.ContinueProfiling.DataRetentionSeconds) * time.Second)
	return util.GetTimeStamp(safePoint)
}

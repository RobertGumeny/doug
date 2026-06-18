package cmd

import (
	"fmt"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/log"
	runstats "github.com/robertgumeny/doug/internal/stats"
)

func persistRunStats(logger log.Logger, logsDir, epicID string, phase agent.RunPhase, taskID string, attempt int, resp agent.RunResponse) {
	if epicID == "" && phase != agent.RunPhaseRuntime {
		epicID = string(phase)
	}
	statsRecord := runstats.FromRunResponse(phase, taskID, attempt, time.Now(), resp)
	if statsPath, statsErr := runstats.WriteRunStats(logsDir, epicID, statsRecord); statsErr != nil {
		logger.Warning(fmt.Sprintf("write agent run stats: %v", statsErr))
	} else {
		logger.Info(fmt.Sprintf("wrote run stats: %s", statsPath))
	}
}

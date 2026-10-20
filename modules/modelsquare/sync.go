package modelsquare

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	// remoteRefreshTTL is how long a job's remote state is trusted before it is
	// polled again. Jobs are refreshed lazily when the console reads them, so a
	// burst of list requests cannot hammer the training service.
	remoteRefreshTTL = 5 * time.Second
	// remoteRefreshBatchLimit bounds how many jobs one list request refreshes.
	remoteRefreshBatchLimit = 20
	// remoteRefreshTimeout bounds the whole refresh pass of one request.
	remoteRefreshTimeout = 15 * time.Second
)

// remoteRefreshTimes records when each job was last polled.
var remoteRefreshTimes sync.Map

// jobNeedsRemoteRefresh reports whether the training service can still tell us
// something new about this job.
func jobNeedsRemoteRefresh(job FinetuneJob) bool {
	return job.ServiceJobId != "" && !isTerminalJobStatus(job.Status) && job.Status != JobStatusDraft
}

// claimRemoteRefresh reserves a refresh slot for one job. It reports false while
// the previous refresh is still considered fresh.
func claimRemoteRefresh(jobId int) bool {
	now := time.Now()
	if lastSeen, ok := remoteRefreshTimes.Load(jobId); ok {
		if lastRefresh, isTime := lastSeen.(time.Time); isTime && now.Sub(lastRefresh) < remoteRefreshTTL {
			return false
		}
	}
	remoteRefreshTimes.Store(jobId, now)
	return true
}

// applyRemoteJobState mirrors the service's view of a job onto the stored row.
func applyRemoteJobState(ctx context.Context, client *trainingClient, job *FinetuneJob) {
	remote, err := client.getJob(ctx, job.ServiceJobId)
	if err != nil {
		// An unreachable service must never be recorded as a failed job: the
		// stored state stays authoritative until the service answers again.
		common.SysLog(common.LogText("failed to refresh training job %d: %s", job.Id, err.Error()))
		return
	}

	status := trainingServiceStatus(remote.Status)
	updates := map[string]any{"progress": remote.Progress}
	job.Progress = remote.Progress
	if status != job.Status {
		updates["status"] = status
		job.Status = status
	}
	if remote.Error != "" && remote.Error != job.ErrorMsg {
		updates["error_msg"] = remote.Error
		job.ErrorMsg = remote.Error
	}
	if remote.Artifact != nil && remote.Artifact.OutputDir != job.OutputDir {
		updates["output_dir"] = remote.Artifact.OutputDir
		job.OutputDir = remote.Artifact.OutputDir
	}
	if remote.Usage != nil {
		if encoded, marshalErr := common.Marshal(remote.Usage); marshalErr == nil {
			updates["usage"] = string(encoded)
			job.Usage = string(encoded)
		}
	}
	if isTerminalJobStatus(status) {
		finishedAt := common.GetTimestamp()
		updates["finished_at"] = finishedAt
		job.FinishedAt = finishedAt
		// The trained artifact is exposed under the job name; the console uses
		// ResultModel as the model label when registering the artifact later.
		if status == JobStatusSucceeded && job.ResultModel == "" {
			updates["result_model"] = job.Name
			job.ResultModel = job.Name
		}
		// Stop tracking a finished job so the map cannot grow without bound.
		remoteRefreshTimes.Delete(job.Id)
	}

	if err := model.DB.Model(job).Updates(updates).Error; err != nil {
		common.SysLog(common.LogText("failed to store the refreshed training job %d: %s", job.Id, err.Error()))
	}
}

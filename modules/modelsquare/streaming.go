package modelsquare

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// Log streaming is a transparent proxy: the training service already speaks
// server-sent events with `data: {...}` frames and a final `data: [DONE]`, so
// the gateway only has to forward bytes and flush them as they arrive.
//
// The browser cannot use the native EventSource API here because console
// authentication travels in a header, so the frontend reads the stream with
// fetch() + getReader(). Reconnecting is a new request with the last `offset`.

// maxLogFrameBytes bounds a single forwarded frame so one giant log line cannot
// balloon a response.
const maxLogFrameBytes = 256 << 10

// JobLogs streams the training log of one job.
func JobLogs(c *gin.Context) {
	job, ok := loadOwnedJob(c)
	if !ok {
		return
	}
	if job.ServiceJobId == "" {
		common.ApiErrorMsg(c, "this job has not been submitted yet")
		return
	}
	offset, ok := parseLogOffset(c)
	if !ok {
		return
	}
	streamTrainingLog(c, "/v1/jobs/"+job.ServiceJobId+"/logs", offset)
}

// DatasetPrepareLogs streams the DataFlow log of one preparation run.
func DatasetPrepareLogs(c *gin.Context) {
	dataset, ok := loadOwnedDataset(c)
	if !ok {
		return
	}
	if dataset.PrepareRunId == "" {
		common.ApiErrorMsg(c, "this dataset has no preparation run")
		return
	}
	offset, ok := parseLogOffset(c)
	if !ok {
		return
	}
	path := "/v1/datasets/" + dataset.RemoteId + "/prepare/" + dataset.PrepareRunId + "/logs"
	streamTrainingLog(c, path, offset)
}

func parseLogOffset(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("offset"))
	if raw == "" {
		return 0, true
	}
	offset, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || offset < 0 {
		common.ApiErrorMsg(c, "invalid log offset")
		return 0, false
	}
	return offset, true
}

// streamTrainingLog opens the upstream stream and forwards it frame by frame.
func streamTrainingLog(c *gin.Context, path string, offset int64) {
	cfg, err := loadTrainingConfig()
	if err != nil {
		respondTrainingError(c, err)
		return
	}

	// The upstream request follows the client connection: when the console tab
	// closes, the service stops streaming too.
	ctx := c.Request.Context()
	response, err := newTrainingClient(cfg).openLogStream(ctx, path, offset)
	if err != nil {
		respondTrainingError(c, err)
		return
	}
	defer response.Body.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// Disable proxy buffering so frames reach the browser immediately.
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	reader := bufio.NewReaderSize(response.Body, 32<<10)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			if _, writeErr := c.Writer.WriteString(truncateLogLine(line)); writeErr != nil {
				return
			}
			c.Writer.Flush()
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, context.Canceled) {
				writeLogStreamError(c, readErr)
			}
			return
		}
	}
}

// truncateLogLine caps one forwarded line, keeping the trailing newline so the
// SSE framing on the client side stays intact.
func truncateLogLine(line string) string {
	if len(line) <= maxLogFrameBytes {
		return line
	}
	return line[:maxLogFrameBytes] + "\n"
}

// writeLogStreamError terminates the stream cleanly with an error frame instead
// of leaving the client waiting for a connection that will never resume.
func writeLogStreamError(c *gin.Context, err error) {
	payload, marshalErr := common.Marshal(gin.H{"error": "the training log stream ended unexpectedly"})
	if marshalErr != nil {
		return
	}
	common.SysLog(common.LogText("training log stream failed: %s", err.Error()))
	_, _ = c.Writer.WriteString("data: " + string(payload) + "\n\n")
	_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

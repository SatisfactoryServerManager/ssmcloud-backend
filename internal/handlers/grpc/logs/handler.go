package logs

import (
	"context"
	"io"
	"sync"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
)

type Handler struct {
	pb.UnimplementedAgentLogServiceServer
}

var (
	activeStreams = make(map[string]context.CancelFunc)
	streamsMu     sync.Mutex
)

func (h *Handler) StreamLog(stream pb.AgentLogService_StreamLogServer) error {

	// derive new context for this stream
	ctx, cancel := context.WithCancel(stream.Context())

	// get API key
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		cancel()
		return err
	}

	key := *apiKey

	logger.GetDebugLogger().Printf("log stream opened (key prefix %s)", keyPrefix(key))

	// register the stream
	streamsMu.Lock()
	activeStreams[key] = cancel
	streamsMu.Unlock()

	defer func() {
		// remove on exit
		streamsMu.Lock()
		delete(activeStreams, key)
		streamsMu.Unlock()
		cancel()
		logger.GetDebugLogger().Printf("log stream closed (key prefix %s)", keyPrefix(key))
	}()

	msgChan := make(chan *pb.AgentLogLineRequest)
	errChan := make(chan error)

	// recv goroutine
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- msg
		}
	}()

	// main loop
	for {
		select {
		case <-ctx.Done():
			return stream.SendAndClose(&pbModels.SSMEmpty{})

		case err := <-errChan:
			if err == io.EOF {
				return stream.SendAndClose(&pbModels.SSMEmpty{})
			}
			return err

		case msg := <-msgChan:
			logger.GetDebugLogger().Printf("log line recv (key prefix %s) type=%s inital=%t len=%d", keyPrefix(key), msg.Type, msg.Inital, len(msg.Line))
			if err := agent.AddAgentLogLine(*apiKey, msg.Type, msg.Line, msg.Inital); err != nil {
				logger.GetErrorLogger().Printf("AddAgentLogLine failed (type=%s): %v", msg.Type, err)
				return err
			}
		}
	}
}

// keyPrefix returns a short, non-sensitive prefix of an API key for logging.
func keyPrefix(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8]
}

func ShutdownAgentLogHandler() {
	streamsMu.Lock()
	defer streamsMu.Unlock()

	for key, cancel := range activeStreams {
		logger.GetDebugLogger().Println("Shutting down log stream:", key)
		cancel()
	}

	// clean map
	activeStreams = make(map[string]context.CancelFunc)
	logger.GetDebugLogger().Println("Shutdown gRPC log handler")
}

package state

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
)

type Handler struct {
	pb.UnimplementedAgentStateServiceServer
}

var (
	activeStreams = make(map[string]context.CancelFunc)
	streamsMu     sync.Mutex
)

func (s *Handler) UpdateAgentStateStream(stream pb.AgentStateService_UpdateAgentStateStreamServer) error {

	// derive new context for this stream
	ctx, cancel := context.WithCancel(stream.Context())

	// get API key
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		cancel()
		return err
	}

	key := *apiKey

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
	}()

	msgChan := make(chan *pb.AgentStateRequest)
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
			return stream.SendAndClose(&pb.Empty{})

		case err := <-errChan:
			if err == io.EOF {
				return stream.SendAndClose(&pb.Empty{})
			}
			return err

		case msg := <-msgChan:
			err := agent.UpdateAgentStatus(
				key, msg.Online, msg.Installed, msg.Running,
				float64(msg.Cpu), msg.Ram,
				msg.InstalledSFVersion, msg.LatestSFVersion,
			)
			if err != nil {
				return err
			}
		}
	}
}

func (s *Handler) UpdateAgentState(ctx context.Context, msg *pb.AgentStateRequest) (*pb.Empty, error) {
	apiKey, err := utils.GetAPIKeyFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := agent.UpdateAgentStatus(*apiKey, msg.Online, msg.Installed, msg.Running, float64(msg.Cpu), msg.Ram, msg.InstalledSFVersion, msg.LatestSFVersion); err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

func ShutdownAgentStateHandler() {
	streamsMu.Lock()
	defer streamsMu.Unlock()

	for key, cancel := range activeStreams {
		fmt.Println("Shutting down state stream:", key)
		cancel()
	}

	// clean map
	activeStreams = make(map[string]context.CancelFunc)
	logger.GetDebugLogger().Println("Shutdown gRPC state handler")
}

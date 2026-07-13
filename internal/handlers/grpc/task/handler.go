package task

import (
	"context"
	"sync"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/grpc/metadata"
)

type Handler struct {
	pb.UnimplementedAgentTaskServiceServer
}

// replicaID identifies this process in agent.connectedTo. It only needs to be
// unique per running replica, not stable across restarts.
var replicaID = bson.NewObjectID().Hex()

// shutdown releases every open task stream. grpcServer.GracefulStop() waits for
// in-flight RPCs to return, and a subscription never returns on its own, so
// without this the backend hangs until the shutdown timeout force-exits it.
var (
	shutdown     = make(chan struct{})
	shutdownOnce sync.Once
)

// ShutdownTaskHandler closes every subscription so GracefulStop can complete.
// The agents reconnect to another replica; their leases are untouched.
func ShutdownTaskHandler() {
	shutdownOnce.Do(func() { close(shutdown) })
}

// SubscribeTasks holds the stream open for the agent's lifetime, forwarding
// assignments the dispatcher claims for it.
//
// Status.Online is deliberately not touched here: it is owned by the state RPC
// pipeline, which fires integration events on transitions. This method only
// records which replica can reach the agent.
func (s *Handler) SubscribeTasks(in *pb.SubscribeTasksRequest, stream pb.AgentTaskService_SubscribeTasksServer) error {
	apiKey, err := utils.GetAPIKeyFromContext(stream.Context())
	if err != nil {
		return err
	}

	theAgent, err := agent.GetAgentByAPIKey(*apiKey)
	if err != nil {
		return err
	}

	// Minted here, not taken from the request: the agent reuses one session id for
	// its whole process, so a client-supplied id cannot distinguish an old stream
	// from the reconnect that replaced it.
	streamID := bson.NewObjectID().Hex()

	if err := setConnection(theAgent.ID, streamID); err != nil {
		return err
	}

	enqueueBootUpdate(theAgent, in.SessionId)

	ch, deregister := agenttask.GetRegistry().Add(theAgent.ID)
	defer func() {
		deregister()
		clearConnection(theAgent.ID, streamID)
	}()

	// Flush headers now rather than letting gRPC send them with the first
	// assignment. This is the agent's "you are subscribed" ack: without it an
	// idle stream is indistinguishable, client-side, from one that never came up.
	if err := stream.SendHeader(metadata.MD{}); err != nil {
		return err
	}

	logger.GetInfoLogger().Printf("agent %s subscribed to tasks (session %s, stream %s)", theAgent.AgentName, in.SessionId, streamID)

	for {
		select {
		case a, ok := <-ch:
			if !ok {
				return nil
			}

			msg := &pb.TaskAssignment{
				TaskId:       a.TaskID,
				Action:       a.Action,
				Data:         a.Data,
				Attempt:      a.Attempt,
				MaxAttempts:  a.MaxAttempts,
				LeaseToken:   a.LeaseToken,
				LeaseSeconds: a.LeaseSeconds,
			}

			if err := stream.Send(msg); err != nil {
				// The agent went away holding a claimed task. Return it now.
				if rErr := agenttask.Release(a.TaskID, a.LeaseToken); rErr != nil {
					logger.GetErrorLogger().Printf("error releasing task %s after send failure: %s", a.TaskID, rErr.Error())
				}
				return err
			}

		case <-stream.Context().Done():
			return nil

		case <-shutdown:
			logger.GetInfoLogger().Printf("closing task stream for agent %s: replica shutting down", theAgent.AgentName)
			return nil
		}
	}
}

func (s *Handler) ReportTaskStatus(ctx context.Context, in *pb.TaskStatusReport) (*pbModels.SSMEmpty, error) {
	if _, err := utils.GetAPIKeyFromContext(ctx); err != nil {
		return nil, err
	}

	switch in.Status {
	case pb.TaskStatus_RUNNING:
		if err := agenttask.ReportProgress(in.TaskId, in.LeaseToken, in.ProgressPercent, in.Message); err != nil {
			return nil, err
		}
	case pb.TaskStatus_COMPLETED:
		if err := agenttask.Complete(in.TaskId, in.LeaseToken); err != nil {
			return nil, err
		}
	case pb.TaskStatus_FAILED:
		if err := agenttask.Fail(in.TaskId, in.LeaseToken, in.Error); err != nil {
			return nil, err
		}
	case pb.TaskStatus_RELEASED:
		if err := agenttask.Release(in.TaskId, in.LeaseToken); err != nil {
			return nil, err
		}
	}

	return &pbModels.SSMEmpty{}, nil
}

func (s *Handler) RenewTaskLease(ctx context.Context, in *pb.TaskLeaseRequest) (*pb.TaskLeaseResponse, error) {
	if _, err := utils.GetAPIKeyFromContext(ctx); err != nil {
		return nil, err
	}

	ok, cancelRequested, err := agenttask.RenewLease(in.TaskId, in.LeaseToken)
	if err != nil {
		return nil, err
	}

	return &pb.TaskLeaseResponse{Ok: ok, CancelRequested: cancelRequested}, nil
}

// enqueueBootUpdate honours ServerConfig.UpdateOnStart, which used to be the
// agent's own boot-time install. It is backend policy now.
//
// Skipped when nothing is installed: updatesfserver fails on a bare machine, and
// the install task (from the workflow or the user) owns that case.
//
// Keyed on the agent's session id, so a reconnect loop cannot stack up update
// tasks while one is still queued. The dedupe index only constrains active tasks,
// so a reconnect after the update finished does enqueue another; that one is a
// cheap no-op, since UpdateSFServer returns early when already at the available
// version.
func enqueueBootUpdate(theAgent *v2.AgentSchema, sessionID string) {
	if !theAgent.ServerConfig.UpdateOnStart || !theAgent.Status.Installed || sessionID == "" {
		return
	}

	accountID, err := agent.GetAccountIDForAgent(theAgent.ID)
	if err != nil {
		logger.GetErrorLogger().Printf("error resolving account for agent %s: %s", theAgent.ID.Hex(), err.Error())
		return
	}

	if _, err := agenttask.Enqueue(
		theAgent.ID, accountID, "updatesfserver", nil,
		agenttask.BootUpdateDedupeKey(sessionID),
		v2.TaskTrigger{Type: v2.TaskTriggerSystem},
		agenttask.EnqueueOpts{},
	); err != nil {
		logger.GetErrorLogger().Printf("error enqueuing boot update for agent %s: %s", theAgent.ID.Hex(), err.Error())
	}
}

func setConnection(agentID bson.ObjectID, streamID string) error {
	col := repositories.GetMongoClient().GetCollection("agents")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := col.UpdateOne(ctx,
		bson.M{"_id": agentID},
		bson.M{"$set": bson.M{"connectedTo": replicaID, "connectionId": streamID, "updatedAt": time.Now()}})
	return err
}

// clearConnection only detaches if this stream is still the current one, so a
// slow teardown cannot detach a freshly reconnected agent.
func clearConnection(agentID bson.ObjectID, streamID string) {
	col := repositories.GetMongoClient().GetCollection("agents")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := col.UpdateOne(ctx,
		bson.M{"_id": agentID, "connectionId": streamID},
		bson.M{"$unset": bson.M{"connectedTo": "", "connectionId": ""}}); err != nil {
		logger.GetErrorLogger().Printf("error clearing agent connection: %s", err.Error())
	}
}

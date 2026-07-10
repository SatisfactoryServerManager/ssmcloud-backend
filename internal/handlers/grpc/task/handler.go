package task

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Handler struct {
	pb.UnimplementedAgentTaskServiceServer
}

// replicaID identifies this process in agent.connectedTo. It only needs to be
// unique per running replica, not stable across restarts.
var replicaID = bson.NewObjectID().Hex()

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

	if err := setConnection(theAgent.ID, in.ConnectionId); err != nil {
		return err
	}

	ch, deregister := agenttask.GetRegistry().Add(theAgent.ID, in.ConnectionId)
	defer func() {
		deregister()
		clearConnection(theAgent.ID, in.ConnectionId)
	}()

	logger.GetInfoLogger().Printf("agent %s subscribed to tasks (conn %s)", theAgent.AgentName, in.ConnectionId)

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

func setConnection(agentID bson.ObjectID, connectionID string) error {
	col := repositories.GetMongoClient().GetCollection("agents")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := col.UpdateOne(ctx,
		bson.M{"_id": agentID},
		bson.M{"$set": bson.M{"connectedTo": replicaID, "connectionId": connectionID, "updatedAt": time.Now()}})
	return err
}

// clearConnection only detaches if this connection is still the current one, so
// a slow teardown cannot detach a freshly reconnected agent.
func clearConnection(agentID bson.ObjectID, connectionID string) {
	col := repositories.GetMongoClient().GetCollection("agents")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := col.UpdateOne(ctx,
		bson.M{"_id": agentID, "connectionId": connectionID},
		bson.M{"$unset": bson.M{"connectedTo": "", "connectionId": ""}}); err != nil {
		logger.GetErrorLogger().Printf("error clearing agent connection: %s", err.Error())
	}
}

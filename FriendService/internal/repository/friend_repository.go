package repository

import (
	"context"
	"errors"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/yonaje/friendservice/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("friendservice/repository")

var (
	ErrSelfRequest           = errors.New("user cannot interact with self")
	ErrAlreadyFriends        = errors.New("users are already friends")
	ErrRequestAlreadyExists  = errors.New("friend request already exists")
	ErrIncomingRequestExists = errors.New("incoming friend request already exists")
	ErrRequestNotFound       = errors.New("friend request not found")
)

type FriendRepository interface {
	SendRequest(ctx context.Context, fromUserID int, toUserID int) error
	AcceptRequest(ctx context.Context, fromUserID int, toUserID int) error
	ListFriends(ctx context.Context, userID int) ([]models.Friend, error)
	ListIncomingRequests(ctx context.Context, userID int) ([]models.FriendRequest, error)
	Recommendations(ctx context.Context, userID int, limit int) ([]models.FriendRecommendation, error)
}

type friendRepository struct {
	driver neo4j.DriverWithContext
}

func NewFriendRepository(driver neo4j.DriverWithContext) FriendRepository {
	return &friendRepository{driver: driver}
}

func (r *friendRepository) SendRequest(ctx context.Context, fromUserID int, toUserID int) error {
	ctx, span := tracer.Start(ctx, "FriendRepository.SendRequest")
	defer span.End()
	span.SetAttributes(
		attribute.Int("friend.from_user_id", fromUserID),
		attribute.Int("friend.to_user_id", toUserID),
	)

	if fromUserID == toUserID {
		span.RecordError(ErrSelfRequest)
		span.SetStatus(codes.Error, "user cannot interact with self")
		return ErrSelfRequest
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		params := map[string]any{
			"fromUserID": fromUserID,
			"toUserID":   toUserID,
		}

		if _, err := tx.Run(ctx, `
			MERGE (:User {id: $fromUserID})
			MERGE (:User {id: $toUserID})
		`, params); err != nil {
			return nil, err
		}

		if count, err := getRelationshipCount(ctx, tx, `
			MATCH (:User {id: $fromUserID})-[r:FRIEND]->(:User {id: $toUserID})
			RETURN COUNT(r) AS count
		`, params); err != nil {
			return nil, err
		} else if count > 0 {
			return nil, ErrAlreadyFriends
		}

		if count, err := getRelationshipCount(ctx, tx, `
			MATCH (:User {id: $fromUserID})-[r:FRIEND_REQUESTED]->(:User {id: $toUserID})
			RETURN COUNT(r) AS count
		`, params); err != nil {
			return nil, err
		} else if count > 0 {
			return nil, ErrRequestAlreadyExists
		}

		if count, err := getRelationshipCount(ctx, tx, `
			MATCH (:User {id: $toUserID})-[r:FRIEND_REQUESTED]->(:User {id: $fromUserID})
			RETURN COUNT(r) AS count
		`, params); err != nil {
			return nil, err
		} else if count > 0 {
			return nil, ErrIncomingRequestExists
		}

		params["createdAt"] = time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.Run(ctx, `
			MATCH (from:User {id: $fromUserID}), (to:User {id: $toUserID})
			CREATE (from)-[:FRIEND_REQUESTED {createdAt: $createdAt}]->(to)
		`, params); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send friend request")
		return err
	}

	span.SetStatus(codes.Ok, "friend request sent successfully")
	return nil
}

func (r *friendRepository) AcceptRequest(ctx context.Context, fromUserID int, toUserID int) error {
	ctx, span := tracer.Start(ctx, "FriendRepository.AcceptRequest")
	defer span.End()
	span.SetAttributes(
		attribute.Int("friend.from_user_id", fromUserID),
		attribute.Int("friend.to_user_id", toUserID),
	)

	if fromUserID == toUserID {
		span.RecordError(ErrSelfRequest)
		span.SetStatus(codes.Error, "user cannot interact with self")
		return ErrSelfRequest
	}

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		params := map[string]any{
			"fromUserID": fromUserID,
			"toUserID":   toUserID,
			"createdAt":  time.Now().UTC().Format(time.RFC3339),
		}

		if count, err := getRelationshipCount(ctx, tx, `
			MATCH (:User {id: $fromUserID})-[r:FRIEND_REQUESTED]->(:User {id: $toUserID})
			RETURN COUNT(r) AS count
		`, params); err != nil {
			return nil, err
		} else if count == 0 {
			return nil, ErrRequestNotFound
		}

		if _, err := tx.Run(ctx, `
			MATCH (from:User {id: $fromUserID})-[request:FRIEND_REQUESTED]->(to:User {id: $toUserID})
			DELETE request
			MERGE (from)-[:FRIEND {createdAt: $createdAt}]->(to)
			MERGE (to)-[:FRIEND {createdAt: $createdAt}]->(from)
		`, params); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to accept friend request")
		return err
	}

	span.SetStatus(codes.Ok, "friend request accepted successfully")
	return nil
}

func (r *friendRepository) ListFriends(ctx context.Context, userID int) ([]models.Friend, error) {
	ctx, span := tracer.Start(ctx, "FriendRepository.ListFriends")
	defer span.End()
	span.SetAttributes(attribute.Int("user.id", userID))

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cursor, err := tx.Run(ctx, `
			MATCH (:User {id: $userID})-[:FRIEND]->(friend:User)
			RETURN DISTINCT friend.id AS id
			ORDER BY id
		`, map[string]any{"userID": userID})
		if err != nil {
			return nil, err
		}

		friends := make([]models.Friend, 0)
		for cursor.Next(ctx) {
			id, _ := cursor.Record().Get("id")
			friends = append(friends, models.Friend{ID: int(id.(int64))})
		}

		return friends, cursor.Err()
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get friends")
		return nil, err
	}

	friends := result.([]models.Friend)
	span.SetAttributes(attribute.Int("friend.count", len(friends)))
	span.SetStatus(codes.Ok, "friends retrieved successfully")
	return friends, nil
}

func (r *friendRepository) ListIncomingRequests(ctx context.Context, userID int) ([]models.FriendRequest, error) {
	ctx, span := tracer.Start(ctx, "FriendRepository.ListIncomingRequests")
	defer span.End()
	span.SetAttributes(attribute.Int("user.id", userID))

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cursor, err := tx.Run(ctx, `
			MATCH (from:User)-[request:FRIEND_REQUESTED]->(:User {id: $userID})
			RETURN from.id AS fromUserID, request.createdAt AS createdAt
			ORDER BY createdAt DESC, fromUserID ASC
		`, map[string]any{"userID": userID})
		if err != nil {
			return nil, err
		}

		requests := make([]models.FriendRequest, 0)
		for cursor.Next(ctx) {
			fromUserID, _ := cursor.Record().Get("fromUserID")
			createdAt, _ := cursor.Record().Get("createdAt")
			requests = append(requests, models.FriendRequest{
				FromUserID: int(fromUserID.(int64)),
				ToUserID:   userID,
				CreatedAt:  createdAt.(string),
			})
		}

		return requests, cursor.Err()
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get incoming requests")
		return nil, err
	}

	requests := result.([]models.FriendRequest)
	span.SetAttributes(attribute.Int("friend.request_count", len(requests)))
	span.SetStatus(codes.Ok, "incoming requests retrieved successfully")
	return requests, nil
}

func (r *friendRepository) Recommendations(ctx context.Context, userID int, limit int) ([]models.FriendRecommendation, error) {
	ctx, span := tracer.Start(ctx, "FriendRepository.Recommendations")
	defer span.End()
	span.SetAttributes(
		attribute.Int("user.id", userID),
		attribute.Int("recommendations.limit", limit),
	)

	session := r.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cursor, err := tx.Run(ctx, `
			MATCH (user:User {id: $userID})-[:FRIEND]->(mutual:User)-[:FRIEND]->(candidate:User)
			WHERE candidate.id <> $userID
				AND NOT (user)-[:FRIEND]->(candidate)
				AND NOT (user)-[:FRIEND_REQUESTED]->(candidate)
				AND NOT (candidate)-[:FRIEND_REQUESTED]->(user)
			RETURN candidate.id AS userID, COUNT(DISTINCT mutual) AS mutualFriends
			ORDER BY mutualFriends DESC, userID ASC
			LIMIT $limit
		`, map[string]any{
			"userID": userID,
			"limit":  limit,
		})
		if err != nil {
			return nil, err
		}

		recommendations := make([]models.FriendRecommendation, 0)
		for cursor.Next(ctx) {
			userIDValue, _ := cursor.Record().Get("userID")
			mutualFriendsValue, _ := cursor.Record().Get("mutualFriends")
			recommendations = append(recommendations, models.FriendRecommendation{
				UserID:        int(userIDValue.(int64)),
				MutualFriends: int(mutualFriendsValue.(int64)),
			})
		}

		return recommendations, cursor.Err()
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get recommendations")
		return nil, err
	}

	recommendations := result.([]models.FriendRecommendation)
	span.SetAttributes(attribute.Int("recommendations.count", len(recommendations)))
	span.SetStatus(codes.Ok, "recommendations retrieved successfully")
	return recommendations, nil
}

func getRelationshipCount(ctx context.Context, tx neo4j.ManagedTransaction, query string, params map[string]any) (int64, error) {
	cursor, err := tx.Run(ctx, query, params)
	if err != nil {
		return 0, err
	}

	record, err := cursor.Single(ctx)
	if err != nil {
		return 0, err
	}

	count, _ := record.Get("count")
	return count.(int64), nil
}

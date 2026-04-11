package service

import "context"

type EventPublisher interface {
	Publish(ctx context.Context, input PublishDomainEventInput) error
}

type PublishDomainEventInput struct {
	EventType      string
	WorkspaceType  string
	OrganizationID string
	ActorUserID    string
	ResourceType   string
	ResourceID     string
	Payload        any
}

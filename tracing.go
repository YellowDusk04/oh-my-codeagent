package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/baggage"
)

// NewBaggageContext creates a context with Langfuse baggage attributes.
// This allows Langfuse to associate traces with the correct user and session.
//
// Parameters:
//   - userID: the user identifier (maps to langfuse.user.id)
//   - sessionID: the session identifier (maps to langfuse.session.id)
func NewBaggageContext(userID, sessionID string) context.Context {
	mSession, err := baggage.NewMemberRaw("langfuse.session.id", sessionID)
	if err != nil {
		log.Printf("Failed to create baggage member for session: %v", err)
	}
	mUser, err := baggage.NewMemberRaw("langfuse.user.id", userID)
	if err != nil {
		log.Printf("Failed to create baggage member for user: %v", err)
	}

	bag, err := baggage.New(mSession, mUser)
	if err != nil {
		log.Printf("Failed to create baggage: %v", err)
	}

	return baggage.ContextWithBaggage(context.Background(), bag)
}

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"lijiaoqiao/supply-api/internal/repository"
)

type fakeStreamClient struct {
	xaddArgs    []*redis.XAddArgs
	xaddErr     error
	xgroupErr   error
	streamName  string
	groupName   string
	startOffset string
}

func (c *fakeStreamClient) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	c.xaddArgs = append(c.xaddArgs, a)
	return redis.NewStringResult("1-0", c.xaddErr)
}

func (c *fakeStreamClient) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	c.streamName = stream
	c.groupName = group
	c.startOffset = start
	return redis.NewStatusResult("OK", c.xgroupErr)
}

func TestOutboxMessageBrokerPublishBuildsExpectedEnvelope(t *testing.T) {
	client := &fakeStreamClient{}
	broker := newOutboxMessageBrokerWithClient(client, "supply-outbox", "relay")
	event := &repository.OutboxEvent{
		EventID:       "evt-1",
		AggregateType: "account",
		AggregateID:   "acc-1",
		EventType:     "created",
		Payload:       json.RawMessage(`{"provider":"openai"}`),
	}

	if err := broker.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(client.xaddArgs) != 1 {
		t.Fatalf("XAdd call count = %d, want 1", len(client.xaddArgs))
	}

	call := client.xaddArgs[0]
	if call.Stream != "supply-outbox" {
		t.Fatalf("stream = %q, want %q", call.Stream, "supply-outbox")
	}
	if call.ID != "*" {
		t.Fatalf("id = %q, want %q", call.ID, "*")
	}

	values, ok := call.Values.(map[string]interface{})
	if !ok {
		t.Fatalf("values type = %T, want map[string]interface{}", call.Values)
	}

	rawData, ok := values["data"].(string)
	if !ok {
		t.Fatalf("data field type = %T, want string", values["data"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rawData), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if payload["event_id"] != event.EventID || payload["aggregate_type"] != event.AggregateType || payload["aggregate_id"] != event.AggregateID {
		t.Fatalf("unexpected envelope: %#v", payload)
	}
	if payload["payload"] != string(event.Payload) {
		t.Fatalf("payload = %#v, want %q", payload["payload"], string(event.Payload))
	}
	if _, err := time.Parse(time.RFC3339, payload["published_at"].(string)); err != nil {
		t.Fatalf("published_at parse error = %v", err)
	}
}

func TestEnsureConsumerGroupHandlesBusyGroupAndOtherErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{
			name:    "busy group is treated as success",
			err:     errors.New("BUSYGROUP Consumer Group name already exists"),
			wantErr: false,
		},
		{
			name:    "other errors are wrapped",
			err:     errors.New("redis unavailable"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeStreamClient{xgroupErr: tt.err}
			broker := newOutboxMessageBrokerWithClient(client, "supply-outbox", "relay")

			err := broker.EnsureConsumerGroup(context.Background())
			if tt.wantErr && err == nil {
				t.Fatal("EnsureConsumerGroup() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("EnsureConsumerGroup() error = %v, want nil", err)
			}
			if client.streamName != "supply-outbox" || client.groupName != "relay" || client.startOffset != "0" {
				t.Fatalf("unexpected group args: stream=%q group=%q start=%q", client.streamName, client.groupName, client.startOffset)
			}
		})
	}
}

package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Shopify/sarama"
	kafka "github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	instancev1 "github.com/cownetwork/instance-controller/api/v1"
	instanceapiv1 "github.com/cownetwork/mooapis-go/cow/instance/v1"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

type Emitter struct {
	c      cloudevents.Client
	sender *kafka.Sender
	source string
}

// NewEmitter creates an new Emitter that emitts events in the cloud event Kafka format
// to the configured Kafka brokers
func NewEmitter(brokers []string, topic, source string) (*Emitter, error) {
	const op = "events/NewPublisher"
	config := sarama.NewConfig()
	config.Version = sarama.V2_0_0_0

	sender, err := kafka.NewSender(brokers, config, topic)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", op, err)
	}

	c, err := cloudevents.NewClient(sender, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		return nil, fmt.Errorf("%s: %v", op, err)
	}

	return &Emitter{
		c:      c,
		sender: sender,
		source: source,
	}, nil
}

// InstanceCreated emitts an InstanceCreatedEvent
func (e *Emitter) InstanceCreated(ctx context.Context, instance *instancev1.Instance) error {
	const op = "event/emitter.InstanceCreated"
	protoinstance, err := instanceToProto(instance)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	msg := &instanceapiv1.InstanceStartedEvent{
		Instance: protoinstance,
	}

	event, err := makeCloudEvent("network.cow.instance.started.v1", e.source, msg)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	if result := e.c.Send(
		kafka.WithMessageKey(ctx, sarama.StringEncoder(event.ID())),
		event,
	); cloudevents.IsUndelivered(result) {
		return fmt.Errorf("%s: failed to send: %v", op, err)
	}
	return nil
}

// InstanceEnded emitts an InstanceEndedEvent
func (e *Emitter) InstanceEnded(ctx context.Context, instance *instancev1.Instance) error {
	const op = "event/emitter.InstanceEnded"
	protoinstance, err := instanceToProto(instance)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	msg := &instanceapiv1.InstanceEndedEvent{
		Instance: protoinstance,
	}

	event, err := makeCloudEvent("network.cow.instance.ended.v1", e.source, msg)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	if result := e.c.Send(
		kafka.WithMessageKey(ctx, sarama.StringEncoder(event.ID())),
		event,
	); cloudevents.IsUndelivered(result) {
		return fmt.Errorf("%s: failed to send: %v", op, err)
	}
	return nil
}

// InstanceStateChanged emitts an InstanceStateChangedEvent
func (e *Emitter) InstanceStateChanged(
	ctx context.Context,
	instance *instancev1.Instance,
	old json.RawMessage,
	new json.RawMessage,
) error {
	const op = "event/emitter.InstanceStateChanged"
	protoinstance, err := instanceToProto(instance)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	oldstate, err := toStructpb(old)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	newstate, err := toStructpb(new)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	msg := &instanceapiv1.InstanceStateChangedEvent{
		Instance: protoinstance,
		OldState: oldstate,
		NewState: newstate,
	}

	event, err := makeCloudEvent("network.cow.instance.state-changed.v1", e.source, msg)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	if result := e.c.Send(
		kafka.WithMessageKey(ctx, sarama.StringEncoder(event.ID())),
		event,
	); cloudevents.IsUndelivered(result) {
		return fmt.Errorf("%s: failed to send: %v", op, err)
	}
	return nil
}

func makeCloudEvent(eventtype, source string, msg proto.Message) (cloudevents.Event, error) {
	const op = "event/makeCloudEvent"
	event := cloudevents.NewEvent()
	id, err := uuid.NewRandom()
	if err != nil {
		return cloudevents.Event{}, fmt.Errorf("%s: %v", op, err)
	}

	event.SetID(id.String())
	event.SetSource(source)
	event.SetType(eventtype)

	data, err := proto.Marshal(msg)
	if err != nil {
		return cloudevents.Event{}, fmt.Errorf("%s: %v", op, err)
	}

	if err := event.SetData("application/protobuf", data); err != nil {
		return cloudevents.Event{}, fmt.Errorf("%s: %v", op, err)
	}

	return event, nil
}

func instanceToProto(instance *instancev1.Instance) (*instanceapiv1.Instance, error) {
	const op = "event/instanceToProto"

	structval, err := toStructpb(instance.Status.Metadata.State)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", op, err)
	}

	data := &instanceapiv1.Instance{
		Id:    instance.Status.ID,
		Name:  instance.Name,
		Ip:    instance.Status.IP,
		State: toAPIState(instance.Status.State),
		Metadata: &instanceapiv1.Metadata{
			State: structval,
		},
	}
	players := make([]*instanceapiv1.Player, 0)
	for _, p := range instance.Status.Metadata.Players {
		apiplayer, err := toAPIPlayer(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", op, err)
		}
		players = append(players, apiplayer)
	}
	data.Metadata.Players = players
	return data, nil
}

func toAPIPlayer(player instancev1.InstancePlayer) (*instanceapiv1.Player, error) {
	const op = "event/toAPIPlayer"
	structval, err := toStructpb(player.Metadata)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", op, err)
	}
	return &instanceapiv1.Player{
		Id:       player.ID,
		Metadata: structval,
	}, nil
}

func toAPIState(state instancev1.InstanceState) instanceapiv1.Instance_State {
	switch state {
	case instancev1.StateInitializing:
		return instanceapiv1.Instance_STATE_INITIALIZING
	case instancev1.StateRunning:
		return instanceapiv1.Instance_STATE_RUNNING
	case instancev1.StateEnding:
		return instanceapiv1.Instance_STATE_ENDING
	}
	return instanceapiv1.Instance_STATE_UNKNOWN
}

func toStructpb(raw json.RawMessage) (*structpb.Struct, error) {
	const op = "event/toStructpb"
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("%s: %v", op, err)
	}
	structval, err := structpb.NewStruct(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", op, err)
	}
	return structval, nil
}

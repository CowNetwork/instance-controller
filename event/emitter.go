package event

import (
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

const (
	SourceURI = "cow.global.instance-service"
)

type Emitter struct {
	c      cloudevents.Client
	sender *kafka.Sender
}

func NewPublisher(brokers []string, topic string) (*Emitter, error) {
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
	}, nil
}

func InstanceCreated(instance instancev1.Instance) {

}

func (p *Emitter) makeCloudEvent(eventtype string, msg proto.Message) error {
	const op = "events/publisher.Publish"
	event := cloudevents.NewEvent()
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	event.SetID(id.String())
	event.SetSource(SourceURI)
	event.SetType(eventtype)

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	if err := event.SetData("application/protobuf", data); err != nil {
		return fmt.Errorf("%s: %v", op, err)
	}

	return nil
}

func instanceToProto(instance *instancev1.Instance) (*instanceapiv1.Instance, error) {
	const op = "events/instanceToProto"

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
	const op = "events/toAPIPlayer"
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
	const op = "events/toStructpb"
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

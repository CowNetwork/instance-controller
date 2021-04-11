package v1

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceState is the current observed state of the instance
// It is restricted to Initializing, Running and Ending
// Applications can register their custom States in InstanceMetadata
// +kubebuilder:validation:Enum=Initializing;Running;Ending
type InstanceState string

const (
	// InitializingState indicates that the Instance is starting.
	// Players should not be able to connect at this point.
	StateInitializing InstanceState = "Initializing"

	// StateRunning indicates that initialization has been completed an players
	// can or have already connected.
	StateRunning InstanceState = "Running"

	// EndingState indicates that the Instance is about to shutdown.
	// At this stage it should be save to kill the Instance at any point.
	StateEnding InstanceState = "Ending"
)

// InstanceSpec defines the desired state of Instance
type InstanceSpec struct {
	// Template defines the underlying pod that will be started when creating the Instance
	Template corev1.PodSpec `json:"template"`
}

// InstanceStatus defines the observed state of Instance
type InstanceStatus struct {

	// State holds the current observed state of the instance
	State InstanceState `json:"state,omitempty"`

	// IP address assigned to the Instance
	IP string `json:"ip,omitempty"`

	// Unique ID of the instance
	ID string `json:"id,omitempty"`

	// Metadata holds application specific metadata about the instance
	Metadata InstanceMetadata `json:"metadata,omitempty"`
}

// InstanceMetadata defines the metadata of the Instance
type InstanceMetadata struct {
	// State holds the current observed state of the application.
	State json.RawMessage `json:"state,omitempty"`

	// Players currently connected to this Instance.
	Players []InstancePlayer `json:"players,omitempty"`
}

// InstancePlayer defines metadata of a player connected to this instance
type InstancePlayer struct {
	// ID of this player
	ID string `json:"id"`

	// Metadata contains custom metadata about this player
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// +kubebuilder:object:root=true

// Instance is the Schema for the instances API
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec,omitempty"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}

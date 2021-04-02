package v1

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceSpec defines the desired state of Instance
type InstanceSpec struct {

	// Template defines the underlying pod that will be started when creating the Instance
	Template corev1.PodSpec `json:"template"`
}

// InstanceStatus defines the observed state of Instance
type InstanceStatus struct {

	// State holds the current observed state of the instance
	// It is restricted to Initializing, Running and Ending
	// Applications can register their custom States in InstanceMetadata
	// These states have the following meanings:
	// - Initializing: Instance is starting players should not be able to connect
	// - Running: Initialization is complete players can or have already joined
	// - Ending: Instance is about to shutdown. At this stage it should be save to
	// kill the instance at any point.
	// +kubebuilder:validation:Enum=Initializing;Running;Ending
	State string `json:"state"`

	// Metadata holds application specific metadata about the instance
	Metadata InstanceMetadata `json:"metadata"`
}

// InstanceMetadata defines the metadata of the Instance
type InstanceMetadata struct {
	// State holds the current observed state of the application.
	State string `json:"state"`

	// Players currently connected to this Instance.
	Players []InstancePlayer `json:"players"`
}

// InstancePlayer defines metadata of a player connected to this instance
type InstancePlayer struct {
	// ID of this player
	ID string `json:"id"`

	// Metadata contains custom metadata about this player
	Metadata json.RawMessage `json:"metadata"`
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

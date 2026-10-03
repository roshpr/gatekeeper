package v1beta1

import (
	"crypto/sha256"
	"encoding/hex"

	"k8s.io/apimachinery/pkg/util/validation"
)

// podLabelHashPrefix marks values derived from pod names that are not
// themselves valid Kubernetes label values. The remainder is a stable
// SHA-256 prefix, so the same pod name always maps to the same label.
const podLabelHashPrefix = "pod-"

// PodLabelValue returns a Kubernetes label value that identifies podName.
//
// Names that are already valid label values are returned unchanged, so
// existing status objects for ordinary pod names keep matching. Names that
// Kubernetes would reject (notably those longer than 63 characters, as with
// static-pod mirror names) are replaced with a stable digest that fits the
// label value limit. The digest is not the pod's name; status ID and owner
// references continue to store the full name.
func PodLabelValue(podName string) string {
	if len(validation.IsValidLabelValue(podName)) == 0 {
		return podName
	}
	sum := sha256.Sum256([]byte(podName))
	// 16 bytes encode to 32 hex characters. With the prefix, the value stays
	// within the 63 character limit and is unique for any realistic set of pods.
	return podLabelHashPrefix + hex.EncodeToString(sum[:16])
}

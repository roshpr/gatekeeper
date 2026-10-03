package v1beta1_test

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/fakes"
	"github.com/open-policy-agent/gatekeeper/v3/test/testutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestPodLabelValue(t *testing.T) {
	shortName := "gatekeeper-controller-manager-7f8c9d"
	maxName := strings.Repeat("a", validation.LabelValueMaxLength)
	longName := strings.Repeat("b", validation.LabelValueMaxLength+1)
	otherLongName := "gatekeeper-controller-manager.ip-10-0-1-12.us-west-2.compute.internal"

	if len(otherLongName) <= validation.LabelValueMaxLength {
		t.Fatalf("fixture pod name length = %d, want > %d", len(otherLongName), validation.LabelValueMaxLength)
	}

	tests := []struct {
		name     string
		podName  string
		wantSame bool
	}{
		{name: "short name is unchanged", podName: shortName, wantSame: true},
		{name: "max length name is unchanged", podName: maxName, wantSame: true},
		{name: "name over 63 characters is hashed", podName: longName, wantSame: false},
		{name: "static pod mirror name is hashed", podName: otherLongName, wantSame: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v1beta1.PodLabelValue(tt.podName)
			if errs := validation.IsValidLabelValue(got); len(errs) > 0 {
				t.Fatalf("PodLabelValue(%q) = %q is not a valid label value: %v", tt.podName, got, errs)
			}
			if tt.wantSame && got != tt.podName {
				t.Fatalf("PodLabelValue(%q) = %q, want the original name", tt.podName, got)
			}
			if !tt.wantSame && got == tt.podName {
				t.Fatalf("PodLabelValue(%q) returned the original name", tt.podName)
			}
			if again := v1beta1.PodLabelValue(tt.podName); again != got {
				t.Fatalf("PodLabelValue is not stable: %q then %q", got, again)
			}
		})
	}

	if v1beta1.PodLabelValue(longName) == v1beta1.PodLabelValue(otherLongName) {
		t.Fatal("different long pod names produced the same label value")
	}
}

func TestNewConstraintTemplateStatusForPod_LongPodName(t *testing.T) {
	podName := "gatekeeper-controller-manager.ip-10-0-1-12.us-west-2.compute.internal"
	podNS := "gatekeeper-system"
	templateName := "denyall"

	if len(podName) <= validation.LabelValueMaxLength {
		t.Fatalf("pod name length = %d, want > %d", len(podName), validation.LabelValueMaxLength)
	}

	testutils.Setenv(t, "POD_NAMESPACE", podNS)

	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pod := fakes.Pod(
		fakes.WithNamespace(podNS),
		fakes.WithName(podName),
	)

	status, err := v1beta1.NewConstraintTemplateStatusForPod(pod, templateName, scheme)
	if err != nil {
		t.Fatal(err)
	}

	if status.Status.ID != podName {
		t.Fatalf("status ID = %q, want full pod name %q", status.Status.ID, podName)
	}
	if len(status.OwnerReferences) != 1 || status.OwnerReferences[0].Name != podName {
		t.Fatalf("owner reference = %+v, want pod name %q", status.OwnerReferences, podName)
	}

	gotLabel := status.Labels[v1beta1.PodLabel]
	if errs := validation.IsValidLabelValue(gotLabel); len(errs) > 0 {
		t.Fatalf("pod label %q is invalid: %v", gotLabel, errs)
	}
	if gotLabel != v1beta1.PodLabelValue(podName) {
		t.Fatalf("pod label = %q, want %q", gotLabel, v1beta1.PodLabelValue(podName))
	}
	if gotLabel == podName {
		t.Fatal("pod label stored the raw pod name")
	}

	if errs := validation.IsDNS1123Subdomain(status.Name); len(errs) > 0 {
		t.Fatalf("status name %q is invalid: %v", status.Name, errs)
	}
}

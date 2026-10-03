package constrainttemplatestatus

import (
	"context"
	"testing"

	"github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/fakes"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/util"
	"github.com/open-policy-agent/gatekeeper/v3/test/testutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestPodStatusToConstraintTemplateMapper_LongPodName(t *testing.T) {
	podName := "gatekeeper-controller-manager.ip-10-0-1-12.us-west-2.compute.internal"
	otherPod := podName + "-other"
	templateName := "denyall"

	if len(podName) <= validation.LabelValueMaxLength {
		t.Fatalf("pod name length = %d, want > %d", len(podName), validation.LabelValueMaxLength)
	}

	testutils.Setenv(t, "POD_NAME", podName)
	testutils.Setenv(t, "POD_NAMESPACE", "gatekeeper-system")

	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	pod := fakes.Pod(fakes.WithNamespace("gatekeeper-system"), fakes.WithName(podName))
	status, err := v1beta1.NewConstraintTemplateStatusForPod(pod, templateName, scheme)
	if err != nil {
		t.Fatal(err)
	}

	mapper := PodStatusToConstraintTemplateMapper(true)
	got := mapper(context.Background(), status)
	if len(got) != 1 || got[0].Name != templateName {
		t.Fatalf("mapper requests = %+v, want template %q", got, templateName)
	}

	other := status.DeepCopy()
	other.Labels[v1beta1.PodLabel] = v1beta1.PodLabelValue(otherPod)
	if reqs := mapper(context.Background(), other); len(reqs) != 0 {
		t.Fatalf("mapper requests for another pod = %+v, want none", reqs)
	}

	if util.GetPodName() != podName {
		t.Fatalf("POD_NAME = %q, want %q", util.GetPodName(), podName)
	}
	if status.Labels[v1beta1.PodLabel] == podName {
		t.Fatal("status label stored the raw pod name")
	}
}

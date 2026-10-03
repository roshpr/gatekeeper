package constrainttemplate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	statusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/fakes"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/util"
	"github.com/open-policy-agent/gatekeeper/v3/test/testutils"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// longMirrorPodName imitates a static-pod mirror name: the pod name plus a
// long node name. It is a valid pod name and longer than a label value.
const longMirrorPodName = "gatekeeper-controller-manager.ip-10-0-1-12.us-west-2.compute.internal"

// labelValidatingClient applies the API server's metadata checks that reject
// label values longer than 63 characters. The fake client does not.
type labelValidatingClient struct {
	client.Client
}

func (c *labelValidatingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if err := validateStatusMetadata(obj); err != nil {
		return err
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *labelValidatingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if err := validateStatusMetadata(obj); err != nil {
		return err
	}
	return c.Client.Update(ctx, obj, opts...)
}

func validateStatusMetadata(obj client.Object) error {
	if errs := validation.IsDNS1123Subdomain(obj.GetName()); len(errs) > 0 {
		return fmt.Errorf("metadata.name %q: %s", obj.GetName(), strings.Join(errs, ", "))
	}
	for key, value := range obj.GetLabels() {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return fmt.Errorf("label key %q: %s", key, strings.Join(errs, ", "))
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("label %q value %q: %s", key, value, strings.Join(errs, ", "))
		}
	}
	return nil
}

func TestConstraintTemplateStatusUpdate_LongPodName(t *testing.T) {
	require.Greater(t, len(longMirrorPodName), validation.LabelValueMaxLength)

	testutils.Setenv(t, "POD_NAME", longMirrorPodName)
	testutils.Setenv(t, "POD_NAMESPACE", "gatekeeper-system")

	scheme := runtime.NewScheme()
	require.NoError(t, statusv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	pod := fakes.Pod(
		fakes.WithNamespace("gatekeeper-system"),
		fakes.WithName(longMirrorPodName),
	)
	base := crfake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	r := &ReconcileConstraintTemplate{
		Client: &labelValidatingClient{Client: base},
		scheme: scheme,
		getPod: func(context.Context) (*corev1.Pod, error) { return pod, nil },
	}

	ctx := context.Background()
	const templateName = "denyall"

	created, err := r.getOrCreatePodStatus(ctx, templateName)
	require.NoError(t, err)
	require.Equal(t, longMirrorPodName, created.Status.ID)
	require.Equal(t, statusv1beta1.PodLabelValue(longMirrorPodName), created.Labels[statusv1beta1.PodLabel])
	require.NotEqual(t, longMirrorPodName, created.Labels[statusv1beta1.PodLabel])
	require.Empty(t, validation.IsValidLabelValue(created.Labels[statusv1beta1.PodLabel]))

	oldStatus := created.Status.DeepCopy()
	created.Status.ObservedGeneration = 2
	created.Status.TemplateUID = "template-uid"
	require.NoError(t, r.persistPodStatus(ctx, created, oldStatus))

	key := types.NamespacedName{Name: created.Name, Namespace: util.GetNamespace()}
	got := &statusv1beta1.ConstraintTemplatePodStatus{}
	require.NoError(t, r.Get(ctx, key, got))
	require.Equal(t, int64(2), got.Status.ObservedGeneration)
	require.Equal(t, types.UID("template-uid"), got.Status.TemplateUID)
	require.Equal(t, longMirrorPodName, got.Status.ID)
	require.Equal(t, statusv1beta1.PodLabelValue(longMirrorPodName), got.Labels[statusv1beta1.PodLabel])
	require.Len(t, got.OwnerReferences, 1)
	require.Equal(t, longMirrorPodName, got.OwnerReferences[0].Name)

	again, err := r.getOrCreatePodStatus(ctx, templateName)
	require.NoError(t, err)
	require.Equal(t, got.Name, again.Name)
	require.Equal(t, int64(2), again.Status.ObservedGeneration)
}

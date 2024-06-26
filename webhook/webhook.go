package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	corev1 "k8s.io/api/core/v1"
)

type InitContainerAdder struct {
	Client client.Client

	log logr.Logger
}

//+kubebuilder:webhook:path=/mutate,mutating=true,failurePolicy=fail,groups=core,resources=pods,verbs=create,versions=v1,name=opensecrecy.io,admissionReviewVersions={v1,v1beta1},sideEffects=None

func (i *InitContainerAdder) Handle(ctx context.Context, req admission.Request) admission.Response {
	i.log = log.FromContext(ctx).WithValues("EncryptedSecret Webhook", req.Name)

	var pod *corev1.Pod

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		i.log.Info("cannot unmarshal object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// check if pod has annotation for encrypted secret and is set to true
	if addInitcontainer, ok := pod.Annotations["secrets.opensecrecy.org/inject-encrypted-secrets"]; !ok || addInitcontainer != "true" {
		i.log.Info("pod does not have annotation for encrypted secret or is not set to true")
		return admission.Allowed("")
	}

	// ToDo: use the container config from a configmap for easier updates
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:  "init-container",
		Image: "opensecrecy/initcontainer:v1alpha3",
		Command: []string{
			"/manager",
			"-encryptedsecret",
			"encryptedsecret-sample",
			"-namespace",
			"default",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "injected-secrets",
				MountPath: "/opt/secrets",
			},
		},
	})

	// add volume to pod
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "injected-secrets",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	marshaledPod, err := json.Marshal(pod)

	if err != nil {
		i.log.Info("cannot marshal object")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

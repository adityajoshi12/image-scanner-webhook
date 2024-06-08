package webhook

import (
	"context"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

type PodImageScanner struct {
	client  client.Client
	decoder *admission.Decoder
	logger  logr.Logger
}

func RegisterPodImageScanWebhook(mgr ctrl.Manager) error {

	m := PodImageScanner{
		client:  mgr.GetClient(),
		decoder: admission.NewDecoder(mgr.GetScheme()),

		logger: mgr.GetLogger(),
	}
	gvk, err := apiutil.GVKForObject(&v1.Pod{}, mgr.GetScheme())
	if err != nil {
		return err
	}
	mgr.GetWebhookServer().Register(GenerateMutatePath(gvk), &webhook.Admission{Handler: &m})
	return nil
}

// Handle implements the admission.Handler interface
func (a *PodImageScanner) Handle(_ context.Context, req admission.Request) admission.Response {

	d := &v1.Pod{}
	a.logger.Info("received request")

	err := a.decoder.Decode(req, d)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	var images []string
	for _, container := range d.Spec.Containers {
		images = append(images, container.Image)
	}

	// add init container
	initContainer, err := getInitContainer(images)

	d.Spec.InitContainers = append(d.Spec.InitContainers, initContainer)

	// set restartPolicy to Never
	d.Spec.RestartPolicy = v1.RestartPolicyNever

	// marshal and patch response
	marshaledPod, err := json.Marshal(d)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)

}

func getInitContainer(images []string) (v1.Container, error) {
	containerCommandTemplate := `
apk add jq;

{{scanCommand}}
`
	scanCommandTemplate := `
snyk container test {{image}}  --severity-threshold=high --json-file-output=/tmp/result.json;
passed=$(jq '.uniqueCount' /tmp/result.json) ;
if [ $passed -gt 0 ];
then
  exit 1;
fi;
`
	var command string
	var scanCommand []string

	for _, image := range images {

		scanCommand = append(scanCommand, strings.Replace(scanCommandTemplate, "{{image}}", image, -1))
	}

	command = strings.Replace(containerCommandTemplate, "{{scanCommand}}", strings.Join(scanCommand, ""), -1)

	return v1.Container{
		Name:    "image-scanner",
		Image:   "snyk/snyk:alpine",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{command},

		Env: []v1.EnvVar{
			{
				Name:  "SNYK_TOKEN",
				Value: os.Getenv("SNYK_TOKEN"),
			},
		},
	}, nil
}

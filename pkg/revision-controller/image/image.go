package image

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	openfunction "github.com/openfunction/apis/core/v1beta1"
	"github.com/openfunction/revision-controller/pkg/constants"
	revisioncontroller "github.com/openfunction/revision-controller/pkg/revision-controller"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RevisionController struct {
	client.Client
	log      logr.Logger
	fn       *openfunction.Function
	config   *Config
	keychain authn.Keychain

	stopCh chan os.Signal
}

type Config struct {
	RevisionControllerType string
	PollingInterval        time.Duration
	imageConfig
}

type imageConfig struct {
	image      string
	insecure   bool
	credential *v1.LocalObjectReference
}

func NewRevisionController(c client.Client, fn *openfunction.Function, revisionControllerType string, config map[string]string) (revisioncontroller.RevisionController, error) {
	r := &RevisionController{
		Client: c,
		log:    ctrl.Log.WithName("RevisionController").WithValues("Function", fn.Namespace+"/"+fn.Name, "Type", revisionControllerType),
		fn:     fn,
		stopCh: make(chan os.Signal),
	}
	signal.Notify(r.stopCh, os.Interrupt, syscall.SIGTERM)

	var err error
	r.config, err = r.getRevisionControllerConfig(config)
	if err != nil {
		return nil, err
	}

	r.keychain, err = r.getKeychain(r.config)
	if err != nil {
		return nil, err
	}

	return r, err
}

func (r *RevisionController) Start() {
	go func() {
		compare := func() {
			digest, err := r.getLatestImageDigest()
			if err != nil {
				r.log.Error(err, "get image digest error")
				return
			}

			currentDigest, err := r.getCurrentImageDigest()
			if currentDigest == digest {
				r.log.V(1).Info("image has no change")
				return
			}

			if err := r.updateFunctionStatus(digest); err != nil {
				r.log.Error(err, "update function status error")
				return
			}

			return
		}

		for {
			select {
			case <-r.stopCh:
				r.log.Info("revision controller stopped")
				return
			default:
			}

			compare()
			time.Sleep(r.config.PollingInterval)
		}
	}()

	r.log.Info("revision controller started")
}

func (r *RevisionController) Update(config map[string]string) error {
	revisionControllerConfig, err := r.getRevisionControllerConfig(config)
	if err != nil {
		return err
	}

	r.keychain, err = r.getKeychain(revisionControllerConfig)
	if err != nil {
		return err
	}

	r.config = revisionControllerConfig
	return nil
}

func (r *RevisionController) Stop() {
	close(r.stopCh)
	signal.Stop(r.stopCh)
}

func (r *RevisionController) getRevisionControllerConfig(config map[string]string) (*Config, error) {
	function, err := r.getFunction()
	if err != nil {
		return nil, err
	}

	interval := constants.DefaultPollingInterval
	str := config[constants.PollingInterval]
	if str != "" {
		var err error
		interval, err = time.ParseDuration(str)
		if err != nil {
			return nil, err
		}
	}

	insecure := true
	insecureStr := config[constants.InsecureRegistry]
	if insecureStr != "true" {
		insecure = false
	}

	revisionControllerConfig := &Config{
		RevisionControllerType: config[constants.RevisionControllerType],
		PollingInterval:        interval,
		imageConfig: imageConfig{
			image:      function.Spec.Image,
			insecure:   insecure,
			credential: function.Spec.ImageCredentials,
		},
	}

	return revisionControllerConfig, nil
}

func (r *RevisionController) getKeychain(revisionControllerConfig *Config) (authn.Keychain, error) {
	if revisionControllerConfig.credential == nil {
		return nil, fmt.Errorf("image credential must be specified")
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      revisionControllerConfig.credential.Name,
			Namespace: r.fn.Namespace,
		},
	}

	if err := r.Get(context.Background(), client.ObjectKeyFromObject(&secret), &secret); err != nil {
		return nil, err
	}

	return k8schain.NewFromPullSecrets(context.Background(), []v1.Secret{secret})
}

func (r *RevisionController) getLatestImageDigest() (string, error) {
	var auth authn.Authenticator
	opts := []name.Option{name.WeakValidation}
	if r.config.insecure {
		opts = append(opts, name.Insecure)
	}
	ref, err := name.ParseReference(r.config.image, opts...)
	if err != nil {
		return "", err
	}

	auth, err = r.keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return "", err
	}

	descriptor, err := remote.Head(ref, remote.WithAuth(auth))
	if err != nil {
		return "", err
	}

	return descriptor.Digest.String(), nil
}

func (r *RevisionController) getCurrentImageDigest() (string, error) {
	function, err := r.getFunction()
	if err != nil {
		return "", err
	}

	if function.Status.Revision == nil {
		return "", nil
	}

	return function.Status.Revision.ImageDigest, nil
}

func (r *RevisionController) updateFunctionStatus(digest string) error {
	function, err := r.getFunction()
	if err != nil {
		return err
	}

	switch r.config.RevisionControllerType {
	case constants.RevisionControllerTypeImage:
		if function.Status.Serving == nil {
			function.Status.Serving = &openfunction.Condition{}
		}
		function.Status.Serving.State = ""
		function.Status.Serving.ResourceHash = ""
		function.Status.Revision = &openfunction.Revision{ImageDigest: digest}
		r.log.Info("image changed, rerun serving")
	case constants.RevisionControllerTypeSourceImage:
		function.Status.Build = nil
		r.log.Info("source image changed, rebuild function")
	}

	return r.Status().Update(context.Background(), function)
}

func (r *RevisionController) getFunction() (*openfunction.Function, error) {
	fn := &openfunction.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.fn.Name,
			Namespace: r.fn.Namespace,
		},
	}

	if err := r.Get(context.Background(), client.ObjectKeyFromObject(fn), fn); err != nil {
		return nil, err
	}

	return fn, nil
}

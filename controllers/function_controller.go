/*
Copyright 2022 The OpenFunction Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	openfunction "github.com/openfunction/apis/core/v1beta1"
	"github.com/openfunction/pkg/util"
	"github.com/openfunction/revision-controller/pkg/constants"
	revisioncontroller "github.com/openfunction/revision-controller/pkg/revision-controller"
	"github.com/openfunction/revision-controller/pkg/revision-controller/git"
	"github.com/openfunction/revision-controller/pkg/revision-controller/image"
	"github.com/openfunction/revision-controller/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	revisionControllerKey       = "openfunction.io/revision-controller"
	revisionControllerParamsKey = "openfunction.io/revision-controller-params"
)

var (
	commitShaRegEx = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// FunctionReconciler reconciles a Function object
type FunctionReconciler struct {
	client.Client
	log logr.Logger

	revisionControllers map[string]revisioncontroller.RevisionController
}

func NewFunctionReconciler(mgr manager.Manager) *FunctionReconciler {
	r := &FunctionReconciler{
		Client:              mgr.GetClient(),
		log:                 ctrl.Log.WithName("controllers").WithName("Function"),
		revisionControllers: make(map[string]revisioncontroller.RevisionController),
	}

	return r
}

//+kubebuilder:rbac:groups=core.openfunction.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openfunction.io,resources=functions/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Function object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("Function", req.NamespacedName)

	fn := &openfunction.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if err := r.Get(ctx, req.NamespacedName, fn); err != nil {
		if util.IsNotFound(err) {
			log.V(1).Info("Function deleted")
			r.cleanRevisionControllerByFunction(fn)
		}

		return ctrl.Result{}, util.IgnoreNotFound(err)
	}

	if fn.Annotations == nil ||
		fn.Annotations[revisionControllerKey] != "enable" {
		r.cleanRevisionControllerByFunction(fn)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, r.addRevisionController(fn)
}

func (r *FunctionReconciler) addRevisionController(fn *openfunction.Function) error {
	config, err := getRevisionControllerConfig(fn.Annotations[revisionControllerParamsKey])
	if err != nil {
		return err
	}

	revisionControllerType := config[constants.RevisionControllerType]
	r.cleanRevisionControllerByFunction(fn, revisionControllerType)

	switch revisionControllerType {
	case constants.RevisionControllerTypeSource:
		if fn.Spec.Build == nil {
			r.deleteRevisionController(fn, revisionControllerType)
			return nil
		}

		if fn.Spec.Build.SrcRepo.Revision != nil {
			if commitShaRegEx.MatchString(*fn.Spec.Build.SrcRepo.Revision) {
				r.log.V(1).Info("source code point to a commit, no need to start revision controller")
				r.deleteRevisionController(fn, revisionControllerType)
				return nil
			}
		}
	case constants.RevisionControllerTypeImage:
		if fn.Spec.Serving == nil {
			r.deleteRevisionController(fn, revisionControllerType)
			return nil
		}
	default:
		return fmt.Errorf("unspport revision controller type, %s", revisionControllerType)
	}

	key := strings.Join([]string{fn.Namespace, fn.Name, revisionControllerType}, "/")
	rc := r.revisionControllers[key]
	if rc != nil {
		return rc.Update(config)
	}

	rc, err = newRevisionController(r.Client, fn, revisionControllerType, config)
	if err != nil {
		return err
	}

	rc.Start()
	r.revisionControllers[key] = rc
	return nil
}

func (r *FunctionReconciler) cleanRevisionControllerByFunction(fn *openfunction.Function, ignored ...string) {
	toBeDeleted := map[string]bool{
		constants.RevisionControllerTypeSource:      true,
		constants.RevisionControllerTypeSourceImage: true,
		constants.RevisionControllerTypeImage:       true,
	}
	for k := range toBeDeleted {
		if !utils.StringInList(k, ignored) {
			r.deleteRevisionController(fn, k)
		}
	}
}

func (r *FunctionReconciler) deleteRevisionController(fn *openfunction.Function, revisionControllerType string) {
	key := strings.Join([]string{fn.Namespace, fn.Name, revisionControllerType}, "/")
	if rc, ok := r.revisionControllers[key]; ok {
		rc.Stop()
		delete(r.revisionControllers, key)
	}
}

func getRevisionControllerConfig(params string) (map[string]string, error) {
	config := make(map[string]string)
	if err := utils.YamlUnmarshal([]byte(params), config); err != nil {
		return nil, err
	}

	if config[constants.RevisionControllerType] == "" {
		config[constants.RevisionControllerType] = constants.RevisionControllerTypeSource
	}

	return config, nil
}

func newRevisionController(c client.Client, fn *openfunction.Function, revisionControllerType string, config map[string]string) (revisioncontroller.RevisionController, error) {
	switch revisionControllerType {
	case constants.RevisionControllerTypeSource:
		return git.NewRevisionController(c, fn, revisionControllerType, config)
	case constants.RevisionControllerTypeImage:
		return image.NewRevisionController(c, fn, revisionControllerType, config)
	default:
		return nil, fmt.Errorf("unspported revision controller type, %s", revisionControllerType)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openfunction.Function{}).
		Complete(r)
}

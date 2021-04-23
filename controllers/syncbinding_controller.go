/*
Copyright 2021.

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
	"github.com/go-logr/logr"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	syncerv1 "github.com/arikkfir/syncer/api/v1"
)

const (
	looperFinalizerName = "looper.finalizers." + syncerv1.Group
)

// SyncBindingReconciler reconciles a SyncBinding object
type SyncBindingReconciler struct {
	client        client.Client
	dynamicClient dynamic.Interface
	log           logr.Logger
	loops         map[string]*looper
}

//+kubebuilder:rbac:groups=syncer.k8s.kfirs.com,resources=syncbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncer.k8s.kfirs.com,resources=syncbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncer.k8s.kfirs.com,resources=syncbindings/finalizers,verbs=update

// Reconcile is the reconciliation loop implementation aiming to continuously
// move the current state of the cluster closer to the desired state, which in
// the SyncBinding controller's view means ensure a reconciliation loop is running
// for each binding.
// TODO: support status
func (r *SyncBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.log.WithValues("syncbinding", req.NamespacedName)

	binding := &syncerv1.SyncBinding{}
	if err := r.client.Get(ctx, req.NamespacedName, binding); err != nil {
		logger.V(1).Info("Failed fetching binding resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If object is being deleted, perform finalization (if haven't already)
	if !binding.ObjectMeta.DeletionTimestamp.IsZero() {
		if containsString(binding.ObjectMeta.Finalizers, looperFinalizerName) {

			// Stop sync loop
			if err := r.removeLooperFor(binding); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed stopping sync loop: %w", err)
			}

			// Remove our finalizer
			binding.ObjectMeta.Finalizers = removeString(binding.ObjectMeta.Finalizers, looperFinalizerName)
			if err := r.client.Update(context.Background(), binding); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed removing finalizer: %w", err)
			}

		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Object is not being deleted! but ensure our finalizer is listed
	if !containsString(binding.ObjectMeta.Finalizers, looperFinalizerName) {
		binding.ObjectMeta.Finalizers = append(binding.ObjectMeta.Finalizers, looperFinalizerName)
		if err := r.client.Update(context.Background(), binding); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed adding finalizer: %w", err)
		}
	}

	// Ensure a sync loop exists for this binding
	if err := r.ensureLooperFor(binding); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update sync loop: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyncBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.loops = make(map[string]*looper)
	r.client = mgr.GetClient()
	r.dynamicClient = dynamic.NewForConfigOrDie(mgr.GetConfig())
	r.log = ctrl.Log.WithName("controllers").WithName("SyncBinding")
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncerv1.SyncBinding{}).
		Complete(r)
}

// Register creates or updates the looper associated with the given binding.
// TODO: ensure thread-safety
func (r *SyncBindingReconciler) ensureLooperFor(binding *syncerv1.SyncBinding) error {
	key := binding.Namespace + "/" + binding.Name
	l, ok := r.loops[key]
	if ok {
		err := l.stop()
		if err != nil {
			return fmt.Errorf("failed updating binding: %w", err)
		}
		l.binding = binding
		err = l.start()
		if err != nil {
			return fmt.Errorf("failed updating binding: %w", err)
		}
		return nil
	} else {
		r.loops[key] = &looper{
			log:           r.log.WithValues("syncbinding", l.binding.Namespace+"/"+l.binding.Name),
			binding:       binding,
			dynamicClient: r.dynamicClient,
		}
		err := r.loops[key].start()
		if err != nil {
			return fmt.Errorf("failed creating sync loop: %w", err)
		}
		return nil
	}
}

// Unregister stops & removes the looper associated with the given binding.
// TODO: ensure thread-safety
func (r *SyncBindingReconciler) removeLooperFor(binding *syncerv1.SyncBinding) error {
	key := binding.Namespace + "/" + binding.Name
	looper, ok := r.loops[key]
	if ok {
		err := looper.stop()
		if err != nil {
			return fmt.Errorf("failed stopping binding reconciliation loop: %w", err)
		}
	}
	return nil
}

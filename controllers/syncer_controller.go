package controllers

import (
	"context"
	"fmt"
	"github.com/Jeffail/gabs/v2"
	syncerv1 "github.com/arikkfir/syncer/api/v1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncerReconciler reconciles a Syncer object
type SyncerReconciler struct {
	client.Client
	DynamicClient dynamic.Interface
	Log           logr.Logger
	ClientSet     *kubernetes.Clientset
}

//+kubebuilder:rbac:groups=syncer.k8s.kfirs.com,resources=syncers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncer.k8s.kfirs.com,resources=syncers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncer.k8s.kfirs.com,resources=syncers/finalizers,verbs=update

func (r *SyncerReconciler) findReferent(ctx context.Context, defaultNamespace string, ref syncerv1.Referent) (string, *schema.GroupVersionResource, *unstructured.Unstructured, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	groupVersion, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return namespace, nil, nil, fmt.Errorf("failed to parse API version '%s': %w", ref.APIVersion, err)
	}
	gvr := &schema.GroupVersionResource{
		Group:    groupVersion.Group,
		Version:  groupVersion.Version,
		Resource: ref.Kind,
	}
	obj, err := r.DynamicClient.Resource(*gvr).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return namespace, gvr, nil, err
	}
	return namespace, gvr, obj, nil
}

func (r *SyncerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("syncer", req.NamespacedName)

	// Fetch the Syncer instance
	syncer := &syncerv1.Syncer{}
	err := r.Get(ctx, req.NamespacedName, syncer)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.V(1).Info("Syncer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get Syncer '%v': %w", syncer, err)
	}

	// Fetch source object & data
	_, _, sourceObj, err := r.findReferent(ctx, syncer.Namespace, syncer.Spec.Source)
	if err != nil {
		if errors.IsNotFound(err) {
			log.V(2).Error(err, "Failed fetching source referent", "ref", syncer.Spec.Source)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed fetching source referent '%v': %w", syncer.Spec.Source, err)
	}
	sourcePtr, err := gabs.Wrap(sourceObj.Object).JSONPointer(syncer.Spec.Source.Property)
	if err != nil {
		log.Error(err, "Failed accessing source property", "ref", syncer.Spec.Source)
		return ctrl.Result{}, nil
	}
	data := sourcePtr.Data()

	// Update target object with value from source
	targetNamespace, targetGVR, _, err := r.findReferent(ctx, syncer.Namespace, syncer.Spec.Target)
	patch := gabs.New()
	_, err = patch.SetJSONPointer(data, syncer.Spec.Target.Property)
	if err != nil {
		log.Error(err, "Failed creating target patch", "property", syncer.Spec.Target.Property, "data", data)
		return ctrl.Result{}, nil
	}
	patchJSONBytes := patch.Bytes()

	_, err = r.DynamicClient.Resource(*targetGVR).Namespace(targetNamespace).Patch(
		ctx,
		syncer.Spec.Target.Name,
		types.StrategicMergePatchType,
		patchJSONBytes,
		metav1.PatchOptions{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed updating target '%v': %w", syncer.Spec.Target, err)
	}

	log.V(2).Info("Updated target",
		"source", syncer.Spec.Source,
		"target", syncer.Spec.Target,
		"data", data)
	return ctrl.Result{}, nil
}

//func (r *SyncerReconciler) test() {
//	factory := informers.NewSharedInformerFactory(r.ClientSet, 0)
//	informer := factory.InformerFor()
//	stopper := make(chan struct{})
//	defer close(stopper)
//	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
//		AddFunc: func(obj interface{}) {
//			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
//			// interface that allows us to get metadata easily
//			mObj := obj.(v1.Object)
//			log.Printf("New Pod Added to Store: %s", mObj.GetName())
//		},
//	})
//	informer.Run(stopper)
//}

// SetupWithManager sets up the controller with the Manager.
func (r *SyncerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncerv1.Syncer{}).
		Complete(r)
}

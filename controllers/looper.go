package controllers

import (
	"context"
	"fmt"
	"github.com/Jeffail/gabs/v2"
	syncerv1 "github.com/arikkfir/syncer/api/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"reflect"
	"time"
)

type looper struct {
	log           logr.Logger
	binding       *syncerv1.SyncBinding
	dynamicClient dynamic.Interface
	stopChannel   chan struct{}
}

func (l *looper) getReferent(ctx context.Context, ref syncerv1.Referent) (*schema.GroupVersionResource, *unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse API version '%s': %w", ref.APIVersion, err)
	}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = l.binding.Namespace
	}
	gvr := &schema.GroupVersionResource{
		Group:    groupVersion.Group,
		Version:  groupVersion.Version,
		Resource: ref.Kind,
	}
	obj, err := l.dynamicClient.
		Resource(*gvr).
		Namespace(namespace).
		Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return gvr, obj, nil
}

func (l *looper) sync(ctx context.Context) {
	_, src, err := l.getReferent(ctx, l.binding.Spec.Source)
	if err != nil {
		l.log.V(1).Error(err, "Failed looking up source referent", "source", l.binding.Spec.Source)
		return
	} else if src == nil {
		l.log.V(2).Error(err, "Source referent not found", "source", l.binding.Spec.Source)
		return
	}
	srcPtr, err := gabs.Wrap(src.Object).JSONPointer(l.binding.Spec.Source.Property)
	if err != nil {
		l.log.V(1).Error(err, "Failed accessing property", "ref", l.binding.Spec.Source)
		return
	}
	srcData := srcPtr.Data()

	dstGVR, dst, err := l.getReferent(ctx, l.binding.Spec.Target)
	if err != nil {
		l.log.V(1).Error(err, "Failed looking up target referent", "target", l.binding.Spec.Target)
		return
	} else if dst == nil {
		l.log.V(2).Error(err, "Target referent not found", "target", l.binding.Spec.Target)
		return
	}
	dstPtr, err := gabs.Wrap(dst.Object).JSONPointer(l.binding.Spec.Target.Property)
	if err != nil {
		l.log.V(1).Error(err, "Failed accessing property", "ref", l.binding.Spec.Target)
		return
	}
	dstData := dstPtr.Data()

	if reflect.DeepEqual(srcData, dstData) {
		l.log.V(3).Info("Target referent is synced")
		return
	}

	patch := gabs.New()
	_, err = patch.SetJSONPointer(srcData, l.binding.Spec.Target.Property)
	if err != nil {
		l.log.V(1).Error(err, "Failed creating target patch")
		return
	}
	patchJSONBytes := patch.Bytes()

	_, err = l.dynamicClient.Resource(*dstGVR).Namespace(dst.GetNamespace()).Patch(
		ctx,
		l.binding.Spec.Target.Name,
		types.StrategicMergePatchType,
		patchJSONBytes,
		metav1.PatchOptions{})
	if err != nil {
		l.log.V(1).Error(err, "Failed updating target", "ref", l.binding.Spec.Target)
		return
	}

	l.log.V(3).Info("Updated target",
		"source", l.binding.Spec.Source,
		"target", l.binding.Spec.Target,
		"data", srcData)
}

func (l *looper) start() error {
	if l.stopChannel != nil {
		return nil
	}

	interval, err := time.ParseDuration(l.binding.Spec.Interval)
	if err != nil {
		return fmt.Errorf("invalid duration '%s': %w", l.binding.Spec.Interval, err)
	}

	l.stopChannel = make(chan struct{})
	ticker := time.NewTicker(interval)
	go func(done chan struct{}, ticket *time.Ticker) {
		l.log.V(1).Info("Starting sync loop")
		defer ticker.Stop()
		for {
			select {
			case <-done:
				l.log.V(1).Info("Stopping sync loop")
				return
			case _ = <-ticker.C:
				ctx := context.TODO()
				l.sync(ctx)
			}
		}
	}(l.stopChannel, ticker)
	return nil
}

func (l *looper) stop() error {
	if l.stopChannel != nil {
		l.stopChannel <- struct{}{}
	}
	return nil
}

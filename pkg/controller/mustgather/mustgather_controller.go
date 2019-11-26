package mustgather

import (
	"context"
	"text/template"

	redhatcopv1alpha1 "github.com/redhat-cop/must-gather-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/scylladb/go-set/strset"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "mustgather-controller"

// generate this by running: go-bindata -o pkg/controller/mustgather/template.go -pkg mustgather templates/
const templateAssetName = "templates/job.template.yaml"

var log = logf.Log.WithName(controllerName)

const defaultMustGatherImage = "quay.io/openshift/origin-must-gather:latest"

var jobTemplate *template.Template

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MustGather Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	var err error
	jobTemplate, err = initializeTemplate()
	if err != nil {
		log.Error(err, "unable to initialize job template")
		return err
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMustGather{ReconcilerBase: util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName))}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MustGather
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.MustGather{}}, &handler.EnqueueRequestForObject{}, util.ResourceGenerationOrFinalizerChangedPredicate{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MustGather
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &redhatcopv1alpha1.MustGather{},
	})
	if err != nil {
		return err
	}

	return nil
}

func initializeTemplate() (*template.Template, error) {
	text, err := Asset(templateAssetName)
	if err != nil {
		log.Error(err, "unable to exctract asset", "name", templateAssetName)
		return &template.Template{}, err
	}
	jobTemplate, err := template.New("MustGatherJob").Parse(string(text))
	if err != nil {
		log.Error(err, "Error parsing template", "template", text)
		return &template.Template{}, err
	}
	return jobTemplate, err
}

// blank assignment to verify that ReconcileMustGather implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMustGather{}

// ReconcileMustGather reconciles a MustGather object
type ReconcileMustGather struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	util.ReconcilerBase
}

// Reconcile reads that state of the cluster for a MustGather object and makes changes based on the state read
// and what is in the MustGather.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMustGather) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MustGather")

	// Fetch the MustGather instance
	instance := &redhatcopv1alpha1.MustGather{}
	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if ok, err := r.IsValid(instance); !ok {
		return r.ManageError(instance, err)
	}

	if !r.IsInitialized(instance) {
		err := r.GetClient().Update(context.TODO(), instance)
		if err != nil {
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(instance, err)
		}
		return reconcile.Result{}, nil
	}

	job, err := r.getJobFromInstance(instance)
	if err != nil {
		log.Error(err, "unable to get job from", "instance", instance)
		return r.ManageError(instance, err)
	}

	job1 := &batchv1.Job{}
	err = r.GetClient().Get(context.TODO(), types.NamespacedName{
		Name:      job.GetName(),
		Namespace: job.GetNamespace(),
	}, job1)

	if err != nil {
		if errors.IsNotFound(err) {
			// job is not there, create it.
			err = r.GetClient().Create(context.TODO(), job, &client.CreateOptions{})
			if err != nil {
				log.Error(err, "unable to create", "job", job)
				return r.ManageError(instance, err)
			}
			return r.ManageSuccess(instance)
		}
		// Error reading the object - requeue the request.
		log.Error(err, "unable to look up", "job", types.NamespacedName{
			Name:      job.GetName(),
			Namespace: job.GetNamespace(),
		})
		return r.ManageError(instance, err)
	}

	// if we get here it means that either
	// 1. the mustgather instance was updated, which we don't support and we are going to ignore
	// 2. the job was updated, probably the status piece. we should the update the status of the instance, not supported yet.

	return r.ManageSuccess(instance)
}

func (r *ReconcileMustGather) IsInitialized(instance *redhatcopv1alpha1.MustGather) bool {
	initialized := true
	imageSet := strset.New(instance.Spec.MustGatherImages...)
	if !imageSet.Has(defaultMustGatherImage) {
		imageSet.Add(defaultMustGatherImage)
		instance.Spec.MustGatherImages = imageSet.List()
		initialized = false
	}
	if instance.Spec.ServiceAccountRef.Name == "" {
		instance.Spec.ServiceAccountRef.Name = "default"
		initialized = false
	}
	return initialized
}

func (r *ReconcileMustGather) getJobFromInstance(instance *redhatcopv1alpha1.MustGather) (*unstructured.Unstructured, error) {
	unstructuredJob, err := util.ProcessTemplate(instance, jobTemplate)
	if err != nil {
		log.Error(err, "unable to process", "template", jobTemplate, "with parameter", instance)
		return &unstructured.Unstructured{}, err
	}
	return unstructuredJob, nil
}

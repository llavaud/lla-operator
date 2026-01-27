/*
Copyright 2026.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mygroupv1alpha1 "github.com/llavaud/lla-operator/api/v1alpha1"
)

// LlaReconciler reconciles a Lla object
type LlaReconciler struct {
	// LLA client Kubernetes pour lire/écrire dans Kubernetes
	client.Client
	/*
		LLA
		le Scheme est un registre qui connait tous les types Go et leur correspondance avec les types Kubernetes
		permet la conversion des objets Go <-> YAML/Json
	*/
	Scheme *runtime.Scheme
}

/*
LLA
annotation "magique" permettant de générer automatiquement
le ClusterRole avec un "make manifests"
*/
// +kubebuilder:rbac:groups=mygroup.example.com,resources=lla,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mygroup.example.com,resources=lla/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mygroup.example.com,resources=lla/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Lla object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
/*
LLA
Fonction de reconciliation
ctx est le Context avec timeout et logger
req contient le NamespacedName de l'objet à reconciler (req.NamespacedName.Name)
*/
func (r *LlaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// TODO(user): your logic here

	/*
		LLA
		retour possible:
		ctrl.Result{}, nil	Succès, terminé
		ctrl.Result{Requeue: true}, nil	Requeue immédiat
		ctrl.Result{RequeueAfter: 5*time.Minute}, nil	Requeue dans 5 min
		ctrl.Result{}, err	Erreur → requeue avec backoff exponentiel
	*/
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
/*
LLA
Fonction pour configurer ce que le controller doit observer
*/
func (r *LlaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mygroupv1alpha1.Lla{}). // LLA watch ma CR Lla
		Named("lla").                // LLA nom du controller pour les logs/metrics
		Complete(r)                  // LLA enregistre le reconciler
}

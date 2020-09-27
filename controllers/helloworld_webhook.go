/*


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

package v1

import (
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var helloworldlog = logf.Log.WithName("helloworld-resource")

func (r *Helloworld) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-cache-example-com-v1alpha1-helloworld,mutating=true,failurePolicy=fail,groups=cache.example.com,resources=helloworlds,verbs=create;update,versions=v1alpha1,name=mhelloworld.kb.io

var _ webhook.Defaulter = &Helloworld{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Helloworld) Default() {
	helloworldlog.Info("default", "name", r.Name)

	if r.Spec.Size == 0 {
		r.Spec.Size = 3
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-cache-example-com-v1alpha1-helloworld,mutating=false,failurePolicy=fail,groups=cache.example.com,resources=helloworlds,versions=v1alpha1,name=vhelloworld.kb.io

var _ webhook.Validator = &Helloworld{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Helloworld) ValidateCreate() error {
	helloworldlog.Info("validate create", "name", r.Name)

	return validateOdd(r.Spec.Size)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Helloworld) ValidateUpdate(old runtime.Object) error {
	helloworldlog.Info("validate update", "name", r.Name)

	return validateOdd(r.Spec.Size)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Helloworld) ValidateDelete() error {
	helloworldlog.Info("validate delete", "name", r.Name)

	return nil
}
func validateOdd(n int32) error {
	if n%2 == 0 {
		return errors.New("Cluster size must be an odd number")
	}
	return nil
}

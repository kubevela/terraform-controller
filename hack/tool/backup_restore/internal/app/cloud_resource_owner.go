/*
Copyright 2022 The KubeVela Authors.

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

package app

import (
	"context"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/controllers/configuration/backend"
)

// CloudResourceOwner is an object which create and manage cloud resources.
// It may be a Configuration or an Application Component
type CloudResourceOwner interface {
	GetConfigurationNamespacedName() *crossplane.Reference
	GetK8SBackend() (*backend.K8SBackend, error)
	Apply(ctx context.Context) error
}

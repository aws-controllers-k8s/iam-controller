// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package role_permissions_boundary

import (
	"context"
)

// customFindPermissionsBoundary returns the input resource as-is.
// The permissions boundary value is already populated on the parent Role
// from the GetRole API response, so no separate read call is needed.
func (rm *resourceManager) customFindPermissionsBoundary(
	ctx context.Context,
	r *resource,
) (*resource, error) {
	return r, nil
}

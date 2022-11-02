// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	ts "github.com/k0sproject/k0s/inttest/toolsuite"
	tsutil "github.com/k0sproject/k0s/inttest/toolsuite/util"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"

	"sigs.k8s.io/yaml"

	"github.com/hashicorp/terraform-exec/tfexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PlanBuilder func(output tsutil.TerraformOutputMap) (*apv1beta2.Plan, error)

// PlanApplyOperation is an Operation that will read output values from terraform,
// build a user-provided Plan, and apply it via kubectl.
func PlanApplyOperation(builder PlanBuilder) ts.ClusterOperation {
	return func(ctx context.Context, data ts.ClusterData) error {
		// The the terraform binary, error if missing
		tfpath, err := exec.LookPath("terraform")
		if err != nil {
			return fmt.Errorf("unable to find terraform binary: %w", err)
		}

		// Initialize terraform to read the state output values.
		tf, err := tfexec.NewTerraform(data.DataDir, tfpath)
		if err != nil {
			return fmt.Errorf("failed to initialize terraform: %w", err)
		}

		tf.SetStdout(os.Stdout)

		// Collect the terraform output which the specific builders will use for collecting
		// whatever information they need.
		tfout, err := tf.Output(ctx)
		if err != nil {
			return fmt.Errorf("unable to collect terraform output: %w", err)
		}

		// Build the plan
		plan, err := builder(tfout)
		if err != nil {
			return fmt.Errorf("failed to build plan: %w", err)
		}

		// Save + apply the plan
		planFile := path.Join(data.DataDir, "plan.yaml")

		if err := savePlan(planFile, plan); err != nil {
			return fmt.Errorf("failed to save plan to '%s': %w", planFile, err)
		}

		if err := applyPlan(data, planFile); err != nil {
			return fmt.Errorf("failed to apply plan: %w", err)
		}

		return nil
	}
}

// savePlan saves a marshalled Plan to the data directory, without status.
func savePlan(planFile string, plan *apv1beta2.Plan) error {
	noStatusPlan := struct {
		metav1.TypeMeta `json:",omitempty,inline"`
		ObjectMeta      metav1.ObjectMeta     `json:"metadata,omitempty"`
		Spec            apv1beta2.PlanSpec    `json:"spec"`
		Status          *apv1beta2.PlanStatus `json:"status,omitempty"`
	}{
		plan.TypeMeta,
		plan.ObjectMeta,
		plan.Spec,
		nil,
	}

	data, err := yaml.Marshal(&noStatusPlan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan for save: %w", err)
	}

	return os.WriteFile(planFile, data, 0644)
}

// applyPlan uses 'kubectl' to apply a plan using the kubeconfig available
// in the data directory.
func applyPlan(clusterData ts.ClusterData, planFile string) error {
	cmd := exec.Command("kubectl", "--kubeconfig", clusterData.KubeConfigFile, "apply", "-f", planFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute kubectl: %w", err)
	}

	return nil
}

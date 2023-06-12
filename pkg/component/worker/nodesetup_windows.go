/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the &quot;License&quot;);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an &quot;AS IS&quot; BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/sirupsen/logrus"
)

// TODO: this all should be dropped in favor of init container for calico stack
type NodesetupHelper struct {
}

var _ manager.Component = (*NodesetupHelper)(nil)

func (c NodesetupHelper) Init(_ context.Context) error {
	for _, path := range []string{
		"C:\\opt\\cni\\bin",
		"C:\\opt\\cni\\conf",
	} {
		if err := dir.Init(path, 777); err != nil {
			if os.IsExist(err) {
				logrus.Warn("CalicoWindows already set up")
			} else {
				return fmt.Errorf("can't create CalicoWindows dir: %v", err)
			}
		}
	}
	return winExecute(createFirewallRules)
}

func (c NodesetupHelper) Start(_ context.Context) error {
	return nil
}

func (c NodesetupHelper) Stop() error {
	return nil
}

const createFirewallRules = `
$existingRule = Get-NetFirewallRule -DisplayName "kubectl exec 10250" -ErrorAction SilentlyContinue; if ($existingRule -eq $null) { New-NetFirewallRule -Name "KubectlExec10250" -Description "Enable kubectl exec and log" -Action Allow -LocalPort 10250 -Enabled True -DisplayName "kubectl exec 10250" -Protocol TCP -ErrorAction SilentlyContinue } else { Write-Output "The firewall rule already exists." }
`

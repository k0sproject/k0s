/*
Copyright 2021 k0s authors

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

package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type CLITestSuite struct {
	suite.Suite
}

func (s *CLITestSuite) TestKubeConfigCreate() {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  api:
    externalAddress: 10.0.0.86
`
	configGetter := testutil.NewConfigGetter(s.T(), yamlData, false, config.DefaultCfgVars())
	cfg := configGetter.FakeConfigFromFile()

	caCert := `
-----BEGIN CERTIFICATE-----
MIIDADCCAeigAwIBAgIUW+2hawM8HgHrfxmDRV51wOq95icwDQYJKoZIhvcNAQEL
BQAwGDEWMBQGA1UEAxMNa3ViZXJuZXRlcy1jYTAeFw0yMTEwMjExMjAxMDBaFw0z
MTEwMTkxMjAxMDBaMBgxFjAUBgNVBAMTDWt1YmVybmV0ZXMtY2EwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDQqkq9cu49/The1CUQSqFNeGaNNblnHYZo
CFcrJYtuimTPc7Abs9vIp6Ax5wqtqGTYzdg0hZc4dKXFDpvVzn8yU17IUpfDY7Ix
j2q8wBDI7bJCJw5Mw8/lcAqI1ub+DEYrdg6sRvCcByCK9qPlvuabc6YAbB0mmES6
rqCXE/Xr8byW9QYPwD+p6wKZoRXm9WlSwCvFT9OCk2OT8G5o+9RagHhmYgsg9vHf
LrDoPUtu/W5zE+fmIaAHGoWoo9yaBGsavPRkFzjbPI7mK1Zci6phnP/YWtLpyIx1
n+evNhdZj7UE9CMrzyhUU45vriK/Arc7co/WAk6pqO82tWsj38zJAgMBAAGjQjBA
MA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQ6hhbj
kcCmAK0BSTI5bU+hkwO00zANBgkqhkiG9w0BAQsFAAOCAQEAITD7bXxrpbS2kHl4
4Z3MKo59+KXHo9ut4a7L+oGwKX694g0/BrEAGHXRZrF5hEY0q8R0g3TdlHax0A6t
jpKePa+9ifNE+34gCz07xvAclcljk87zUM1mYYu1kgSc0XWeHnzMVXalo+gWzTBL
q8mVPQ4v+nk+MVP06r7GA42GsqTZGhH1xDQF0GLa25UHw4pzEX1olwaBWybl7Wql
K3icRdyke+TCLl+YqsCKG2n95cK4CMMEm8a1KVWRZKwDqLD7rFdemNdmzCNlpFW/
1uC/IeGA0XwM6CLsS7VAe0wbgxbgw0vLvkAnAEl6+VAOqr2ux3js1BbQ5g0d/x1L
nzXu8A==
-----END CERTIFICATE-----
`

	tmpDir := s.T().TempDir()
	caCertPath := filepath.Join(tmpDir, "ca-cert")
	s.Require().NoError(os.WriteFile(caCertPath, []byte(caCert), 0644))

	caCertKey := `
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0KpKvXLuPf04XtQlEEqhTXhmjTW5Zx2GaAhXKyWLbopkz3Ow
G7PbyKegMecKrahk2M3YNIWXOHSlxQ6b1c5/MlNeyFKXw2OyMY9qvMAQyO2yQicO
TMPP5XAKiNbm/gxGK3YOrEbwnAcgivaj5b7mm3OmAGwdJphEuq6glxP16/G8lvUG
D8A/qesCmaEV5vVpUsArxU/TgpNjk/BuaPvUWoB4ZmILIPbx3y6w6D1Lbv1ucxPn
5iGgBxqFqKPcmgRrGrz0ZBc42zyO5itWXIuqYZz/2FrS6ciMdZ/nrzYXWY+1BPQj
K88oVFOOb64ivwK3O3KP1gJOqajvNrVrI9/MyQIDAQABAoIBAQCMFieDNIuZdkzH
7SjM3S2ZcwF2P+EuxvWbFi5fOx92oNa5J3PNxVwCQ/caSYAzwd+iZd+Gs0Eol7dK
qloYmj9uq+XwGvLkLCRPfXctLMyX+Gw6WToSc0s5P5Ty9UOyvs7FEscbBa03Mtm4
MYkrDpSHPIbvtaWEaamKov4RL0dklHh/HQPXOujCPGiwbqHMIIQ0+sQwEDCVcC+Z
eHw4GCiC4GPgABRyhMO1CHdLxvU7xWUGTsPM4jzVmfWT5ZUx6mGi/v+weE600GU6
qSKt6fP4oygGISRA8ya9rtGQe2qQpeMlw0QhJWdRb2KAwbyQKCr1/f9uWTrhhwJ1
T1kezEUBAoGBAPxNdLjjh+UAQ9vF0UWDiTdC5wLi6YCbrcXvIcWfh7AXDinPOpKN
VxSx6cj9KQS/tWWJS8awDNZ+R/vItHJJsmyBBtY2yTQSa+LM44gRpuhdFdKhPvqN
thZVnRWbrY3xgR26Qb8F9iZy3qjkGYH8KCuBwd0a969HwVv8dqJjDhf5AoGBANO5
IAYdzy4TOkh870diAEjigVpBP/QHTRu4SXQgafqn0jPG2qRCEOpCBEjLQzCvzSf/
QMfIpcvT7xaxkSxil7mzt1X0qAjrrzpCsa6fWbiQqiFHrES10b0j4IT5bRRH4Nvz
lKXe4IKsRZ3Hmi7HRX51V1eBy7fQxRhXDy/R/69RAoGAdYebZPlQ96NM+RbIaqpg
hCadOGH9xhQ/OeIwiD/NVIEY7u8C6PwAYbqTHjaYIgcv+BGiA/dEs7J109tl+4tL
G3JrfeRdi+085pTtNRiL+NhL7yeAD/Vtqi/NkiBIE8Q5kmCOee7MAJMoF+LR4xRU
nhe++EG0uakicLhFh1W/XfkCgYBMEuyKxhM3PvlmKl3fjDsF9Tz9LQzJpgXyu9jI
vQzXX42LxRuygXqKcYYQkdhmmgRhJrokDthj0JbL1KmRBSv3MbfiTrJB4k1n5abq
U59tTa2Tn6kqVxoxl76IiQbEjr8gyPjUUKzixvuMobeorzktIwRrENweBAmNoVp3
mEECwQKBgQDCYi2EubaseSNu25UQY7ij1TsxPZpBvPlQFUtwmpz9MmBvqZcJYsco
z+5UodDFCnUsfprMjfTdY2Vk99PT4++SrJ5iTOn7xgKRrd1MPkBv7SXwnPtxCBAK
yJm2KSue0toWmkBFK8WMTjAvmAw3Z/qUhJRKoqCu3k6Mf8DNl6t+Uw==
-----END RSA PRIVATE KEY-----
`
	caKeyPath := filepath.Join(tmpDir, "ca-key")
	s.Require().NoError(os.WriteFile(caKeyPath, []byte(caCertKey), 0644))

	userReq := certificate.Request{
		Name:   "test-user",
		CN:     "test-user",
		O:      "groups",
		CACert: caCertPath,
		CAKey:  caKeyPath,
	}

	k0sVars, err := config.NewCfgVars(nil, s.T().TempDir())
	s.Require().NoError(err)

	certManager := certificate.Manager{
		K0sVars: k0sVars,
	}

	s.Require().NoError(os.MkdirAll(k0sVars.CertRootDir, 0755))

	userCert, err := certManager.EnsureCertificate(userReq, "root")
	s.Require().NoError(err)
	clusterAPIURL := cfg.Spec.API.APIAddressURL()

	data := struct {
		CACert     string
		ClientCert string
		ClientKey  string
		User       string
		JoinURL    string
	}{
		CACert:     base64.StdEncoding.EncodeToString([]byte(caCert)),
		ClientCert: base64.StdEncoding.EncodeToString([]byte(userCert.Cert)),
		ClientKey:  base64.StdEncoding.EncodeToString([]byte(userCert.Key)),
		User:       "test-user",
		JoinURL:    clusterAPIURL,
	}

	var buf bytes.Buffer
	s.Require().NoError(userKubeconfigTemplate.Execute(&buf, &data))

	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	s.Require().NoError(os.WriteFile(kubeconfigPath, buf.Bytes(), 0644))

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	s.Require().NoError(err)
	s.Equal("https://10.0.0.86:6443", config.Host)
}

func TestCLITestSuite(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}

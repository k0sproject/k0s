package install

import (
	"fmt"

	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

var (
	k0sServiceName = "k0s"
	k0sDescription = "k0s - Zero Friction Kubernetes"
)

type program struct{}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

// EnsureService installs the k0s service, per the given arguments, and the detected platform
func EnsureService(args []string) error {
	var deps []string
	var k0sDisplayName string

	prg := &program{}
	for _, v := range args {
		if v == "controller" || v == "worker" {
			k0sDisplayName = "k0s " + v
			k0sServiceName = k0sServiceName + v
			break
		}
	}

	// initial svc config
	svcConfig := &service.Config{
		Name:        k0sServiceName,
		DisplayName: k0sDisplayName,
		Description: k0sDescription,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	// fetch service type
	svcType := s.Platform()
	switch svcType {
	case "linux-openrc":
		deps = []string{"need net", "use dns", "after firewall"}
	case "linux-systemd":
		deps = []string{"After=network-online.target", "Wants=network-online.target"}
		svcConfig.Option = map[string]interface{}{
			"SystemdScript": systemdScript,
			"LimitNOFILE":   999999,
		}
	default:
	}

	svcConfig.Dependencies = deps
	svcConfig.Arguments = args

	logrus.Info("Installing k0s service")
	err = s.Install()
	if err != nil {
		return fmt.Errorf("failed to install service: %v", err)
	}
	return nil
}

func GetSysInit() (string, error) {
	prg := &program{}
	s, err := service.New(prg, &service.Config{Name: "132"})
	if err != nil {
		return "", err
	}
	return s.Platform(), nil
}

// Upstream kardianos/service does not support all the options we want to set to the systemd unit, hence we override the template
// Currently mostly for KillMode=process so we get systemd to only send the sigterm to the main process
const systemdScript = `[Unit]
Description={{.Description}}
Documentation=https://docs.k0sproject.io
ConditionFileIsExecutable={{.Path|cmdEscape}}
{{range $i, $dep := .Dependencies}}
{{$dep}} {{end}}

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmdEscape}}{{end}}

RestartSec=120
Delegate=yes
KillMode=process
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0

{{- if .ChRoot}}RootDirectory={{.ChRoot|cmd}}{{- end}}

{{- if .WorkingDirectory}}WorkingDirectory={{.WorkingDirectory|cmdEscape}}{{- end}}
{{- if .UserName}}User={{.UserName}}{{end}}
{{- if .ReloadSignal}}ExecReload=/bin/kill -{{.ReloadSignal}} "$MAINPID"{{- end}}
{{- if .PIDFile}}PIDFile={{.PIDFile|cmd}}{{- end}}
{{- if and .LogOutput .HasOutputFileSupport -}}
StandardOutput=file:/var/log/{{.Name}}.out
StandardError=file:/var/log/{{.Name}}.err
{{- end}}

{{- if .SuccessExitStatus}}SuccessExitStatus={{.SuccessExitStatus}}{{- end}}
{{ if gt .LimitNOFILE -1 }}LimitNOFILE={{.LimitNOFILE}}{{- end}}
{{ if .Restart}}Restart={{.Restart}}{{- end}}

[Install]
WantedBy=multi-user.target
`

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

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
{{- if .Option.Environment}}{{range .Option.Environment}}
Environment="{{.}}"{{end}}{{- end}}

RestartSec=10
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

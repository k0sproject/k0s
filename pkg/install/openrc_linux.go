// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

const openRCScript = `#!/sbin/openrc-run
{{- if .Option.Environment}}{{range .Option.Environment}}
export {{.}}{{end}}{{- end}}
supervisor=supervise-daemon
description="{{.Description}}"
command={{.Path|cmdEscape}}
{{- if .Arguments }}
command_args="{{range .Arguments}}'{{.}}' {{end}}"
{{- end }}
name=$(basename $(readlink -f $command))
supervise_daemon_args="--stdout /var/log/${name}.log --stderr /var/log/${name}.err"

: "${rc_ulimit=-n 1048576 -u unlimited}"

{{- if .Dependencies }}
depend() {
{{- range $i, $dep := .Dependencies}} 
{{"\t"}}{{$dep}}{{end}}
}
{{- end}}
`

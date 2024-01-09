/*
Copyright 2022 k0s authors

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

package install

const openRCScript = `#!/sbin/openrc-run
if [ -f "/etc/conf.d/{{.Name}}" ]; then
  . /etc/conf.d/{{.Name}}
fi
{{- if .Option.Environment}}{{range .Option.Environment}}
export {{.}}{{end}}{{- end}}
supervisor=supervise-daemon
name="{{.DisplayName}}"
description="{{.Description}}"
command={{.Path|cmdEscape}}
{{- if .Arguments }}
command_args="{{range .Arguments}}'{{.}}' {{end}} ${K0S_EXTRA_ARGS}"
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

package install

const openRCScript = `#!/sbin/openrc-run
{{- if .Option.Environment}}{{range .Option.Environment}}
export {{.}}{{end}}{{- end}}
supervisor=supervise-daemon
name="{{.DisplayName}}"
description="{{.Description}}"
command={{.Path|cmdEscape}}
{{- if .Arguments }}
command_args="{{range .Arguments}}{{.}} {{end}}"
{{- end }}
name=$(basename $(readlink -f $command))
supervise_daemon_args="--stdout /var/log/${name}.log --stderr /var/log/${name}.err"

{{- if .Dependencies }}
depend() {
{{- range $i, $dep := .Dependencies}} 
{{"\t"}}{{$dep}}{{end}}
}
{{- end}}
`

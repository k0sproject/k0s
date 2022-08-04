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

const upstartScript = `# {{.Description}}

{{- if .Option.Environment}}{{range .Option.Environment}}
env {{.}}{{end}}{{- end}}

{{if .DisplayName}}description    "{{.DisplayName}}"{{end}}

{{if .HasKillStanza}}kill signal INT{{end}}
{{if .ChRoot}}chroot {{.ChRoot}}{{end}}
{{if .WorkingDirectory}}chdir {{.WorkingDirectory}}{{end}}
start on filesystem or runlevel [2345]
stop on runlevel [!2345]

{{if and .UserName .HasSetUIDStanza}}setuid {{.UserName}}{{end}}

respawn
respawn limit 10 5
umask 022

console none

pre-start script
    test -x {{.Path}} || { stop; exit 0; }
end script

# Start
script
	{{if .LogOutput}}
	stdout_log="/var/log/{{.Name}}.out"
	stderr_log="/var/log/{{.Name}}.err"
	{{end}}
	
	if [ -f "/etc/sysconfig/{{.Name}}" ]; then
		set -a
		source /etc/sysconfig/{{.Name}}
		set +a
	fi

	exec {{if and .UserName (not .HasSetUIDStanza)}}sudo -E -u {{.UserName}} {{end}}{{.Path}}{{range .Arguments}} {{.|cmd}}{{end}}{{if .LogOutput}} >> $stdout_log 2>> $stderr_log{{end}}
end script
`

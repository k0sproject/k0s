package install

const launchdConfig = `<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN"
"http://www.apple.com/DTDs/PropertyList-1.0.dtd" >
<plist version='1.0'>
  <dict>
	{{- if .Option.Environment}}
    <key>EnvironmentVariables</key>
	<dict>
	{{range $k, $v := .Option.EnvironmentMap -}}
	<key>{{html $k}}</key>
	<string>{{html $v}}</string>
	{{end -}}
	{{end -}}
	</dict>
    <key>Label</key>
    <string>{{html .Name}}</string>
    <key>ProgramArguments</key>
    <array>
      <string>{{html .Path}}</string>
    {{range .Config.Arguments}}
      <string>{{html .}}</string>
    {{end}}
    </array>
    {{if .UserName}}<key>UserName</key>
    <string>{{html .UserName}}</string>{{end}}
    {{if .ChRoot}}<key>RootDirectory</key>
    <string>{{html .ChRoot}}</string>{{end}}
    {{if .WorkingDirectory}}<key>WorkingDirectory</key>
    <string>{{html .WorkingDirectory}}</string>{{end}}
    <key>SessionCreate</key>
    <{{bool .SessionCreate}}/>
    <key>KeepAlive</key>
    <{{bool .KeepAlive}}/>
    <key>RunAtLoad</key>
    <{{bool .RunAtLoad}}/>
    <key>Disabled</key>
    <false/>
    
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/{{html .Name}}.out.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/{{html .Name}}.err.log</string>
  
  </dict>
</plist>
`

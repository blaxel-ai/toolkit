FROM {{.BaseImage}}
WORKDIR /blaxel
COPY {{ .RequirementFile }} /blaxel/{{ .RequirementFile }}
{{if .LockFile}}
COPY {{.LockFile}} /blaxel/{{.LockFile}}
{{end}}
RUN {{.InstallCommand}}
COPY . .
{{if .BuildCommand}}
RUN {{.BuildCommand}}
{{end}}
ENTRYPOINT ["{{.Entrypoint}}"]

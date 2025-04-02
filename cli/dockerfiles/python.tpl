FROM {{.BaseImage}}
WORKDIR /blaxel
RUN {{ .PreInstall }}
COPY {{ .RequirementFile }} /blaxel/{{ .RequirementFile }}
{{if .LockFile}}
COPY {{.LockFile}} /blaxel/{{.LockFile}}
{{end}}
RUN {{.InstallCommand}}
COPY . .
{{if .BuildCommand}}
RUN {{.BuildCommand}}
{{end}}

ENV PATH="/blaxel/.venv/bin:$PATH"

ENTRYPOINT ["{{.Entrypoint}}"]

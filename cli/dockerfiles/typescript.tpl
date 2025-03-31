FROM {{.BaseImage}}
WORKDIR /blaxel
COPY package.json /blaxel/package.json
{{if .LockFile}}
COPY {{.LockFile}} /blaxel/{{.LockFile}}
{{end}}
RUN {{.InstallCommand}}
COPY . .
{{if .BuildCommand}}
RUN {{.BuildCommand}}
{{end}}
ENTRYPOINT ["{{.Entrypoint}}"]

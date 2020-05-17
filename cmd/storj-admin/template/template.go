//go:generate $GOPATH/bin/go-bindata -pkg template -ignore template.go -ignore bindata.go .

package template

import (
	"html/template"
)

var (
	Public = template.Must(
		template.New("public").Parse(string(MustAsset("public.html.tmpl"))),
	)
)

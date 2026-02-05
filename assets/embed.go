package assets

import _ "embed"

//go:embed com.mirego.ekiden.plist.tmpl
var EkidenPlist []byte

//go:embed config.yaml.example
var ConfigExample []byte

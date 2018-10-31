package dcosupgrade

//go:generate go-bindata -nometadata -pkg $GOPACKAGE -prefix ../../../parts/ -o windows_upgrade_scripts.go ../../../parts/windowsupgradescripts/...
//go:generate gofmt -s -l -w windows_upgrade_scripts.go
// fileloader use go-bindata (https://github.com/go-bindata/go-bindata)
// go-bindata is the way we handle embedded files, like binary, template, etc.

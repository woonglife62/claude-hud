.PHONY: build icon clean

build: resource.syso
	go build -ldflags "-H windowsgui" -o claude-hud.exe

icon:
	go run tools/mkicon.go
	windres resource.rc -o resource.syso

resource.syso: resource.rc icon.ico
	windres resource.rc -o resource.syso

clean:
	rm -f claude-hud.exe resource.syso icon.ico

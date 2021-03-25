



MacOS:
	mkdir -p release/JumpWay.app/Contents/{MacOS,Resources}
	go build -o release/JumpWay.app/Contents/MacOS/JumpWay ./cmd/jumpway
	cp icon/icon.png release/JumpWay.app/Contents/Resources/JumpWay.icns
	./tools/icns.sh
	cp tools/macos/Info.plist release/JumpWay.app/Contents/

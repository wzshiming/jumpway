package icon

import (
	_ "embed"
)

//go:embed icon_black.png
var Black []byte

//go:embed icon_white.png
var White []byte

//go:embed icon_gray.png
var Gray []byte

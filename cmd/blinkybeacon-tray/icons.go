package main

import (
	_ "embed"
	"encoding/binary"
)

//go:embed icons/siren.png
var sirenPNG []byte

// wrapPNGInICO wraps PNG bytes in a minimal ICO container.
// Windows Vista+ supports PNG-in-ICO natively via LoadImageW.
func wrapPNGInICO(pngData []byte) []byte {
	const dataOffset = 22 // 6-byte header + 16-byte directory entry
	ico := make([]byte, dataOffset+len(pngData))

	// ICO file header
	binary.LittleEndian.PutUint16(ico[2:], 1) // type: ICO
	binary.LittleEndian.PutUint16(ico[4:], 1) // image count: 1

	// Image directory entry (width=0 and height=0 mean 256; actual size lives in the PNG header)
	binary.LittleEndian.PutUint16(ico[10:], 1)                     // color planes
	binary.LittleEndian.PutUint16(ico[12:], 32)                    // bits per pixel
	binary.LittleEndian.PutUint32(ico[14:], uint32(len(pngData)))  // data size
	binary.LittleEndian.PutUint32(ico[18:], uint32(dataOffset))    // data offset

	copy(ico[dataOffset:], pngData)
	return ico
}

var iconSiren = wrapPNGInICO(sirenPNG)

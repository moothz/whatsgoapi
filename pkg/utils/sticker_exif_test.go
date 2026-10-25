package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

func TestBuildWhatsAppExif(t *testing.T) {
	meta := StickerMetadata{
		PackName:  "Pacote dos Pampas",
		Publisher: "Gaucho Taura",
		Emojis:    []string{"🧉", "🐎"},
	}

	exifBytes, err := BuildWhatsAppExif(meta)
	if err != nil {
		t.Fatalf("BuildWhatsAppExif failed: %v", err)
	}

	// Validar tamanho mínimo: 22 bytes cabeçalho TIFF + JSON + 4 bytes trailer
	if len(exifBytes) < 26 {
		t.Fatalf("exifBytes too short: %d", len(exifBytes))
	}

	// Validar cabeçalho TIFF 'II' (Little-Endian)
	if !bytes.Equal(exifBytes[0:4], []byte{0x49, 0x49, 0x2A, 0x00}) {
		t.Fatalf("Invalid TIFF magic bytes: %x", exifBytes[0:4])
	}

	// Offset para o primeiro IFD: 8
	if binary.LittleEndian.Uint32(exifBytes[4:8]) != 8 {
		t.Fatalf("Invalid IFD offset: %d", binary.LittleEndian.Uint32(exifBytes[4:8]))
	}

	// Tag ID: 0x41, 0x57 ('AW')
	if exifBytes[10] != 0x41 || exifBytes[11] != 0x57 {
		t.Fatalf("Invalid tag ID: %x %x", exifBytes[10], exifBytes[11])
	}

	// Tipo: 7
	if binary.LittleEndian.Uint16(exifBytes[12:14]) != 7 {
		t.Fatalf("Invalid tag type: %d", binary.LittleEndian.Uint16(exifBytes[12:14]))
	}

	// Tamanho do JSON
	jsonLen := binary.LittleEndian.Uint32(exifBytes[14:18])

	// Offset para dados: 22 (0x16)
	if binary.LittleEndian.Uint32(exifBytes[18:22]) != 22 {
		t.Fatalf("Invalid data offset: %d", binary.LittleEndian.Uint32(exifBytes[18:22]))
	}

	// Extrai e parseia o JSON
	rawJSON := exifBytes[22 : 22+jsonLen]
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("Failed to parse embedded JSON: %v", err)
	}

	if parsed["sticker-pack-name"] != "Pacote dos Pampas" {
		t.Errorf("Unexpected pack-name: %v", parsed["sticker-pack-name"])
	}
	if parsed["sticker-pack-publisher"] != "Gaucho Taura" {
		t.Errorf("Unexpected publisher: %v", parsed["sticker-pack-publisher"])
	}
	if parsed["sticker-pack-id"] == "" {
		t.Errorf("sticker-pack-id should not be empty")
	}
}

// Cria um WebP sintético válido para testes
func createDummyWebP(chunkType string) []byte {
	var chunkPayload []byte
	switch chunkType {
	case "VP8 ":
		// Keyframe VP8 de 512x512
		chunkPayload = make([]byte, 20)
		chunkPayload[3] = 0x9D
		chunkPayload[4] = 0x01
		chunkPayload[5] = 0x2A
		binary.LittleEndian.PutUint16(chunkPayload[6:8], 512)
		binary.LittleEndian.PutUint16(chunkPayload[8:10], 512)
	case "VP8L":
		// Lossless VP8L de 512x512
		chunkPayload = make([]byte, 10)
		chunkPayload[0] = 0x2F
		wMinus1 := uint32(511)
		hMinus1 := uint32(511)
		chunkPayload[1] = byte(wMinus1 & 0xFF)
		chunkPayload[2] = byte((wMinus1>>8)&0x3F) | byte((hMinus1&0x03)<<6)
		chunkPayload[3] = byte((hMinus1 >> 2) & 0xFF)
		chunkPayload[4] = byte((hMinus1 >> 10) & 0x0F)
	case "VP8X":
		chunkPayload = make([]byte, 10)
		binary.LittleEndian.PutUint32(chunkPayload[0:4], 0x00) // Sem flags iniciais
		chunkPayload[4] = byte(511 & 0xFF)
		chunkPayload[5] = byte((511 >> 8) & 0xFF)
		chunkPayload[7] = byte(511 & 0xFF)
		chunkPayload[8] = byte((511 >> 8) & 0xFF)
	}

	payloadLen := len(chunkPayload)
	paddedLen := payloadLen + (payloadLen & 1)
	totalRIFF := 4 + 8 + paddedLen // WEBP + FourCC + Size + Payload

	buf := make([]byte, 0, 8+totalRIFF)
	buf = append(buf, []byte("RIFF")...)
	sizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(totalRIFF))
	buf = append(buf, sizeBuf...)
	buf = append(buf, []byte("WEBP")...)

	buf = append(buf, []byte(chunkType)...)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(payloadLen))
	buf = append(buf, sizeBuf...)
	buf = append(buf, chunkPayload...)
	if payloadLen&1 != 0 {
		buf = append(buf, 0x00)
	}

	return buf
}

func TestInjectStickerExif(t *testing.T) {
	meta := StickerMetadata{
		PackName:  "Sticker Pack Test",
		Publisher: "Ravena",
		Emojis:    []string{"🔥"},
	}

	testCases := []struct {
		name      string
		chunkType string
	}{
		{"Lossy VP8", "VP8 "},
		{"Lossless VP8L", "VP8L"},
		{"Extended VP8X", "VP8X"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputWebP := createDummyWebP(tc.chunkType)
			injected, err := InjectStickerExif(inputWebP, meta)
			if err != nil {
				t.Fatalf("InjectStickerExif failed for %s: %v", tc.name, err)
			}

			// Validar integridade RIFF/WEBP
			if len(injected) < 12 {
				t.Fatalf("Output too small")
			}
			if string(injected[0:4]) != "RIFF" || string(injected[8:12]) != "WEBP" {
				t.Fatalf("Invalid RIFF/WEBP header")
			}

			totalSize := binary.LittleEndian.Uint32(injected[4:8])
			if int(totalSize)+8 != len(injected) {
				t.Fatalf("RIFF total size mismatch: header says %d, actual len is %d", totalSize+8, len(injected))
			}

			// Primeiro chunk DEVE ser VP8X
			if string(injected[12:16]) != "VP8X" {
				t.Fatalf("Expected first chunk to be VP8X, got %s", string(injected[12:16]))
			}

			// Verificar se a flag EXIF (bit 3 = 0x08) está ligada no VP8X
			vp8xFlags := binary.LittleEndian.Uint32(injected[20:24])
			if (vp8xFlags & 0x08) == 0 {
				t.Fatalf("VP8X EXIF flag is not set: %x", vp8xFlags)
			}

			// Verificar se o chunk EXIF está presente e contém nossos metadados
			var foundEXIF bool
			offset := 12
			for offset < len(injected) {
				if offset+8 > len(injected) {
					break
				}
				fourCC := string(injected[offset : offset+4])
				cSize := binary.LittleEndian.Uint32(injected[offset+4 : offset+8])
				pStart := offset + 8
				pEnd := pStart + int(cSize)

				if fourCC == "EXIF" {
					foundEXIF = true
					payload := injected[pStart:pEnd]
					// Validar TIFF header
					if !bytes.Equal(payload[0:4], []byte{0x49, 0x49, 0x2A, 0x00}) {
						t.Errorf("EXIF chunk missing TIFF header")
					}
					// Validar JSON
					jsonLen := binary.LittleEndian.Uint32(payload[14:18])
					jsonBytes := payload[22 : 22+jsonLen]
					var res map[string]interface{}
					if err := json.Unmarshal(jsonBytes, &res); err != nil {
						t.Errorf("Failed to parse EXIF JSON: %v", err)
					}
					if res["sticker-pack-name"] != "Sticker Pack Test" || res["sticker-pack-publisher"] != "Ravena" {
						t.Errorf("Metadata mismatch in EXIF: %v", res)
					}
				}

				padded := int(cSize) + int(cSize&1)
				offset = pStart + padded
			}

			if !foundEXIF {
				t.Fatalf("EXIF chunk was not injected into WebP")
			}
		})
	}
}

func TestInjectStickerExif_ReplacesExistingEXIF(t *testing.T) {
	inputWebP := createDummyWebP("VP8X")

	meta1 := StickerMetadata{PackName: "Pack 1", Publisher: "Author 1"}
	meta2 := StickerMetadata{PackName: "Pack 2", Publisher: "Author 2"}

	// Primeira injeção
	firstPass, err := InjectStickerExif(inputWebP, meta1)
	if err != nil {
		t.Fatalf("First injection failed: %v", err)
	}

	// Segunda injeção deve substituir a primeira
	secondPass, err := InjectStickerExif(firstPass, meta2)
	if err != nil {
		t.Fatalf("Second injection failed: %v", err)
	}

	// Contar quantos chunks EXIF existem (deve ser exatamente 1)
	exifCount := 0
	offset := 12
	for offset < len(secondPass) {
		if offset+8 > len(secondPass) {
			break
		}
		fourCC := string(secondPass[offset : offset+4])
		cSize := binary.LittleEndian.Uint32(secondPass[offset+4 : offset+8])
		if fourCC == "EXIF" {
			exifCount++
			jsonLen := binary.LittleEndian.Uint32(secondPass[offset+8+14 : offset+8+18])
			jsonBytes := secondPass[offset+8+22 : offset+8+22+int(jsonLen)]
			var res map[string]interface{}
			_ = json.Unmarshal(jsonBytes, &res)
			if res["sticker-pack-name"] != "Pack 2" {
				t.Errorf("Did not replace old EXIF: %v", res)
			}
		}
		padded := int(cSize) + int(cSize&1)
		offset = offset + 8 + padded
	}

	if exifCount != 1 {
		t.Fatalf("Expected exactly 1 EXIF chunk, found %d", exifCount)
	}
}

func TestGenerateStyxSticker(t *testing.T) {
	inputPath := "/home/moothz/ravena-ai/temp/styx_ffmpeg.webp"
	outputPath := "/home/moothz/ravena-ai/public/attachments/styx_essence_exif.webp"

	inputWebP, err := os.ReadFile(inputPath)
	if err != nil {
		t.Skipf("Skipping real file test: %v", err)
		return
	}

	meta := StickerMetadata{
		PackName:  "Jardim das Sombras",
		Publisher: "Ravena AI (Taura)",
		Emojis:    []string{"🌿", "🔮"},
	}

	injected, err := InjectStickerExif(inputWebP, meta)
	if err != nil {
		t.Fatalf("Failed to inject EXIF: %v", err)
	}

	if err := os.WriteFile(outputPath, injected, 0644); err != nil {
		t.Fatalf("Failed to save output WebP: %v", err)
	}
	t.Logf("Successfully generated %s (%d bytes)", outputPath, len(injected))
}

func TestMimeDetect(t *testing.T) {
	data, err := os.ReadFile("/home/moothz/ravena-ai/public/attachments/styx_essence_exif.webp")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("DETECTED MIME: '%s'", http.DetectContentType(data))
}

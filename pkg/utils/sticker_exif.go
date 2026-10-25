package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// StickerMetadata representa os metadados do pacote e autor de figurinhas do WhatsApp
type StickerMetadata struct {
	PackID    string   `json:"sticker-pack-id"`
	PackName  string   `json:"sticker-pack-name"`
	Publisher string   `json:"sticker-pack-publisher"`
	Emojis    []string `json:"emojis,omitempty"`
}

var (
	ErrNotWebP         = errors.New("data is not a valid WebP image (missing RIFF/WEBP header)")
	ErrWebPTooShort    = errors.New("WebP data is truncated or too short")
	ErrCorruptWebP     = errors.New("corrupted WebP chunks")
	ErrUnsupportedWebP = errors.New("unsupported WebP image structure")
)

// BuildWhatsAppExif constrói a carga binária TIFF little-endian requerida pelo WhatsApp
func BuildWhatsAppExif(meta StickerMetadata) ([]byte, error) {
	if meta.PackID == "" {
		h := sha256.Sum256([]byte(meta.PackName + "|" + meta.Publisher))
		meta.PackID = hex.EncodeToString(h[:16])
	}
	if meta.Emojis == nil {
		meta.Emojis = []string{}
	}

	jsonMap := map[string]interface{}{
		"sticker-pack-id":        meta.PackID,
		"sticker-pack-name":      meta.PackName,
		"sticker-pack-publisher": meta.Publisher,
		"emojis":                 meta.Emojis,
	}

	jsonBytes, err := json.Marshal(jsonMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sticker metadata: %w", err)
	}

	// Cabeçalho TIFF Little-Endian (Intel 'II')
	// 0x49, 0x49, 0x2A, 0x00: TIFF little endian marker ('II', 42)
	// 0x08, 0x00, 0x00, 0x00: Offset para o primeiro IFD (8)
	// 0x01, 0x00: 1 entrada no diretório IFD
	// 0x41, 0x57: Tag privada proprietária do WhatsApp (ASCII 'AW' = 0x5741)
	// 0x07, 0x00: Tipo 7 (UNDEFINED bytes brutos)
	// 4 bytes: Tamanho do JSON (Little Endian uint32)
	// 0x16, 0x00, 0x00, 0x00: Offset para a carga JSON (22 bytes a partir do início = 0x16)
	// jsonBytes
	// 0x00, 0x00, 0x00, 0x00: Próximo IFD offset (0)
	exifHeader := []byte{
		0x49, 0x49, 0x2A, 0x00,
		0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x41, 0x57,
		0x07, 0x00,
		0x00, 0x00, 0x00, 0x00, // Preenchido com len(jsonBytes)
		0x16, 0x00, 0x00, 0x00,
	}
	binary.LittleEndian.PutUint32(exifHeader[14:18], uint32(len(jsonBytes)))

	totalLen := len(exifHeader) + len(jsonBytes)
	buf := make([]byte, 0, totalLen)
	buf = append(buf, exifHeader...)
	buf = append(buf, jsonBytes...)

	return buf, nil
}

// riffChunk representa um chunk interno do contêiner RIFF
type riffChunk struct {
	fourCC  string
	payload []byte
}

// InjectStickerExif injeta os metadados EXIF no contêiner WebP, promovendo para VP8X se necessário
func InjectStickerExif(webpData []byte, meta StickerMetadata) ([]byte, error) {
	if len(webpData) < 12 {
		return nil, ErrWebPTooShort
	}

	if string(webpData[0:4]) != "RIFF" || string(webpData[8:12]) != "WEBP" {
		return nil, ErrNotWebP
	}

	exifPayload, err := BuildWhatsAppExif(meta)
	if err != nil {
		return nil, err
	}

	// Faz o parse de todos os chunks do contêiner
	offset := 12
	var chunks []riffChunk
	var hasVP8X bool
	var vp8xFlags uint32
	var canvasWidthMinus1 uint32
	var canvasHeightMinus1 uint32
	var hasAlpha bool

	for offset < len(webpData) {
		if offset+8 > len(webpData) {
			break
		}

		fourCC := string(webpData[offset : offset+4])
		chunkSize := binary.LittleEndian.Uint32(webpData[offset+4 : offset+8])
		payloadStart := offset + 8
		payloadEnd := payloadStart + int(chunkSize)

		if payloadEnd > len(webpData) {
			return nil, ErrCorruptWebP
		}

		payload := make([]byte, chunkSize)
		copy(payload, webpData[payloadStart:payloadEnd])

		if fourCC == "VP8X" {
			hasVP8X = true
			if len(payload) >= 10 {
				vp8xFlags = binary.LittleEndian.Uint32(payload[0:4])
				canvasWidthMinus1 = uint32(payload[4]) | (uint32(payload[5]) << 8) | (uint32(payload[6]) << 16)
				canvasHeightMinus1 = uint32(payload[7]) | (uint32(payload[8]) << 8) | (uint32(payload[9]) << 16)
			}
		} else if fourCC == "VP8 " && !hasVP8X {
			// WebP Lossy simples: extrai dimensões do cabeçalho de frame VP8
			if len(payload) >= 10 && payload[3] == 0x9D && payload[4] == 0x01 && payload[5] == 0x2A {
				w := binary.LittleEndian.Uint16(payload[6:8]) & 0x3FFF
				h := binary.LittleEndian.Uint16(payload[8:10]) & 0x3FFF
				if w > 0 {
					canvasWidthMinus1 = uint32(w - 1)
				}
				if h > 0 {
					canvasHeightMinus1 = uint32(h - 1)
				}
			}
		} else if fourCC == "VP8L" && !hasVP8X {
			// WebP Lossless simples: extrai dimensões do cabeçalho VP8L
			if len(payload) >= 5 && payload[0] == 0x2F {
				canvasWidthMinus1 = uint32(payload[1]) | (uint32(payload[2]&0x3F) << 8)
				canvasHeightMinus1 = (uint32(payload[2]>>6)) | (uint32(payload[3]) << 2) | (uint32(payload[4]&0x0F) << 10)
				hasAlpha = (payload[4] & 0x10) != 0
			}
		} else if fourCC == "ALPH" {
			hasAlpha = true
		}

		// Descarta chunk EXIF existente para substituir pelo novo
		if fourCC != "EXIF" {
			chunks = append(chunks, riffChunk{
				fourCC:  fourCC,
				payload: payload,
			})
		}

		// Chunks em RIFF são alinhados em múltiplos de 2 bytes
		paddedSize := int(chunkSize) + int(chunkSize&1)
		offset = payloadStart + paddedSize
	}

	// Prepara a lista final de chunks
	var finalChunks []riffChunk

	// Garante a presença do chunk VP8X
	vp8xPayload := make([]byte, 10)
	vp8xFlags |= 0x08 // Ativa flag EXIF (bit 3)
	if hasAlpha {
		vp8xFlags |= 0x10 // Ativa flag Alpha (bit 4)
	}

	binary.LittleEndian.PutUint32(vp8xPayload[0:4], vp8xFlags)
	vp8xPayload[4] = byte(canvasWidthMinus1 & 0xFF)
	vp8xPayload[5] = byte((canvasWidthMinus1 >> 8) & 0xFF)
	vp8xPayload[6] = byte((canvasWidthMinus1 >> 16) & 0xFF)
	vp8xPayload[7] = byte(canvasHeightMinus1 & 0xFF)
	vp8xPayload[8] = byte((canvasHeightMinus1 >> 8) & 0xFF)
	vp8xPayload[9] = byte((canvasHeightMinus1 >> 16) & 0xFF)

	finalChunks = append(finalChunks, riffChunk{
		fourCC:  "VP8X",
		payload: vp8xPayload,
	})

	// Anexa os demais chunks (exceto VP8X original se existia, pois substituímos no topo)
	var xmpChunk *riffChunk
	for _, ch := range chunks {
		if ch.fourCC == "VP8X" {
			continue
		}
		if ch.fourCC == "XMP " {
			// Guarda XMP para vir depois do EXIF de acordo com a spec WebP
			xmpCopy := ch
			xmpChunk = &xmpCopy
			continue
		}
		finalChunks = append(finalChunks, ch)
	}

	// Anexa o novo chunk EXIF
	finalChunks = append(finalChunks, riffChunk{
		fourCC:  "EXIF",
		payload: exifPayload,
	})

	// Anexa XMP no final se existia
	if xmpChunk != nil {
		finalChunks = append(finalChunks, *xmpChunk)
	}

	// Calcula tamanho total do novo arquivo
	totalPayloadSize := 4 // "WEBP"
	for _, ch := range finalChunks {
		chunkLen := len(ch.payload)
		paddedLen := chunkLen + (chunkLen & 1)
		totalPayloadSize += 8 + paddedLen
	}

	// Monta o buffer final
	out := make([]byte, 0, 8+totalPayloadSize)
	out = append(out, []byte("RIFF")...)

	sizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(totalPayloadSize))
	out = append(out, sizeBuf...)
	out = append(out, []byte("WEBP")...)

	for _, ch := range finalChunks {
		out = append(out, []byte(ch.fourCC)...)
		binary.LittleEndian.PutUint32(sizeBuf, uint32(len(ch.payload)))
		out = append(out, sizeBuf...)
		out = append(out, ch.payload...)
		if len(ch.payload)&1 != 0 {
			out = append(out, 0x00) // Padding byte
		}
	}

	return out, nil
}

package utils

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/vincent-petithory/dataurl"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ProcessProfilePictureBytes decodifica, corta quadrado centralizado,
// redimensiona para até 640x640 e codifica em JPEG puro a 90% de qualidade,
// atendendo aos requisitos estritos da API do WhatsApp.
func ProcessProfilePictureBytes(imageInput string) ([]byte, error) {
	var rawData []byte

	imageInput = strings.TrimSpace(imageInput)
	if imageInput == "" {
		return nil, errors.New("image input is empty")
	}

	if strings.HasPrefix(imageInput, "http://") || strings.HasPrefix(imageInput, "https://") {
		resp, err := http.Get(imageInput)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch image from URL: %w", err)
		}
		defer resp.Body.Close()
		rawData, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read image data from URL: %w", err)
		}
	} else if strings.HasPrefix(imageInput, "data:") {
		dataURL, err := dataurl.DecodeString(imageInput)
		if err != nil {
			return nil, fmt.Errorf("failed to decode data URL: %w", err)
		}
		rawData = dataURL.Data
	} else if fileBytes, err := os.ReadFile(imageInput); err == nil {
		rawData = fileBytes
	} else if decoded, err := base64.StdEncoding.DecodeString(imageInput); err == nil && len(decoded) > 0 {
		rawData = decoded
	} else {
		return nil, errors.New("invalid image: must be HTTP URL, data URL, base64 string, or local file path")
	}

	if len(rawData) == 0 {
		return nil, errors.New("image data is empty")
	}

	// Decodifica imagem (suporta PNG, WebP, GIF, JPEG, etc.)
	img, _, err := image.Decode(bytes.NewReader(rawData))
	if err != nil {
		// Se falhar o decode, tenta passar os bytes originais se ja for JPEG
		if len(rawData) > 3 && rawData[0] == 0xFF && rawData[1] == 0xD8 && rawData[2] == 0xFF {
			return rawData, nil
		}
		return nil, fmt.Errorf("failed to decode image format: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// O WhatsApp exige avatar quadrado com dimensao recomendada de ate 640x640
	minDim := width
	if height < minDim {
		minDim = height
	}
	startX := bounds.Min.X + (width-minDim)/2
	startY := bounds.Min.Y + (height-minDim)/2
	cropRect := image.Rect(startX, startY, startX+minDim, startY+minDim)

	targetDim := 640
	if minDim < targetDim {
		targetDim = minDim
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetDim, targetDim))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, cropRect, draw.Over, nil)

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

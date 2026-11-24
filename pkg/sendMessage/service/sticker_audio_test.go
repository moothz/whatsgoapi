package send_service

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func buildMinimalWAV(samples int, sampleRate int) []byte {
	dataSize := samples * 2
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	for i := 0; i < samples; i++ {
		_ = binary.Write(&buf, binary.LittleEndian, int16((i%80)*200))
	}
	return buf.Bytes()
}

func buildMinimalVP8XWebP(width uint32, height uint32, animated bool) []byte {
	var vp8x [10]byte
	if animated {
		vp8x[0] = 0x02
	}
	w := width - 1
	h := height - 1
	vp8x[4] = byte(w)
	vp8x[5] = byte(w >> 8)
	vp8x[6] = byte(w >> 16)
	vp8x[7] = byte(h)
	vp8x[8] = byte(h >> 8)
	vp8x[9] = byte(h >> 16)

	var body bytes.Buffer
	body.WriteString("WEBP")
	body.WriteString("VP8X")
	_ = binary.Write(&body, binary.LittleEndian, uint32(len(vp8x)))
	body.Write(vp8x[:])

	var full bytes.Buffer
	full.WriteString("RIFF")
	_ = binary.Write(&full, binary.LittleEndian, uint32(body.Len()))
	full.Write(body.Bytes())
	return full.Bytes()
}

func TestInspectWebPSticker(t *testing.T) {
	staticWebP := buildMinimalVP8XWebP(512, 256, false)
	w, h, anim := inspectWebPSticker(staticWebP)
	if w != 512 || h != 256 || anim {
		t.Fatalf("expected 512x256 static, got %dx%d anim=%v", w, h, anim)
	}

	animWebP := buildMinimalVP8XWebP(320, 320, true)
	w, h, anim = inspectWebPSticker(animWebP)
	if w != 320 || h != 320 || !anim {
		t.Fatalf("expected 320x320 animated, got %dx%d anim=%v", w, h, anim)
	}
}

func TestExtractStickerMessageSupportsAudioStickerWrapper(t *testing.T) {
	msg := &waE2E.Message{
		AudioStickerMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				StickerMessage: &waE2E.StickerMessage{
					Mimetype: proto.String("image/webp"),
					Audio: &waE2E.StickerMessage_AudioMessage{
						AudioMessage: &waE2E.AudioMessage{
							Mimetype: proto.String("audio/ogg; codecs=opus"),
						},
					},
				},
			},
		},
	}

	got := extractStickerMessage(msg)
	if got == nil || got.GetAudioMessage() == nil {
		t.Fatalf("expected extractStickerMessage to unwrap AudioStickerMessage")
	}
}

func TestFetchMediaFromInputSupportsHTTPAndDataURI(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(pngBuf.Bytes())
	}))
	defer srv.Close()

	httpBytes, err := fetchMediaFromInput(srv.URL)
	if err != nil || !bytes.Equal(httpBytes, pngBuf.Bytes()) {
		t.Fatalf("expected HTTP media match, err=%v", err)
	}

	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	uriBytes, err := fetchMediaFromInput(dataURI)
	if err != nil || !bytes.Equal(uriBytes, pngBuf.Bytes()) {
		t.Fatalf("expected data URI media match, err=%v", err)
	}

	webpBytes, err := convertToWebP(dataURI, "")
	if err != nil {
		t.Fatalf("expected PNG data URI to convert to WebP, err=%v", err)
	}
	if len(webpBytes) < 12 || string(webpBytes[0:4]) != "RIFF" || string(webpBytes[8:12]) != "WEBP" {
		t.Fatalf("expected valid WebP header")
	}
}

func TestConvertAudioToStickerOpus(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable in environment")
	}

	wav := buildMinimalWAV(16000, 16000) // ~1 segundo
	opusBytes, seconds, err := convertAudioToStickerOpus(wav)
	if err != nil {
		t.Fatalf("expected audio conversion to succeed, got err=%v", err)
	}
	if len(opusBytes) == 0 || len(opusBytes) > maxStickerAudioBytes {
		t.Fatalf("unexpected opus size: %d", len(opusBytes))
	}
	if !bytes.HasPrefix(opusBytes, []byte("OggS")) {
		t.Fatalf("expected OggS header in converted sticker audio")
	}
	if seconds < 1 {
		t.Fatalf("expected seconds >= 1, got %d", seconds)
	}
}

package whatsmeow_service

import (
	"bytes"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestViewOnceCacheSetGetAndEviction(t *testing.T) {
	c := NewViewOnceCache(5*time.Minute, 2, 1024)

	c.Set("inst1", "msg1", []byte("one"), "image/jpeg", "msg1.jpg")
	c.Set("inst1", "msg2", []byte("two"), "video/mp4", "msg2.mp4")

	if got, ok := c.Get("inst1", "msg1"); !ok || !bytes.Equal(got.Data, []byte("one")) || got.Mimetype != "image/jpeg" {
		t.Fatalf("expected cached msg1, got ok=%v val=%+v", ok, got)
	}

	// Exceeds maxEntries=2 -> msg1 evicted
	c.Set("inst1", "msg3", []byte("three"), "audio/ogg", "msg3.ogg")
	if _, ok := c.Get("inst1", "msg1"); ok {
		t.Fatalf("expected msg1 to be evicted")
	}
	if _, ok := c.Get("inst1", "msg3"); !ok {
		t.Fatalf("expected msg3 to be present")
	}

	// Oversized item ignored
	c.Set("inst1", "msgBig", make([]byte, 2048), "image/jpeg", "big.jpg")
	if _, ok := c.Get("inst1", "msgBig"); ok {
		t.Fatalf("expected oversized entry to be ignored")
	}
}

func TestViewOnceCacheExpiration(t *testing.T) {
	c := NewViewOnceCache(10*time.Millisecond, 10, 1024)
	c.Set("inst1", "msgExp", []byte("bye"), "image/png", "exp.png")
	time.Sleep(25 * time.Millisecond)

	if _, ok := c.Get("inst1", "msgExp"); ok {
		t.Fatalf("expected expired entry to miss")
	}
}

func TestUnwrapViewOnceAndEphemeral(t *testing.T) {
	img := &waE2E.ImageMessage{Mimetype: proto.String("image/jpeg"), ViewOnce: proto.Bool(true)}
	nested := &waE2E.Message{
		EphemeralMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ViewOnceMessageV2: &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						ImageMessage: img,
					},
				},
			},
		},
	}

	unwrapped, isVO := unwrapViewOnceAndEphemeral(nested)
	if !isVO {
		t.Fatalf("expected nested Ephemeral -> ViewOnceV2 to be detected as viewOnce")
	}
	if unwrapped.GetImageMessage() == nil {
		t.Fatalf("expected inner ImageMessage after unwrap")
	}

	extNested := &waE2E.Message{
		ViewOnceMessageV2Extension: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{Mimetype: proto.String("audio/ogg")},
			},
		},
	}
	unwrappedExt, isVOExt := unwrapViewOnceAndEphemeral(extNested)
	if !isVOExt || unwrappedExt.GetAudioMessage() == nil {
		t.Fatalf("expected ViewOnceMessageV2Extension unwrap to succeed")
	}
}

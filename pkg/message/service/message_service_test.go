package message_service

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	instance_model "whatsgo/pkg/instance/model"
	whatsmeow_service "whatsgo/pkg/whatsmeow/service"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

type stubWhatsmeowService struct {
	whatsmeow_service.WhatsmeowService
	cache *whatsmeow_service.ViewOnceCache
}

func (s *stubWhatsmeowService) GetViewOnceMedia(instanceId, messageId string) (*whatsmeow_service.ViewOnceCachedMedia, bool) {
	return s.cache.Get(instanceId, messageId)
}

func TestUnwrapMediaMessageNested(t *testing.T) {
	nested := &waE2E.Message{
		EphemeralMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ViewOnceMessageV2: &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						VideoMessage: &waE2E.VideoMessage{
							Mimetype: proto.String("video/mp4"),
							ViewOnce: proto.Bool(true),
						},
					},
				},
			},
		},
	}

	unwrapped := unwrapMediaMessage(nested)
	kind, downloadable, mimetype := resolveDownloadTarget(unwrapped)
	if kind != "video" || downloadable == nil || mimetype != "video/mp4" {
		t.Fatalf("expected unwrapped video/mp4, got kind=%q dl=%v mime=%q", kind, downloadable != nil, mimetype)
	}
}

func TestDownloadMediaUsesViewOnceCacheWhenNoURL(t *testing.T) {
	cache := whatsmeow_service.NewViewOnceCache(5*time.Minute, 10, 1024*1024)
	cache.Set("inst1", "VO123", []byte("img-bytes"), "image/jpeg", "VO123.jpg")

	svc := &messageService{
		whatsmeowService: &stubWhatsmeowService{cache: cache},
	}

	_ = svc
	got, ok := svc.whatsmeowService.GetViewOnceMedia("inst1", "VO123")
	if !ok || !bytes.Equal(got.Data, []byte("img-bytes")) || got.Mimetype != "image/jpeg" {
		t.Fatalf("expected view-once cache hit, got ok=%v got=%+v", ok, got)
	}
	_ = (&instance_model.Instance{Id: "inst1"})
	_ = (*http.Request)(nil)
}

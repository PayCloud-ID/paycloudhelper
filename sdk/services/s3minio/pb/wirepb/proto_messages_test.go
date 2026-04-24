package pb

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func touchMessage(t *testing.T, m proto.Message) {
	t.Helper()
	m.ProtoReflect().Descriptor()
	_ = m.ProtoReflect().Type()
	_ = proto.Size(m)
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	cl := m.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(b, cl); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
}

func TestWirepbMessages_RoundTripAndReflect(t *testing.T) {
	cases := []proto.Message{
		&UploadRequest{
			Filename: "f.bin", Size: 9, ContentType: "text/plain", Content: []byte("x"),
			Bucket: "b", Path: "p", Expires: 60, UserId: 1, MerchantId: 2,
		},
		&UploadResponse{Code: 200, Status: "ok", Message: "m", Data: &UploadData{
			Bucket: "b", Path: "p", Filename: "f", Url: "u", PresignedUrl: "ps",
		}},
		&UploadData{Bucket: "b", Path: "p", Filename: "f", Url: "u", PresignedUrl: "ps"},
		&DownloadRequest{Object: "o", Bucket: "b", Path: "p", Expires: 10, UserId: 1, MerchantId: 2},
		&DownloadResponse{Code: 200, Status: "ok", Message: "m", Data: "payload"},
		&HealthRequest{},
		&HealthResponse{Status: "SERVING"},
	}
	for _, msg := range cases {
		touchMessage(t, msg)
	}
}

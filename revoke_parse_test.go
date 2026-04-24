package paycloudhelper

import (
	"encoding/pem"
	"strings"
	"testing"
)

func TestParsePublicKey_invalidPEM(t *testing.T) {
	_, err := parsePublicKey([]byte("not valid pem"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no key found") {
		t.Fatalf("err=%v", err)
	}
}

func TestParsePublicKey_unsupportedBlockType(t *testing.T) {
	b := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x01}})
	_, err := parsePublicKey(b)
	if err == nil {
		t.Fatal("expected error")
	}
}

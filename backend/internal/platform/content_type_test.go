package platform

import (
	"strings"
	"testing"
)

func TestDetectContentType(t *testing.T) {
	ct := DetectContentType("application/octet-stream", []byte(`{"files":[]}`))
	if !strings.Contains(ct, "json") {
		t.Fatal(ct)
	}
	ct = DetectContentType("", []byte(`<html></html>`))
	if !strings.Contains(ct, "html") {
		t.Fatal(ct)
	}
}

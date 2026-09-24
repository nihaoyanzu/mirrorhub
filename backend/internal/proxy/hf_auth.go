package proxy

import (
	"net/http"

	hfhandler "github.com/livehl/mirrorhub/internal/handlers/huggingface"
)

func stripXetFromHeader(h http.Header) {
	if h == nil {
		return
	}
	hfhandler.StripXetHeaders(h)
}

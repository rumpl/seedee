package cli

import (
	"net/http"
	"strings"

	"github.com/rumpl/seedee/gen/seedee/v1/seedeev1connect"
)

func newCIClient(addr string) seedeev1connect.CIServiceClient {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return seedeev1connect.NewCIServiceClient(http.DefaultClient, addr)
}

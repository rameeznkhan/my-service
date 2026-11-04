package mux

import (
	"net/http"

	"github.com/rameeznkhan/my-service/api/services/sales/route/sys/checkapi"
)

func WebAPI() *http.ServeMux {
	mux := http.NewServeMux()

	checkapi.Routes(mux)
	return mux
}

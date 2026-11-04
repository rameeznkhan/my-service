package mux

import (
	"os"

	"github.com/rameeznkhan/my-service/api/services/sales/route/sys/checkapi"
	"github.com/rameeznkhan/my-service/foundation/web"
)

func WebAPI(shutdown chan os.Signal) *web.App {

	app := web.NewApp(shutdown)
	checkapi.Routes(app)
	return app
}

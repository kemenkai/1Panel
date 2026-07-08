package v2

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/1Panel-dev/1Panel/agent/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/agent/app/dto/request"
	"github.com/gin-gonic/gin"
)

// @Tags Windows service
// @Summary List windows services
// @Success 200 {array} response.WindowsServiceInfo
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services [get]
func (b *BaseApi) ListWindowsServices(c *gin.Context) {
	data, err := windowsServiceService.List()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, data)
}

// @Tags Windows service
// @Summary Get default windows service config template
// @Success 200 {object} response.WindowsServiceConfigTemplate
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/config-template [get]
func (b *BaseApi) GetWindowsServiceConfigTemplate(c *gin.Context) {
	serviceType := c.Query("serviceType")
	data, err := windowsServiceService.GetConfigTemplate(serviceType)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, data)
}

// @Tags Windows service
// @Summary Get windows service config file
// @Success 200 {object} response.WindowsServiceConfigFile
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/:id/config-file [get]
func (b *BaseApi) GetWindowsServiceConfigFile(c *gin.Context) {
	id, err := helper.GetParamID(c)
	if err != nil {
		helper.BadRequest(c, err)
		return
	}
	fileType := c.Query("type")
	data, err := windowsServiceService.GetConfigFile(id, fileType)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, data)
}

// @Tags Windows service
// @Summary Update windows service config file
// @Accept json
// @Param request body request.WindowsServiceConfigFileUpdate true "request"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/:id/config-file [post]
func (b *BaseApi) UpdateWindowsServiceConfigFile(c *gin.Context) {
	id, err := helper.GetParamID(c)
	if err != nil {
		helper.BadRequest(c, err)
		return
	}
	var req request.WindowsServiceConfigFileUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := windowsServiceService.UpdateConfigFile(id, req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Windows service
// @Summary Get windows service log metadata
// @Success 200 {object} response.WindowsServiceLogFile
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/:id/log [get]
func (b *BaseApi) GetWindowsServiceLogFile(c *gin.Context) {
	id, err := helper.GetParamID(c)
	if err != nil {
		helper.BadRequest(c, err)
		return
	}
	data, err := windowsServiceService.GetLogFile(id)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, data)
}

// @Tags Windows service
// @Summary Create windows service
// @Accept json
// @Param request body request.WindowsServiceCreate true "request"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services [post]
func (b *BaseApi) CreateWindowsService(c *gin.Context) {
	var req request.WindowsServiceCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := windowsServiceService.Create(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Windows service
// @Summary Upload windows service package
// @Accept multipart/form-data
// @Success 200 {object} response.WindowsServiceJarUploadResult
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/upload-package [post]
func (b *BaseApi) UploadWindowsServicePackage(c *gin.Context) {
	name := c.PostForm("name")
	serviceType := c.PostForm("serviceType")
	workDir := c.PostForm("workDir")
	if strings.TrimSpace(workDir) != "" {
		data, err := windowsServiceService.PreviewPackage(serviceType, workDir)
		if err != nil {
			helper.InternalServer(c, err)
			return
		}
		helper.SuccessWithData(c, data)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		helper.BadRequest(c, err)
		return
	}
	safeFileName := filepath.Base(file.Filename)
	if strings.EqualFold(filepath.Ext(safeFileName), ".jar") && (name == "" || serviceType == "") {
		helper.BadRequest(c, fmt.Errorf("name and serviceType are required"))
		return
	}
	src, err := file.Open()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	defer src.Close()

	data, err := windowsServiceService.UploadPackage(name, serviceType, safeFileName, src)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, data)
}

// @Tags Windows service
// @Summary Upload windows service jar
// @Accept multipart/form-data
// @Success 200 {object} response.WindowsServiceJarUploadResult
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/upload-jar [post]
func (b *BaseApi) UploadWindowsServiceJar(c *gin.Context) {
	b.UploadWindowsServicePackage(c)
}

// @Tags Windows service
// @Summary Update windows service
// @Accept json
// @Param request body request.WindowsServiceUpdate true "request"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/update [post]
func (b *BaseApi) UpdateWindowsService(c *gin.Context) {
	var req request.WindowsServiceUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := windowsServiceService.Update(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Windows service
// @Summary Operate windows service
// @Accept json
// @Param request body request.WindowsServiceOperate true "request"
// @Success 200 {object} response.WindowsServiceInfo
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/operate [post]
func (b *BaseApi) OperateWindowsService(c *gin.Context) {
	var req request.WindowsServiceOperate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	data, err := windowsServiceService.Operate(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, data)
}

// @Tags Windows service
// @Summary Delete windows service
// @Accept json
// @Param request body request.DelReq true "request"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /windows/services/del [post]
func (b *BaseApi) DeleteWindowsService(c *gin.Context) {
	var req struct {
		ID uint `json:"id" validate:"required"`
	}
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := windowsServiceService.Delete(req.ID); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

package controller

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

type SingboxController struct {
	service *service.SingboxService
}

type singboxInboundRequest struct {
	Remark   string          `json:"remark"`
	Enable   bool            `json:"enable"`
	Listen   string          `json:"listen"`
	Port     int             `json:"port"`
	Protocol string          `json:"protocol"`
	Tag      string          `json:"tag"`
	Settings json.RawMessage `json:"settings"`
}

type singboxEnableRequest struct {
	Enable bool `json:"enable"`
}

func NewSingboxController(g *gin.RouterGroup) *SingboxController {
	a := &SingboxController{service: service.NewSingboxService()}
	a.initRouter(g)
	return a
}

func (a *SingboxController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)
	g.GET("/status", a.status)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)
	g.POST("/check", a.check)
	g.POST("/restart", a.restart)
}

func (a *SingboxController) list(c *gin.Context) {
	user := session.GetLoginUser(c)
	rows, err := a.service.GetInbounds(user.Id)
	jsonObj(c, rows, err)
}

func (a *SingboxController) get(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	row, err := a.service.GetInbound(user.Id, id)
	jsonObj(c, row, err)
}

func (a *SingboxController) add(c *gin.Context) {
	user := session.GetLoginUser(c)
	req, ok := bindSingboxRequest(c)
	if !ok {
		return
	}
	row := req.toModel(user.Id)
	created, err := a.service.AddInbound(row)
	jsonMsgObj(c, "sing-box inbound created", created, err)
}

func (a *SingboxController) update(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	req, ok := bindSingboxRequest(c)
	if !ok {
		return
	}
	updated, err := a.service.UpdateInbound(user.Id, id, req.toModel(user.Id))
	jsonMsgObj(c, "sing-box inbound updated", updated, err)
}

func (a *SingboxController) del(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	err = a.service.DeleteInbound(user.Id, id)
	jsonMsgObj(c, "sing-box inbound deleted", id, err)
}

func (a *SingboxController) setEnable(c *gin.Context) {
	user := session.GetLoginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid id", err)
		return
	}
	var req singboxEnableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid request", err)
		return
	}
	err = a.service.SetEnable(user.Id, id, req.Enable)
	jsonMsg(c, "sing-box inbound state updated", err)
}

func (a *SingboxController) check(c *gin.Context) {
	jsonMsg(c, "sing-box config check", a.service.CheckDatabaseConfig())
}
func (a *SingboxController) restart(c *gin.Context) {
	jsonMsg(c, "sing-box restarted", a.service.Restart())
}
func (a *SingboxController) status(c *gin.Context) {
	status, version, err := a.service.Status()
	jsonObj(c, gin.H{"status": status, "version": version}, err)
}

func bindSingboxRequest(c *gin.Context) (singboxInboundRequest, bool) {
	var req singboxInboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid sing-box inbound request", err)
		return req, false
	}
	if len(req.Settings) == 0 {
		jsonMsg(c, "invalid sing-box inbound request", fmt.Errorf("settings are required"))
		return req, false
	}
	return req, true
}

func (r singboxInboundRequest) toModel(userID int) model.SingboxInbound {
	return model.SingboxInbound{UserId: userID, Remark: r.Remark, Enable: r.Enable, Listen: r.Listen, Port: r.Port, Protocol: r.Protocol, Tag: r.Tag, Settings: string(r.Settings)}
}

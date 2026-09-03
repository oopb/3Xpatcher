package controller

import (
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
	"github.com/gin-gonic/gin"
)

func (a *ServerController) generateSingboxSniCert(c *gin.Context) {
	sni := strings.TrimSpace(c.PostForm("sni"))
	days := 3650
	if raw := strings.TrimSpace(c.PostForm("validityDays")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		} else {
			jsonObj(c, nil, err)
			return
		}
	}
	info, err := singbox.RegenerateSelfSignedCertificate(sni, days)
	jsonObj(c, info, err)
}

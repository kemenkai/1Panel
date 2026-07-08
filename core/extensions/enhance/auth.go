package enhance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/buserr"
	"github.com/1Panel-dev/1Panel/core/constant"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/1Panel-dev/1Panel/core/init/session/psession"
	"github.com/1Panel-dev/1Panel/core/utils/common"
	"github.com/gin-gonic/gin"
)

const (
	simpleNodeLoginPath     = "/api/v2/core/auth/simple-node/login"
	simpleNodeLoginScope    = "1panel-simple-node-login"
	simpleNodeLoginSkew     = time.Minute
	simpleNodeLoginValidity = 5 * time.Minute
)

func SimpleNodeLogin(c *gin.Context) (string, error) {
	if global.Api.ApiInterfaceStatus != constant.StatusEnable {
		return "", buserr.New("ErrApiConfigStatusInvalid")
	}

	apiKey := strings.TrimSpace(global.Api.ApiKey)
	if apiKey == "" {
		return "", buserr.New("ErrApiConfigKeyInvalid")
	}

	timestamp := c.Query("ts")
	signature := c.Query("signature")
	clientIP := strings.TrimSpace(c.Query("client"))
	redirectPath := c.Query("redirect")

	if err := validateSimpleNodeLoginTimestamp(timestamp); err != nil {
		return "", err
	}
	redirectPath, err := normalizeSimpleNodeRedirectPath(redirectPath)
	if err != nil {
		return "", err
	}
	if clientIP == "" || clientIP != common.GetRealClientIP(c) {
		return "", buserr.New("ErrApiConfigIPInvalid")
	}
	if !verifySimpleNodeLoginSignature(apiKey, timestamp, redirectPath, clientIP, signature) {
		return "", buserr.New("ErrApiConfigKeyInvalid")
	}

	settingRepo := repo.NewISettingRepo()
	nameSetting, err := settingRepo.Get(repo.WithByKey("UserName"))
	if err != nil {
		return "", buserr.New("ErrRecordNotFound")
	}
	if err := createSession(c, nameSetting.Value); err != nil {
		return "", err
	}
	if entrance := getSecurityEntrance(settingRepo); entrance != "" {
		setSecurityEntranceCookie(c, entrance, settingRepo)
	}
	return redirectPath, nil
}

func buildSimpleNodeLoginSignature(apiKey, timestamp, redirectPath, clientIP string) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(simpleNodeLoginScope))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(timestamp))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(redirectPath))
	mac.Write([]byte{'\n'})
	mac.Write([]byte(clientIP))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifySimpleNodeLoginSignature(apiKey, timestamp, redirectPath, clientIP, signature string) bool {
	expected := buildSimpleNodeLoginSignature(apiKey, timestamp, redirectPath, clientIP)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

func validateSimpleNodeLoginTimestamp(timestamp string) error {
	loginAt, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return errors.New("invalid login timestamp")
	}
	now := time.Now().Unix()
	if loginAt > now+int64(simpleNodeLoginSkew.Seconds()) {
		return errors.New("login link is not yet valid")
	}
	if now-loginAt > int64((simpleNodeLoginValidity + simpleNodeLoginSkew).Seconds()) {
		return errors.New("login link has expired")
	}
	return nil
}

func normalizeSimpleNodeRedirectPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/", nil
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() || parsed.Host != "" {
		return "", errors.New("invalid redirect path")
	}
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.RequestURI(), nil
}

func createSession(c *gin.Context, name string) error {
	settingRepo := repo.NewISettingRepo()
	timeoutSetting, err := settingRepo.Get(repo.WithByKey("SessionTimeout"))
	if err != nil {
		return err
	}
	sslSetting, err := settingRepo.Get(repo.WithByKey("SSL"))
	if err != nil {
		return err
	}
	lifeTime, err := strconv.Atoi(timeoutSetting.Value)
	if err != nil {
		return err
	}
	sessionUser := psession.SessionUser{Name: name}
	return global.SESSION.SetFresh(c, sessionUser, sslSetting.Value == constant.StatusEnable, lifeTime)
}

func getSecurityEntrance(settingRepo repo.ISettingRepo) string {
	item, err := settingRepo.Get(repo.WithByKey("SecurityEntrance"))
	if err != nil || item.Value == "" {
		return ""
	}
	return item.Value
}

func setSecurityEntranceCookie(c *gin.Context, entrance string, settingRepo repo.ISettingRepo) {
	entranceValue := base64.StdEncoding.EncodeToString([]byte(entrance))
	sslEnabled := false
	if setting, err := settingRepo.Get(repo.WithByKey("SSL")); err == nil {
		sslEnabled = setting.Value == constant.StatusEnable
	}
	c.SetCookie("SecurityEntrance", entranceValue, 0, "/", "", sslEnabled, true)
}

func LoginRedirectPath() string {
	settingRepo := repo.NewISettingRepo()
	loginPath := "/login"
	if entrance := getSecurityEntrance(settingRepo); entrance != "" {
		loginPath = "/" + entrance
	}
	return loginPath
}

func LoginRedirectStatus() int {
	return http.StatusFound
}

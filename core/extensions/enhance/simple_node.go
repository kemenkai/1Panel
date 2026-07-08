package enhance

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/buserr"
	"github.com/1Panel-dev/1Panel/core/constant"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/1Panel-dev/1Panel/core/utils/common"
	"github.com/1Panel-dev/1Panel/core/utils/encrypt"
	"github.com/1Panel-dev/1Panel/core/utils/xpack"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
)

const simpleNodeProbeTimeout = 8 * time.Second

type SimpleNodeCreate struct {
	Name             string `json:"name" validate:"required,max=64"`
	Addr             string `json:"addr" validate:"required,url,max=256"`
	SecurityEntrance string `json:"securityEntrance" validate:"max=256"`
	APIKey           string `json:"apiKey" validate:"max=512"`
	Description      string `json:"description" validate:"max=256"`
}

type SimpleNodeUpdate struct {
	ID uint `json:"id" validate:"required"`
	SimpleNodeCreate
}

type SimpleNodeCheck struct {
	ID uint `json:"id"`
	SimpleNodeCreate
}

type SimpleNodeVisit struct {
	ID       uint   `json:"id" validate:"required"`
	Redirect string `json:"redirect"`
}

type SimpleNodeInfo struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name"`
	Addr              string     `json:"addr"`
	Description       string     `json:"description"`
	SystemVersion     string     `json:"systemVersion"`
	SecurityEntrance  string     `json:"securityEntrance"`
	APIKey            string     `json:"apiKey"`
	CPUUsedPercent    float64    `json:"cpuUsedPercent"`
	CPUTotal          int        `json:"cpuTotal"`
	MemoryTotal       uint64     `json:"memoryTotal"`
	MemoryUsedPercent float64    `json:"memoryUsedPercent"`
	Status            string     `json:"status"`
	Message           string     `json:"message"`
	LastCheckAt       *time.Time `json:"lastCheckAt"`
}

type simpleNodeProbeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type simpleNodeAPIProbeResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func ListSimpleNodes(c *gin.Context) {
	list, err := listSimpleNodes()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, list)
}

func CreateSimpleNode(c *gin.Context) {
	var req SimpleNodeCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := createSimpleNode(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func UpdateSimpleNode(c *gin.Context) {
	var req SimpleNodeUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := updateSimpleNode(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func DeleteSimpleNode(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" validate:"required"`
	}
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := deleteSimpleNodes(req.IDs); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

func CheckSimpleNode(c *gin.Context) {
	var req SimpleNodeCheck
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	item, err := checkSimpleNode(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, item)
}

func RefreshSimpleNode(c *gin.Context) {
	var req struct {
		ID uint `json:"id" validate:"required"`
	}
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	item, err := refreshSimpleNode(req.ID)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, item)
}

func RefreshAllSimpleNodes(c *gin.Context) {
	list, err := refreshAllSimpleNodes()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, list)
}

func BuildSimpleNodeVisitURL(c *gin.Context) {
	var req SimpleNodeVisit
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	visitURL, err := buildVisitURL(req.ID, req.Redirect, common.GetRealClientIP(c))
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, visitURL)
}

func listSimpleNodes() ([]SimpleNodeInfo, error) {
	items, err := repo.NewISimpleNodeRepo().List()
	if err != nil {
		return nil, err
	}
	return toSimpleNodeInfos(items)
}

func createSimpleNode(req SimpleNodeCreate) error {
	payload, err := normalizeCreate(req)
	if err != nil {
		return err
	}
	if err := ensureUnique(0, payload.Name, payload.Addr); err != nil {
		return err
	}
	item, err := probe(payload.Name, payload.Addr, payload.SecurityEntrance, payload.APIKey, payload.Description)
	if err != nil {
		return err
	}
	node := &model.SimpleNode{
		Name:             item.Name,
		Addr:             item.Addr,
		SecurityEntrance: item.SecurityEntrance,
		APIKey:           encryptSimpleNodeAPIKey(payload.APIKey),
		Description:      item.Description,
		Status:           item.Status,
		Message:          item.Message,
		LastCheckAt:      item.LastCheckAt,
	}
	return repo.NewISimpleNodeRepo().Create(context.Background(), node)
}

func updateSimpleNode(req SimpleNodeUpdate) error {
	node, err := repo.NewISimpleNodeRepo().Get(repo.WithByID(req.ID))
	if err != nil {
		return err
	}
	payload, err := normalizeCreate(req.SimpleNodeCreate)
	if err != nil {
		return err
	}
	if err := ensureUnique(req.ID, payload.Name, payload.Addr); err != nil {
		return err
	}
	item, err := probe(payload.Name, payload.Addr, payload.SecurityEntrance, payload.APIKey, payload.Description)
	if err != nil {
		return err
	}
	node.Name = item.Name
	node.Addr = item.Addr
	node.SecurityEntrance = item.SecurityEntrance
	node.APIKey = encryptSimpleNodeAPIKey(payload.APIKey)
	node.Description = item.Description
	node.Status = item.Status
	node.Message = item.Message
	node.LastCheckAt = item.LastCheckAt
	return repo.NewISimpleNodeRepo().Save(context.Background(), &node)
}

func deleteSimpleNodes(ids []uint) error {
	if len(ids) == 0 {
		return errors.New("ids is required")
	}
	return repo.NewISimpleNodeRepo().Delete(context.Background(), repo.WithByIDs(ids))
}

func checkSimpleNode(req SimpleNodeCheck) (*SimpleNodeInfo, error) {
	payload, err := normalizeCreate(req.SimpleNodeCreate)
	if err != nil {
		return nil, err
	}
	if err := ensureUnique(req.ID, payload.Name, payload.Addr); err != nil {
		return nil, err
	}
	return probe(payload.Name, payload.Addr, payload.SecurityEntrance, payload.APIKey, payload.Description)
}

func refreshSimpleNode(id uint) (*SimpleNodeInfo, error) {
	node, err := repo.NewISimpleNodeRepo().Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	item, err := probe(node.Name, node.Addr, node.SecurityEntrance, decryptSimpleNodeAPIKey(node.APIKey), node.Description)
	if err != nil {
		return nil, err
	}
	node.Status = item.Status
	node.Message = item.Message
	node.LastCheckAt = item.LastCheckAt
	if err := repo.NewISimpleNodeRepo().Save(context.Background(), &node); err != nil {
		return nil, err
	}
	item.ID = node.ID
	return item, nil
}

func refreshAllSimpleNodes() ([]SimpleNodeInfo, error) {
	nodes, err := repo.NewISimpleNodeRepo().List()
	if err != nil {
		return nil, err
	}
	items := make([]SimpleNodeInfo, 0, len(nodes))
	for i := range nodes {
		item, err := probe(nodes[i].Name, nodes[i].Addr, nodes[i].SecurityEntrance, decryptSimpleNodeAPIKey(nodes[i].APIKey), nodes[i].Description)
		if err != nil {
			return nil, err
		}
		nodes[i].Status = item.Status
		nodes[i].Message = item.Message
		nodes[i].LastCheckAt = item.LastCheckAt
		if err := repo.NewISimpleNodeRepo().Save(context.Background(), &nodes[i]); err != nil {
			return nil, err
		}
		item.ID = nodes[i].ID
		items = append(items, *item)
	}
	return items, nil
}

func buildVisitURL(id uint, redirectPath, clientIP string) (string, error) {
	node, err := repo.NewISimpleNodeRepo().Get(repo.WithByID(id))
	if err != nil {
		return "", err
	}
	apiKey := decryptSimpleNodeAPIKey(node.APIKey)
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.New("target node API key is empty")
	}
	redirectPath, err = normalizeSimpleNodeRedirectPath(redirectPath)
	if err != nil {
		return "", err
	}
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return "", errors.New("client ip is required")
	}
	loginURL, err := url.Parse(strings.TrimRight(node.Addr, "/") + simpleNodeLoginPath)
	if err != nil {
		return "", err
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	query := loginURL.Query()
	query.Set("ts", timestamp)
	query.Set("redirect", redirectPath)
	query.Set("client", clientIP)
	query.Set("signature", buildSimpleNodeLoginSignature(apiKey, timestamp, redirectPath, clientIP))
	loginURL.RawQuery = query.Encode()
	return loginURL.String(), nil
}

func normalizeCreate(req SimpleNodeCreate) (SimpleNodeCreate, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.SecurityEntrance = normalizeSecurityEntrance(req.SecurityEntrance)
	req.APIKey = strings.TrimSpace(req.APIKey)
	addr, err := normalizeSimpleNodeAddr(req.Addr)
	if err != nil {
		return req, err
	}
	req.Addr = addr
	return req, nil
}

func ensureUnique(id uint, name, addr string) error {
	if exist, _ := repo.NewISimpleNodeRepo().Get(repo.WithByName(name)); exist.ID != 0 && exist.ID != id {
		return buserr.New("ErrRecordExist")
	}
	if exist, _ := repo.NewISimpleNodeRepo().Get(repo.WithByAddr(addr)); exist.ID != 0 && exist.ID != id {
		return buserr.New("ErrRecordExist")
	}
	return nil
}

func probe(name, addr, securityEntrance, apiKey, description string) (*SimpleNodeInfo, error) {
	now := time.Now()
	item := &SimpleNodeInfo{
		Name:             name,
		Addr:             addr,
		Description:      description,
		SecurityEntrance: securityEntrance,
		APIKey:           apiKey,
		Status:           constant.StatusHealthy,
		LastCheckAt:      &now,
	}
	var err error
	if apiKey != "" {
		err = probeSimpleNodeRemoteWithAPIKey(addr, apiKey)
	} else {
		err = probeSimpleNodeRemote(addr)
	}
	if err != nil {
		item.Status = constant.StatusUnhealthy
		item.Message = err.Error()
	}
	return item, nil
}

func probeSimpleNodeRemoteWithAPIKey(addr, apiKey string) error {
	endpoint := strings.TrimRight(addr, "/") + "/api/v2/dashboard/base/os"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	req.Header.Set("1Panel-Timestamp", timestamp)
	req.Header.Set("1Panel-Token", generate1PanelToken(apiKey, timestamp))
	req.Header.Set("CurrentNode", "local")

	client := &http.Client{
		Timeout:   simpleNodeProbeTimeout,
		Transport: xpack.LoadRequestTransport(),
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return probeSimpleNodeRemote(addr)
	}

	var result simpleNodeAPIProbeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return probeSimpleNodeRemote(addr)
	}
	if result.Code != http.StatusOK {
		if err := probeSimpleNodeRemote(addr); err == nil {
			return nil
		}
		if strings.TrimSpace(result.Message) != "" {
			return errors.New(result.Message)
		}
		return fmt.Errorf("target panel returned code %d", result.Code)
	}
	return nil
}

func probeSimpleNodeRemote(addr string) error {
	endpoint := strings.TrimRight(addr, "/") + "/api/v2/core/auth/setting"
	client := &http.Client{
		Timeout:   simpleNodeProbeTimeout,
		Transport: xpack.LoadRequestTransport(),
	}
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var result simpleNodeProbeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return errors.New("target did not return a valid 1Panel JSON response; check domain binding or IP whitelist")
	}
	if result.Code != http.StatusOK {
		if strings.TrimSpace(result.Message) != "" {
			return errors.New(result.Message)
		}
		return fmt.Errorf("target panel returned code %d", result.Code)
	}
	return nil
}

func normalizeSimpleNodeAddr(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid panel address")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func normalizeSecurityEntrance(entrance string) string {
	entrance = strings.TrimSpace(entrance)
	entrance = strings.Trim(entrance, "/")
	return entrance
}

func toSimpleNodeInfos(items []model.SimpleNode) ([]SimpleNodeInfo, error) {
	result := make([]SimpleNodeInfo, 0, len(items))
	for i := range items {
		var item SimpleNodeInfo
		if err := copier.Copy(&item, &items[i]); err != nil {
			return nil, buserr.WithDetail("ErrStructTransform", err.Error(), nil)
		}
		item.APIKey = decryptSimpleNodeAPIKey(items[i].APIKey)
		result = append(result, item)
	}
	return result, nil
}

func generate1PanelToken(apiKey, timestamp string) string {
	hash := md5.New()
	hash.Write([]byte("1panel" + apiKey + timestamp))
	return hex.EncodeToString(hash.Sum(nil))
}

func encryptSimpleNodeAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	encrypted, err := encrypt.StringEncrypt(value)
	if err != nil {
		global.LOG.Warnf("encrypt simple node api key failed: %v", err)
		return value
	}
	return encrypted
}

func decryptSimpleNodeAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decrypted, err := encrypt.StringDecrypt(value)
	if err != nil {
		return value
	}
	return decrypted
}

package v2

import (
	"github.com/1Panel-dev/1Panel/core/extensions/enhance"
	"github.com/gin-gonic/gin"
)

func (b *BaseApi) ListAllSimpleNodes(c *gin.Context) {
	enhance.ListSimpleNodes(c)
}

func (b *BaseApi) CreateSimpleNode(c *gin.Context) {
	enhance.CreateSimpleNode(c)
}

func (b *BaseApi) UpdateSimpleNode(c *gin.Context) {
	enhance.UpdateSimpleNode(c)
}

func (b *BaseApi) DeleteSimpleNode(c *gin.Context) {
	enhance.DeleteSimpleNode(c)
}

func (b *BaseApi) CheckSimpleNode(c *gin.Context) {
	enhance.CheckSimpleNode(c)
}

func (b *BaseApi) RefreshSimpleNode(c *gin.Context) {
	enhance.RefreshSimpleNode(c)
}

func (b *BaseApi) RefreshAllSimpleNodes(c *gin.Context) {
	enhance.RefreshAllSimpleNodes(c)
}

func (b *BaseApi) BuildSimpleNodeVisitURL(c *gin.Context) {
	enhance.BuildSimpleNodeVisitURL(c)
}

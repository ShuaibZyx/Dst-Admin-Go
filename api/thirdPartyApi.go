package api

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ThirdPartyApi struct {
}

// 获取饥荒的版本号
func (t *ThirdPartyApi) GetDstVersion(c *gin.Context) {

	url := "http://ver.tugos.cn/getLocalVersion"
	response, err := http.Get(url)
	if err != nil || response.StatusCode != http.StatusOK {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	reader := response.Body
	contentLength := response.ContentLength
	contentType := response.Header.Get("Content-Type")

	extraHeaders := map[string]string{
		//"Content-Disposition": `attachment; filename="gopher.png"`,
	}

	c.DataFromReader(http.StatusOK, contentLength, contentType, reader, extraHeaders)
}

// 获取第三方饥荒服务器
func (t *ThirdPartyApi) GiteeProxy(c *gin.Context) {
	// 获取路径参数
	filePath := c.Param("filepath")

	// 处理路径前缀的斜杠（Gin 的路径参数会保留斜杠）
	if len(filePath) > 0 && filePath[0] == '/' {
		filePath = filePath[1:]
	}

	// 构建目标 URL
	targetURL := "https://gitee.com/hhhuhu23/dst-static/raw/master/" + filePath

	// 创建带超时的 HTTP 客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建代理请求
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create request",
		})
		return
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			// 排除 Host 头，避免影响转发
			if key == "Host" || key == "Referer" {
				continue
			}
			req.Header.Add(key, value)
		}
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"error": "Failed to fetch resource",
		})
		return
	}
	defer resp.Body.Close()

	// 设置 CORS 头
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Origin, Content-Type")

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 设置状态码
	c.Status(resp.StatusCode)

	// 复制响应体
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to stream response",
		})
	}
}

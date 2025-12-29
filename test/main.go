package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

type FileNode struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "file" or "folder"
	Path     string      `json:"path"`
	Content  string      `json:"content,omitempty"`
	Children []FileNode  `json:"children,omitempty"`
}

var idCounter = 0

// 生成唯一ID
func generateID() string {
	idCounter++
	return strconv.Itoa(idCounter)
}

// 递归读取文件夹结构
func buildFileTree(path string, rootPath string) (*FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// 计算相对路径
	relativePath, err := filepath.Rel(rootPath, path)
	if err != nil {
		relativePath = path
	}
	if relativePath == "." {
		relativePath = ""
	}

	node := &FileNode{
		ID:   generateID(),
		Name: info.Name(),
		Path: "/" + filepath.ToSlash(relativePath),
	}

	if info.IsDir() {
		node.Type = "folder"
		
		// 读取文件夹内容
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}

		node.Children = []FileNode{}
		for _, file := range files {
			childPath := filepath.Join(path, file.Name())
			childNode, err := buildFileTree(childPath, rootPath)
			if err != nil {
				continue // 跳过无法读取的文件
			}
			node.Children = append(node.Children, *childNode)
		}
	} else {
		node.Type = "file"
		
		// 读取文件内容
		content, err := ioutil.ReadFile(path)
		if err == nil {
			node.Content = string(content)
		}
	}

	return node, nil
}

func main() {
	r := gin.Default()

	// CORS 中间件(如果需要跨域访问)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 获取文件树的API
	r.GET("/api/files", func(c *gin.Context) {
		// 获取查询参数中的路径,默认为当前目录
		targetPath := c.DefaultQuery("path", ".")
		
		// 重置ID计数器
		idCounter = 0
		
		// 获取绝对路径
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "无效的路径",
			})
			return
		}

		// 构建文件树
		fileTree, err := buildFileTree(absPath, absPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, fileTree)
	})

	// 健康检查接口
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// 启动服务
	port := ":8080"
	println("Server is running on http://localhost" + port)
	println("访问 http://localhost" + port + "/api/files 获取当前目录文件树")
	println("访问 http://localhost" + port + "/api/files?path=/your/path 获取指定目录文件树")
	
	r.Run(port)
}
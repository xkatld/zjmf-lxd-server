package services

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"lxdweb/database"
	"lxdweb/models"
	"net/http"
	"sync"
	"time"
	
	"gorm.io/gorm/clause"
)

func StartNodeCacheService() {
	log.Println("[NODE-CACHE] 节点信息缓存服务启动")

	go func() {
		refreshAllNodeCache()
	}()

	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			refreshAllNodeCache()
		}
	}()
}

func refreshAllNodeCache() {
	var nodes []models.Node
	database.DB.Find(&nodes)
	
	if len(nodes) == 0 {
		return
	}
	
	log.Printf("[NODE-CACHE] 开始刷新 %d 个节点缓存", len(nodes))
	
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) 
	
	for _, node := range nodes {
		wg.Add(1)
		go func(n models.Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			
			cacheNodeInfo(n)
		}(node)
	}
	
	wg.Wait()
	log.Println("[NODE-CACHE] 缓存刷新完成")
}

func cacheNodeInfo(node models.Node) {
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	
	req, err := http.NewRequest("GET", node.Address+"/", nil)
	if err != nil {
		log.Printf("[NODE-CACHE] 节点 %s 创建请求失败: %v", node.Name, err)
		clearNodeCache(node.ID)
		return
	}
	
	if node.APIKey != "" {
		req.Header.Set("apikey", node.APIKey)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[NODE-CACHE] 节点 %s 连接失败: %v", node.Name, err)
		clearNodeCache(node.ID)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		log.Printf("[NODE-CACHE] 节点 %s 返回状态码: %d", node.Name, resp.StatusCode)
		clearNodeCache(node.ID)
		return
	}
	
	var sysInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sysInfo); err != nil {
		log.Printf("[NODE-CACHE] 节点 %s 解析响应失败: %v", node.Name, err)
		clearNodeCache(node.ID)
		return
	}

	sysInfoJSON, err := json.Marshal(sysInfo)
	if err != nil {
		log.Printf("[NODE-CACHE] 节点 %s 序列化失败: %v", node.Name, err)
		return
	}

	cache := models.NodeInfoCache{
		NodeID:     node.ID,
		SystemInfo: string(sysInfoJSON),
		LastSync:   time.Now(),
	}
	
	result := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"system_info", "last_sync"}),
	}).Create(&cache)
	
	if result.Error != nil {
		log.Printf("[NODE-CACHE] 节点 %s 保存缓存失败: %v", node.Name, result.Error)
	} else {
		log.Printf("[NODE-CACHE] 节点 %s 缓存成功", node.Name)
	}
}

func clearNodeCache(nodeID uint) {
	database.DB.Unscoped().Where("node_id = ?", nodeID).Delete(&models.NodeInfoCache{})
}

func GetNodeCache(nodeID uint) (map[string]interface{}, error) {
	var cache models.NodeInfoCache
	if err := database.DB.Where("node_id = ?", nodeID).First(&cache).Error; err != nil {
		return nil, err
	}
	
	var sysInfo map[string]interface{}
	if err := json.Unmarshal([]byte(cache.SystemInfo), &sysInfo); err != nil {
		return nil, err
	}
	
	return sysInfo, nil
}

func RefreshNodeCache(nodeID uint) error {
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return err
	}
	
	cacheNodeInfo(node)
	return nil
}


package services

import (
	"log"
	"lxdweb/database"
	"lxdweb/models"
	"sync"
	"time"
)

var (
	autoSyncEnabled bool
	autoSyncMutex   sync.Mutex
	stopChan        chan bool
)

func StartAutoSyncService() {
	log.Println("[AUTO-SYNC] 自动同步服务启动")
	autoSyncEnabled = true
	stopChan = make(chan bool)
	
	go autoSyncLoop()
}

func autoSyncLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-stopChan:
			log.Println("[AUTO-SYNC] 自动同步服务已停止")
			return
		case <-ticker.C:
			checkAndSyncNodes()
		}
	}
}

func checkAndSyncNodes() {
	var nodes []models.Node
	if err := database.DB.Where("status = ? AND auto_sync = ?", "active", true).Find(&nodes).Error; err != nil {
		log.Printf("[AUTO-SYNC] 查询节点失败: %v", err)
		return
	}
	
	if len(nodes) == 0 {
		return
	}
	
	now := time.Now()
	for _, node := range nodes {
		var lastTask models.SyncTask
		database.DB.Where("node_id = ?", node.ID).Order("created_at DESC").First(&lastTask)
		
		shouldSync := false
		if lastTask.ID == 0 {
			shouldSync = true
		} else if lastTask.StartTime != nil {
			elapsed := now.Sub(*lastTask.StartTime).Seconds()
			if elapsed >= float64(node.SyncInterval) {
				shouldSync = true
			}
		}
		
		if shouldSync {
			log.Printf("[AUTO-SYNC] 触发节点 %s (ID: %d) 自动同步", node.Name, node.ID)
			go SyncNodeContainers(node.ID, false)
		}
	}
}

func EnableAutoSync() {
	autoSyncMutex.Lock()
	defer autoSyncMutex.Unlock()
	
	if !autoSyncEnabled {
		autoSyncEnabled = true
		stopChan = make(chan bool)
		go autoSyncLoop()
		log.Println("[AUTO-SYNC] 自动同步已启用")
	}
}

func DisableAutoSync() {
	autoSyncMutex.Lock()
	defer autoSyncMutex.Unlock()
	
	if autoSyncEnabled {
		autoSyncEnabled = false
		close(stopChan)
		log.Println("[AUTO-SYNC] 自动同步已禁用")
	}
}

func IsAutoSyncEnabled() bool {
	autoSyncMutex.Lock()
	defer autoSyncMutex.Unlock()
	return autoSyncEnabled
}

// SyncAllNodesFullAsync 完整同步所有节点的所有数据
func SyncAllNodesFullAsync() {
	log.Println("[AUTO-SYNC] 开始执行完整实时同步任务")

	var nodes []models.Node
	if err := database.DB.Where("status = ?", "active").Find(&nodes).Error; err != nil {
		log.Printf("[AUTO-SYNC] 查询节点失败: %v", err)
		return
	}

	if len(nodes) == 0 {
		log.Println("[AUTO-SYNC] 没有活跃的节点需要同步")
		return
	}

	log.Printf("[AUTO-SYNC] 找到 %d 个活跃节点，开始实时同步", len(nodes))

	for i, node := range nodes {
		log.Printf("[AUTO-SYNC] 处理节点 %d/%d: %s", i+1, len(nodes), node.Name)
		syncNodeFull(node)

		if i < len(nodes)-1 {
			interval := time.Duration(node.BatchInterval) * time.Second
			log.Printf("[AUTO-SYNC] 等待 %v 后处理下一个节点", interval)
			time.Sleep(interval)
		}
	}

	log.Println("[AUTO-SYNC] 所有节点完整同步任务完成")
}

// syncNodeFull 完整同步单个节点的所有数据类型
func syncNodeFull(n models.Node) {
	log.Printf("[AUTO-SYNC] 实时同步节点: %s (ID: %d)", n.Name, n.ID)

	if err := SyncNodeContainers(n.ID, false); err != nil {
		log.Printf("[AUTO-SYNC] 节点 %s 容器同步失败: %v", n.Name, err)
	}

	log.Printf("[AUTO-SYNC] 节点 %s 同步完成", n.Name)
}


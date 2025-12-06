package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/cofedish/3x-UI-agents/logger"
	"github.com/cofedish/3x-UI-agents/web/global"
	"github.com/cofedish/3x-UI-agents/web/service"

	"github.com/gin-gonic/gin"
)

var filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)

// ServerController handles server management and status-related operations.
type ServerController struct {
	BaseController

	serverService  service.ServerService
	settingService service.SettingService
	serverMgmt     *service.ServerManagementService

	lastStatus *service.Status

	lastVersions        []string
	lastGetVersionsTime int64 // unix seconds
}

// NewServerController creates a new ServerController, initializes routes, and starts background tasks.
func NewServerController(g *gin.RouterGroup) *ServerController {
	a := &ServerController{
		serverMgmt: &service.ServerManagementService{},
	}
	a.initRouter(g)
	a.startTask()
	return a
}

// initRouter sets up the routes for server status, Xray management, and utility endpoints.
func (a *ServerController) initRouter(g *gin.RouterGroup) {

	g.GET("/status", a.status)
	g.GET("/aggregatedStatus", a.aggregatedStatus)
	g.GET("/cpuHistory/:bucket", a.getCpuHistoryBucket)
	g.GET("/getXrayVersion", a.getXrayVersion)
	g.GET("/getConfigJson", a.getConfigJson)
	g.GET("/getDb", a.getDb)
	g.GET("/getNewUUID", a.getNewUUID)
	g.GET("/getNewX25519Cert", a.getNewX25519Cert)
	g.GET("/getNewmldsa65", a.getNewmldsa65)
	g.GET("/getNewmlkem768", a.getNewmlkem768)
	g.GET("/getNewVlessEnc", a.getNewVlessEnc)

	g.POST("/stopXrayService", a.stopXrayService)
	g.POST("/restartXrayService", a.restartXrayService)
	g.POST("/installXray/:version", a.installXray)
	g.POST("/updateGeofile", a.updateGeofile)
	g.POST("/updateGeofile/:fileName", a.updateGeofile)
	g.POST("/logs/:count", a.getLogs)
	g.POST("/xraylogs/:count", a.getXrayLogs)
	g.POST("/importDB", a.importDB)
	g.POST("/getNewEchCert", a.getNewEchCert)
}

// refreshStatus updates the cached server status and collects CPU history.
func (a *ServerController) refreshStatus() {
	a.lastStatus = a.serverService.GetStatus(a.lastStatus)
	// collect cpu history when status is fresh
	if a.lastStatus != nil {
		a.serverService.AppendCpuSample(time.Now(), a.lastStatus.Cpu)
	}
}

// startTask initiates background tasks for continuous status monitoring.
func (a *ServerController) startTask() {
	webServer := global.GetWebServer()
	c := webServer.GetCron()
	c.AddFunc("@every 2s", func() {
		// Always refresh to keep CPU history collected continuously.
		// Sampling is lightweight and capped to ~6 hours in memory.
		a.refreshStatus()
	})
}

// getServerIdFromRequest extracts server_id from query parameter, defaults to 1 for backward compatibility.
func (a *ServerController) getServerIdFromRequest(c *gin.Context) int {
	serverIdStr := c.DefaultQuery("server_id", "1")
	serverId, err := strconv.Atoi(serverIdStr)
	if err != nil || serverId < 1 {
		return 1
	}
	return serverId
}

// convertSystemStatsToStatus converts SystemStats (remote) to Status (local) format for frontend compatibility.
func convertSystemStatsToStatus(stats *service.SystemStats, health *service.HealthStatus) map[string]interface{} {
	// Parse load average string to array
	loads := []float64{0, 0, 0}
	if stats.LoadAverage != "" {
		fmt.Sscanf(stats.LoadAverage, "%f, %f, %f", &loads[0], &loads[1], &loads[2])
	}

	// Determine Xray state from health
	xrayState := "stop"
	xrayVersion := "Unknown"
	if health != nil {
		if health.XrayRunning {
			xrayState = "running"
		}
		xrayVersion = health.XrayVersion
	}

	return map[string]interface{}{
		"cpu":         stats.CPUUsage,
		"cpuCores":    stats.CPUCores,
		"logicalPro":  0, // Not available from agent
		"cpuSpeedMhz": 0, // Not available from agent
		"mem": map[string]uint64{
			"current": stats.MemUsed,
			"total":   stats.MemTotal,
		},
		"swap": map[string]uint64{
			"current": stats.SwapUsed,
			"total":   stats.SwapTotal,
		},
		"disk": map[string]uint64{
			"current": stats.DiskUsed,
			"total":   stats.DiskTotal,
		},
		"netIO": map[string]int64{
			"up":   stats.NetOutSpeed,
			"down": stats.NetInSpeed,
		},
		"netTraffic": map[string]uint64{
			"sent": 0, // Not available from agent stats
			"recv": 0, // Not available from agent stats
		},
		"publicIP": map[string]string{
			"ipv4": stats.PublicIPv4,
			"ipv6": stats.PublicIPv6,
		},
		"uptime":    stats.Uptime,
		"appUptime": 0, // Not available from agent
		"appStats": map[string]interface{}{
			"threads": 0,
			"mem":     0,
			"uptime":  0,
		},
		"loads":    loads,
		"tcpCount": stats.TCPConnections,
		"udpCount": stats.UDPConnections,
		"xray": map[string]interface{}{
			"state":    xrayState,
			"version":  xrayVersion,
			"errorMsg": "",
		},
	}
}

// status returns the current server status information.
// Supports optional server_id query parameter for multi-server mode.
func (a *ServerController) status(c *gin.Context) {
	serverId := a.getServerIdFromRequest(c)

	// For backward compatibility, use local cache if server_id=1
	if serverId == 1 {
		jsonObj(c, a.lastStatus, nil)
		return
	}

	// Multi-server mode: use connector to get fresh status
	connector, err := a.serverMgmt.GetConnector(serverId)
	if err != nil {
		jsonMsg(c, "Failed to connect to server", err)
		return
	}

	stats, err := connector.GetSystemStats(c.Request.Context())
	if err != nil {
		jsonMsg(c, "Failed to get server status", err)
		return
	}

	// Get health info for Xray state
	health, err := connector.GetHealth(c.Request.Context())
	if err != nil {
		// Log error but continue with nil health (Xray will show as stopped)
		logger.Warning("Failed to get health info:", err)
		health = nil
	}

	// Convert SystemStats to Status format for frontend compatibility
	statusMap := convertSystemStatsToStatus(stats, health)
	jsonObj(c, statusMap, nil)
}

// aggregatedStatus returns aggregated status across all servers (local + remote).
// This endpoint is used when server_id=0 ("All Servers" view in UI).
func (a *ServerController) aggregatedStatus(c *gin.Context) {
	type AggregatedStats struct {
		TotalServers   int     `json:"totalServers"`
		OnlineServers  int     `json:"onlineServers"`
		OfflineServers int     `json:"offlineServers"`
		AvgCpu         float64 `json:"cpu"`         // Average CPU percentage
		TotalCpuCores  int     `json:"cpuCores"`    // Total CPU cores across all servers
		TotalMemory    uint64  `json:"totalMemory"` // Total memory across all servers
		UsedMemory     uint64  `json:"usedMemory"`  // Total used memory
		TotalDisk      uint64  `json:"totalDisk"`   // Total disk space
		UsedDisk       uint64  `json:"usedDisk"`    // Total used disk
		TotalUpload    uint64  `json:"totalUp"`     // Total upload traffic
		TotalDownload  uint64  `json:"totalDown"`   // Total download traffic
		NetUpSpeed     int64   `json:"netUpSpeed"`  // Total upload speed (bytes/sec)
		NetDownSpeed   int64   `json:"netDownSpeed"` // Total download speed (bytes/sec)
		TotalTCP       int     `json:"totalTCP"`    // Total TCP connections
		TotalUDP       int     `json:"totalUDP"`    // Total UDP connections
		PublicIPv4     string  `json:"publicIPv4"`  // First available public IPv4
		PublicIPv6     string  `json:"publicIPv6"`  // First available public IPv6
		XrayRunning    int     `json:"xrayRunning"` // Count of servers with Xray running
		XrayStopped    int     `json:"xrayStopped"` // Count of servers with Xray stopped
		XrayError      int     `json:"xrayError"`   // Count of servers with Xray errors
	}

	aggregated := &AggregatedStats{}

	// Get all servers
	servers, err := a.serverMgmt.GetAllServers()
	if err != nil {
		jsonMsg(c, "Failed to get servers", err)
		return
	}

	// Include local server (id=1)
	aggregated.TotalServers = len(servers) + 1

	// Bounded concurrency for collecting stats
	maxConcurrency := 10
	sem := make(chan struct{}, maxConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Debug: track which servers contributed to aggregation
	type ServerDebug struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		CPUCores int    `json:"cpuCores"`
		MemGB    string `json:"memGB"`
		DiskGB   string `json:"diskGB"`
	}
	var debugServers []ServerDebug

	// Helper to aggregate stats
	aggregateStats := func(serverID int, serverName string, stats interface{}, health *service.HealthStatus) {
		mu.Lock()
		defer mu.Unlock()

		aggregated.OnlineServers++

		// Handle local server stats (service.Status)
		if status, ok := stats.(*service.Status); ok {
			aggregated.AvgCpu += status.Cpu
			aggregated.TotalCpuCores += status.CpuCores
			aggregated.TotalMemory += status.Mem.Total
			aggregated.UsedMemory += status.Mem.Current
			aggregated.TotalDisk += status.Disk.Total
			aggregated.UsedDisk += status.Disk.Current
			aggregated.TotalUpload += status.NetTraffic.Sent
			aggregated.TotalDownload += status.NetTraffic.Recv
			aggregated.NetUpSpeed += int64(status.NetIO.Up)
			aggregated.NetDownSpeed += int64(status.NetIO.Down)
			aggregated.TotalTCP += status.TcpCount
			aggregated.TotalUDP += status.UdpCount

			// Collect first available public IPs
			if aggregated.PublicIPv4 == "" && status.PublicIP.IPv4 != "" && status.PublicIP.IPv4 != "0.0.0.0" {
				aggregated.PublicIPv4 = status.PublicIP.IPv4
			}
			if aggregated.PublicIPv6 == "" && status.PublicIP.IPv6 != "" && status.PublicIP.IPv6 != "::" {
				aggregated.PublicIPv6 = status.PublicIP.IPv6
			}

			// Debug info
			debugServers = append(debugServers, ServerDebug{
				ID:       serverID,
				Name:     serverName,
				CPUCores: status.CpuCores,
				MemGB:    fmt.Sprintf("%.2f", float64(status.Mem.Total)/(1024*1024*1024)),
				DiskGB:   fmt.Sprintf("%.2f", float64(status.Disk.Total)/(1024*1024*1024)),
			})

			// Aggregate Xray status
			switch status.Xray.State {
			case "running":
				aggregated.XrayRunning++
			case "stop":
				aggregated.XrayStopped++
			case "error":
				aggregated.XrayError++
			}
			return
		}

		// Handle remote server stats (service.SystemStats)
		if sysStats, ok := stats.(*service.SystemStats); ok {
			aggregated.AvgCpu += sysStats.CPUUsage
			aggregated.TotalCpuCores += sysStats.CPUCores
			aggregated.TotalMemory += sysStats.MemTotal
			aggregated.UsedMemory += sysStats.MemUsed
			aggregated.TotalDisk += sysStats.DiskTotal
			aggregated.UsedDisk += sysStats.DiskUsed
			aggregated.NetUpSpeed += sysStats.NetOutSpeed
			aggregated.NetDownSpeed += sysStats.NetInSpeed
			aggregated.TotalTCP += sysStats.TCPConnections
			aggregated.TotalUDP += sysStats.UDPConnections

			// Collect first available public IPs
			if aggregated.PublicIPv4 == "" && sysStats.PublicIPv4 != "" && sysStats.PublicIPv4 != "0.0.0.0" {
				aggregated.PublicIPv4 = sysStats.PublicIPv4
			}
			if aggregated.PublicIPv6 == "" && sysStats.PublicIPv6 != "" && sysStats.PublicIPv6 != "::" {
				aggregated.PublicIPv6 = sysStats.PublicIPv6
			}

			// Debug info
			debugServers = append(debugServers, ServerDebug{
				ID:       serverID,
				Name:     serverName,
				CPUCores: sysStats.CPUCores,
				MemGB:    fmt.Sprintf("%.2f", float64(sysStats.MemTotal)/(1024*1024*1024)),
				DiskGB:   fmt.Sprintf("%.2f", float64(sysStats.DiskTotal)/(1024*1024*1024)),
			})

			// Aggregate Xray status from health
			if health != nil {
				if health.XrayRunning {
					aggregated.XrayRunning++
				} else {
					aggregated.XrayStopped++
				}
			}
			return
		}
	}

	// Collect local server stats
	if a.lastStatus != nil {
		aggregateStats(1, "Local Server", a.lastStatus, nil) // Local status already includes Xray state
	} else {
		aggregated.OfflineServers++
	}

	// Collect remote server stats concurrently
	for _, server := range servers {
		wg.Add(1)
		server := server // Capture loop variable

		go func() {
			defer wg.Done()

			// Acquire semaphore slot
			sem <- struct{}{}
			defer func() { <-sem }()

			// Skip local server (id=1) as it's already processed via lastStatus
			if server.Id == 1 {
				return
			}

			// Skip disabled servers
			if !server.Enabled {
				mu.Lock()
				aggregated.OfflineServers++
				mu.Unlock()
				return
			}

			// Get connector and fetch stats
			connector, err := a.serverMgmt.GetConnector(server.Id)
			if err != nil {
				mu.Lock()
				aggregated.OfflineServers++
				mu.Unlock()
				return
			}

			ctx := c.Request.Context()
			stats, err := connector.GetSystemStats(ctx)
			if err != nil {
				mu.Lock()
				aggregated.OfflineServers++
				mu.Unlock()
				return
			}

			// Get health status for Xray state
			health, _ := connector.GetHealth(ctx)

			aggregateStats(server.Id, server.Name, stats, health)
		}()
	}

	wg.Wait()

	// Calculate average CPU
	if aggregated.OnlineServers > 0 {
		aggregated.AvgCpu = aggregated.AvgCpu / float64(aggregated.OnlineServers)
	}

	// Convert to Status-compatible format for frontend
	statusFormat := map[string]interface{}{
		"cpu":         aggregated.AvgCpu,
		"cpuCores":    aggregated.TotalCpuCores,
		"logicalPro":  0,
		"cpuSpeedMhz": 0,
		"mem": map[string]uint64{
			"current": aggregated.UsedMemory,
			"total":   aggregated.TotalMemory,
		},
		"swap": map[string]uint64{
			"current": 0,
			"total":   0,
		},
		"disk": map[string]uint64{
			"current": aggregated.UsedDisk,
			"total":   aggregated.TotalDisk,
		},
		"xray": map[string]interface{}{
			"state":    "unknown",
			"errorMsg": "",
			"version":  "",
		},
		"uptime": uint64(0),
		"loads":  []float64{0, 0, 0},
		"tcpCount": aggregated.TotalTCP,
		"udpCount": aggregated.TotalUDP,
		"netIO": map[string]int64{
			"up":   aggregated.NetUpSpeed,
			"down": aggregated.NetDownSpeed,
		},
		"netTraffic": map[string]uint64{
			"sent": aggregated.TotalUpload,
			"recv": aggregated.TotalDownload,
		},
		"publicIP": map[string]string{
			"ipv4": aggregated.PublicIPv4,
			"ipv6": aggregated.PublicIPv6,
		},
		"appStats": map[string]interface{}{
			"threads": 0,
			"mem":     0,
			"uptime":  0,
		},
		// Add aggregated metadata
		"_aggregated": map[string]interface{}{
			"totalServers":   aggregated.TotalServers,
			"onlineServers":  aggregated.OnlineServers,
			"offlineServers": aggregated.OfflineServers,
			"xrayRunning":    aggregated.XrayRunning,
			"xrayStopped":    aggregated.XrayStopped,
			"xrayError":      aggregated.XrayError,
			"servers":        debugServers, // Debug: show which servers contributed
		},
	}

	// Determine aggregated Xray state
	if aggregated.XrayRunning > 0 {
		statusFormat["xray"].(map[string]interface{})["state"] = "running"
	} else if aggregated.XrayStopped > 0 {
		statusFormat["xray"].(map[string]interface{})["state"] = "stop"
	} else if aggregated.XrayError > 0 {
		statusFormat["xray"].(map[string]interface{})["state"] = "error"
	}

	jsonObj(c, statusFormat, nil)
}

// getCpuHistoryBucket retrieves aggregated CPU usage history based on the specified time bucket.
func (a *ServerController) getCpuHistoryBucket(c *gin.Context) {
	bucketStr := c.Param("bucket")
	bucket, err := strconv.Atoi(bucketStr)
	if err != nil || bucket <= 0 {
		jsonMsg(c, "invalid bucket", fmt.Errorf("bad bucket"))
		return
	}
	allowed := map[int]bool{
		2:   true, // Real-time view
		30:  true, // 30s intervals
		60:  true, // 1m intervals
		120: true, // 2m intervals
		180: true, // 3m intervals
		300: true, // 5m intervals
	}
	if !allowed[bucket] {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	points := a.serverService.AggregateCpuHistory(bucket, 60)
	jsonObj(c, points, nil)
}

// getXrayVersion retrieves available Xray versions, with caching for 1 minute.
func (a *ServerController) getXrayVersion(c *gin.Context) {
	now := time.Now().Unix()
	if now-a.lastGetVersionsTime <= 60 { // 1 minute cache
		jsonObj(c, a.lastVersions, nil)
		return
	}

	versions, err := a.serverService.GetXrayVersions()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "getVersion"), err)
		return
	}

	a.lastVersions = versions
	a.lastGetVersionsTime = now

	jsonObj(c, versions, nil)
}

// installXray installs or updates Xray to the specified version.
func (a *ServerController) installXray(c *gin.Context) {
	version := c.Param("version")
	err := a.serverService.UpdateXray(version)
	jsonMsg(c, I18nWeb(c, "pages.index.xraySwitchVersionPopover"), err)
}

// updateGeofile updates the specified geo file for Xray.
// Supports optional server_id query parameter for multi-server mode.
func (a *ServerController) updateGeofile(c *gin.Context) {
	fileName := c.Param("fileName")

	// Validate the filename for security (prevent path traversal attacks)
	if fileName != "" && !a.serverService.IsValidGeofileName(fileName) {
		jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"),
			fmt.Errorf("invalid filename: contains unsafe characters or path traversal patterns"))
		return
	}

	serverId := a.getServerIdFromRequest(c)

	// For backward compatibility, use local service if server_id=1
	if serverId == 1 {
		err := a.serverService.UpdateGeofile(fileName)
		jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"), err)
		return
	}

	// Multi-server mode: use connector
	connector, err := a.serverMgmt.GetConnector(serverId)
	if err != nil {
		jsonMsg(c, "Failed to connect to server", err)
		return
	}

	err = connector.UpdateGeoFiles(c.Request.Context())
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"), nil)
}

// stopXrayService stops the Xray service.
// Supports optional server_id query parameter for multi-server mode.
func (a *ServerController) stopXrayService(c *gin.Context) {
	serverId := a.getServerIdFromRequest(c)

	// For backward compatibility, use local service if server_id=1
	if serverId == 1 {
		err := a.serverService.StopXrayService()
		if err != nil {
			jsonMsg(c, I18nWeb(c, "pages.xray.stopError"), err)
			return
		}
		jsonMsg(c, I18nWeb(c, "pages.xray.stopSuccess"), err)
		return
	}

	// Multi-server mode: use connector
	connector, err := a.serverMgmt.GetConnector(serverId)
	if err != nil {
		jsonMsg(c, "Failed to connect to server", err)
		return
	}

	err = connector.StopXray(c.Request.Context())
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.xray.stopError"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.stopSuccess"), nil)
}

// restartXrayService restarts the Xray service.
// Supports optional server_id query parameter for multi-server mode.
func (a *ServerController) restartXrayService(c *gin.Context) {
	serverId := a.getServerIdFromRequest(c)

	// For backward compatibility, use local service if server_id=1
	if serverId == 1 {
		err := a.serverService.RestartXrayService()
		if err != nil {
			jsonMsg(c, I18nWeb(c, "pages.xray.restartError"), err)
			return
		}
		jsonMsg(c, I18nWeb(c, "pages.xray.restartSuccess"), err)
		return
	}

	// Multi-server mode: use connector
	connector, err := a.serverMgmt.GetConnector(serverId)
	if err != nil {
		jsonMsg(c, "Failed to connect to server", err)
		return
	}

	err = connector.RestartXray(c.Request.Context())
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.xray.restartError"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.restartSuccess"), nil)
}

// getLogs retrieves the application logs based on count, level, and syslog filters.
// Supports optional server_id query parameter for multi-server mode.
func (a *ServerController) getLogs(c *gin.Context) {
	count := c.Param("count")
	serverId := a.getServerIdFromRequest(c)

	// For backward compatibility, use local service if server_id=1
	if serverId == 1 {
		level := c.PostForm("level")
		syslog := c.PostForm("syslog")
		logs := a.serverService.GetLogs(count, level, syslog)
		jsonObj(c, logs, nil)
		return
	}

	// Multi-server mode: use connector
	connector, err := a.serverMgmt.GetConnector(serverId)
	if err != nil {
		jsonMsg(c, "Failed to connect to server", err)
		return
	}

	// Convert count to integer
	countInt, err := strconv.Atoi(count)
	if err != nil {
		countInt = 100
	}

	logs, err := connector.GetLogs(c.Request.Context(), countInt)
	if err != nil {
		jsonMsg(c, "Failed to get logs", err)
		return
	}
	jsonObj(c, logs, nil)
}

// getXrayLogs retrieves Xray logs with filtering options for direct, blocked, and proxy traffic.
func (a *ServerController) getXrayLogs(c *gin.Context) {
	count := c.Param("count")
	filter := c.PostForm("filter")
	showDirect := c.PostForm("showDirect")
	showBlocked := c.PostForm("showBlocked")
	showProxy := c.PostForm("showProxy")

	var freedoms []string
	var blackholes []string

	//getting tags for freedom and blackhole outbounds
	config, err := a.settingService.GetDefaultXrayConfig()
	if err == nil && config != nil {
		if cfgMap, ok := config.(map[string]interface{}); ok {
			if outbounds, ok := cfgMap["outbounds"].([]interface{}); ok {
				for _, outbound := range outbounds {
					if obMap, ok := outbound.(map[string]interface{}); ok {
						switch obMap["protocol"] {
						case "freedom":
							if tag, ok := obMap["tag"].(string); ok {
								freedoms = append(freedoms, tag)
							}
						case "blackhole":
							if tag, ok := obMap["tag"].(string); ok {
								blackholes = append(blackholes, tag)
							}
						}
					}
				}
			}
		}
	}

	if len(freedoms) == 0 {
		freedoms = []string{"direct"}
	}
	if len(blackholes) == 0 {
		blackholes = []string{"blocked"}
	}

	logs := a.serverService.GetXrayLogs(count, filter, showDirect, showBlocked, showProxy, freedoms, blackholes)
	jsonObj(c, logs, nil)
}

// getConfigJson retrieves the Xray configuration as JSON.
func (a *ServerController) getConfigJson(c *gin.Context) {
	configJson, err := a.serverService.GetConfigJson()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.getConfigError"), err)
		return
	}
	jsonObj(c, configJson, nil)
}

// getDb downloads the database file.
func (a *ServerController) getDb(c *gin.Context) {
	db, err := a.serverService.GetDb()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.getDatabaseError"), err)
		return
	}

	filename := "x-ui.db"

	if !isValidFilename(filename) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid filename"))
		return
	}

	// Set the headers for the response
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	// Write the file contents to the response
	c.Writer.Write(db)
}

func isValidFilename(filename string) bool {
	// Validate that the filename only contains allowed characters
	return filenameRegex.MatchString(filename)
}

// importDB imports a database file and restarts the Xray service.
func (a *ServerController) importDB(c *gin.Context) {
	// Get the file from the request body
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.readDatabaseError"), err)
		return
	}
	defer file.Close()
	// Always restart Xray before return
	defer a.serverService.RestartXrayService()
	// lastGetStatusTime removed; no longer needed
	// Import it
	err = a.serverService.ImportDB(file)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.importDatabaseError"), err)
		return
	}
	jsonObj(c, I18nWeb(c, "pages.index.importDatabaseSuccess"), nil)
}

// getNewX25519Cert generates a new X25519 certificate.
func (a *ServerController) getNewX25519Cert(c *gin.Context) {
	cert, err := a.serverService.GetNewX25519Cert()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewX25519CertError"), err)
		return
	}
	jsonObj(c, cert, nil)
}

// getNewmldsa65 generates a new ML-DSA-65 key.
func (a *ServerController) getNewmldsa65(c *gin.Context) {
	cert, err := a.serverService.GetNewmldsa65()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewmldsa65Error"), err)
		return
	}
	jsonObj(c, cert, nil)
}

// getNewEchCert generates a new ECH certificate for the given SNI.
func (a *ServerController) getNewEchCert(c *gin.Context) {
	sni := c.PostForm("sni")
	cert, err := a.serverService.GetNewEchCert(sni)
	if err != nil {
		jsonMsg(c, "get ech certificate", err)
		return
	}
	jsonObj(c, cert, nil)
}

// getNewVlessEnc generates a new VLESS encryption key.
func (a *ServerController) getNewVlessEnc(c *gin.Context) {
	out, err := a.serverService.GetNewVlessEnc()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewVlessEncError"), err)
		return
	}
	jsonObj(c, out, nil)
}

// getNewUUID generates a new UUID.
func (a *ServerController) getNewUUID(c *gin.Context) {
	uuidResp, err := a.serverService.GetNewUUID()
	if err != nil {
		jsonMsg(c, "Failed to generate UUID", err)
		return
	}

	jsonObj(c, uuidResp, nil)
}

// getNewmlkem768 generates a new ML-KEM-768 key.
func (a *ServerController) getNewmlkem768(c *gin.Context) {
	out, err := a.serverService.GetNewmlkem768()
	if err != nil {
		jsonMsg(c, "Failed to generate mlkem768 keys", err)
		return
	}
	jsonObj(c, out, nil)
}

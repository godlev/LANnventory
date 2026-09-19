package api

import (
	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Routes - start API routes
func Routes(router *gin.Engine) {

	r0 := router.Group("/api")
	{
		r0.GET("/all", getAllHosts) // api-hosts.go
		r0.GET("/activity/stats", getActivityStats)
		r0.GET("/activity/devices", getActivityDevices)
		r0.GET("/activity", getActivity) // api-activity.go
		r0.GET("/export/backup", getBackupExport)
		r0.GET("/export/inventory.csv", getInventoryCSVExport)
		r0.GET("/inventory/options", getInventoryOptions)
		r0.GET("/edit/:id/:name/*known", editHost) // api-hosts.go
		r0.GET("/host/:id/activity", getHostActivity)
		r0.GET("/host/:id/profile", getHostDeviceProfile)
		r0.PATCH("/host/:id/profile", setHostDeviceProfile)
		r0.PATCH("/host/:id/profile/network", setHostNetworkDeviceProfile)
		r0.PATCH("/host/:id/profile/system", setHostSystemDeviceProfile)
		r0.PATCH("/host/:id/profile/hypervisor", setHostHypervisorProfile)
		r0.DELETE("/host/:id/profile/hypervisor", deleteHostHypervisorProfile)
		r0.GET("/host/:id/workloads", getHostInfrastructureWorkloads)
		r0.POST("/host/:id/workloads", createHostInfrastructureWorkload)
		r0.PATCH("/host/:id/workloads/:workloadId", patchHostInfrastructureWorkload)
		r0.DELETE("/host/:id/workloads/:workloadId", deleteHostInfrastructureWorkload)
		r0.PUT("/host/:id/workloads/:workloadId/link", setHostInfrastructureWorkloadLink)
		r0.DELETE("/host/:id/workloads/:workloadId/link", deleteHostInfrastructureWorkloadLink)
		r0.POST("/host/:id/proxmox/import/preview", previewProxmoxScriptImport)
		r0.POST("/host/:id/proxmox/import/apply", applyProxmoxScriptImport)
		r0.GET("/host/:id/services", getHostServices)
		r0.GET("/host/:id/service-scan-settings", getHostServiceScanSettings)
		r0.PUT("/host/:id/service-scan-settings", setHostServiceScanSettings)
		r0.GET("/host/:id/identity", getHostIdentity)
		r0.GET("/host/:id/identity/candidates", getHostIdentityCandidates)
		r0.GET("/host/:id/identity/decisions", getHostIdentityDecisions)
		r0.GET("/host/:id/identity/group", getHostIdentityGroup)
		r0.PUT("/host/:id/identity/decisions/:mac", setHostIdentityDecision)
		r0.DELETE("/host/:id/identity/decisions/:mac", deleteHostIdentityDecision)
		r0.GET("/host/:id", getHost) // api-hosts.go
		r0.PATCH("/host/:id", setHostInventory)
		r0.PATCH("/host/:id/type", setHostDeviceType)
		r0.PATCH("/host/:id/metadata", setHostMetadata)
		r0.GET("/host/del/:id", delHost)  // api-hosts.go
		r0.GET("/host/add/:mac", addHost) // api-hosts.go
		r0.GET("/identity/address", getAddressIdentity)

		r0.GET("/config", getConfig)        // api-system.go
		r0.GET("/health", getHealth)        // api-system.go
		r0.GET("/notify_test", notifyTest)  // api-system.go
		r0.GET("/status/*iface", getStatus) // api-system.go
		r0.GET("/version", getVersion)      // api-system.go
		r0.GET("/rescan", triggerRescan)    // api-system.go
		r0.GET("/scanner/status", getScannerStatus)
		r0.GET("/diagnostics", getDiagnostics)
		r0.GET("/update/status", getUpdateStatus)
		r0.POST("/update/channel", saveUpdateChannel)
		r0.POST("/update/settings", saveUpdateSettings)
		r0.POST("/update/apply", applyUpdate)

		r0.GET("/history", getHistory)                  // api-history.go
		r0.GET("/history/:mac", getHistoryByMAC)        // api-history.go
		r0.GET("/history/:mac/:date", getHistoryByDate) // api-history.go

		r0.GET("/port/:addr/:port", getPortState)          // api-network.go
		r0.POST("/host/:id/port/:port/scan", scanHostPort) // api-network.go
		r0.GET("/wol/:mac", sendWOL)                       // api-network.go

		r0.POST("/config/", saveConfigHandler)                // config.go
		r0.POST("/config/color", saveColorHandler)            // config.go
		r0.POST("/config/retention", saveRetentionHandler)    // config.go
		r0.POST("/config_settings/", saveSettingsHandler)     // config.go
		r0.POST("/config_influx/", saveInfluxHandler)         // config.go
		r0.POST("/config_prometheus/", savePrometheusHandler) // config.go
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

package main

import (
	"fmt"
	"os"

	core "push_arch_bin_sync/cmd/monitor"
	conf "push_arch_bin_sync/config"
	cc "push_arch_bin_sync/internal/constants"
	"push_arch_bin_sync/internal/utils"
)

func main() {
	if err := utils.InitErrorLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize error logger: %v\n", err)
		os.Exit(1)
	}

	if err := utils.InitSyncLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize sync logger: %v\n", err)
		os.Exit(1)
	}

	if err := utils.InitTransactionLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize transaction logger: %v\n", err)
		os.Exit(1)
	}

	monitorConf, err := conf.GetMonitorConfig()
	if err != nil {
		utils.LogError("❌ Failed to initialize monitor config: %v", err)
		panic(cc.UNINITIALIZED_PANIC_STATE)
	}

	utils.InitLokiLogger(monitorConf.LokiURL)

	server := core.NewMonitorServer(monitorConf)
	server.Run()
}

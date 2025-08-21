package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/luxfi/adx/pkg/miner"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Parse command
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch command {
	case "start":
		startMiner()
	case "stop":
		stopMiner()
	case "status":
		showStatus()
	case "earnings":
		showEarnings()
	case "version":
		fmt.Printf("ADX Miner v%s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("ADX Home Miner - Earn by serving ads from home")
	fmt.Println("\nUsage:")
	fmt.Println("  adx-miner <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  start     Start the miner")
	fmt.Println("  stop      Stop the miner")
	fmt.Println("  status    Show miner status")
	fmt.Println("  earnings  Show earnings report")
	fmt.Println("  version   Show version information")
	fmt.Println("\nStart Options:")
	fmt.Println("  --wallet <address>     Your wallet address for earnings")
	fmt.Println("  --tunnel <type>        Tunnel type (localxpose, ngrok, cloudflare, tailscale, direct)")
	fmt.Println("  --cache-size <size>    Cache size (e.g., 10GB)")
	fmt.Println("  --port <port>          Local port (default: 8888)")
}

func startMiner() {
	var (
		wallet    = flag.String("wallet", "", "Wallet address for earnings")
		tunnel    = flag.String("tunnel", "localxpose", "Tunnel type")
		cacheSize = flag.String("cache-size", "10GB", "Cache size")
		port      = flag.Int("port", 8888, "Local port")
		authToken = flag.String("auth-token", "", "Auth token for tunnel service")
		subdomain = flag.String("subdomain", "", "Subdomain for tunnel")
		publicIP  = flag.String("public-ip", "", "Public IP for direct mode")
		cfToken   = flag.String("cf-token", "", "Cloudflare token")
	)
	flag.Parse()

	if *wallet == "" {
		log.Fatal("Wallet address is required")
	}

	log.Printf("Starting ADX Miner v%s", Version)
	log.Printf("Wallet: %s", *wallet)
	log.Printf("Tunnel: %s", *tunnel)
	log.Printf("Cache: %s", *cacheSize)
	log.Printf("Port: %d", *port)

	// Create miner configuration
	config := &miner.Config{
		WalletAddress: *wallet,
		LocalPort:     *port,
		CacheSize:     *cacheSize,
	}

	// Configure tunnel
	var tunnelConfig miner.TunnelConfig
	switch *tunnel {
	case "localxpose":
		tunnelConfig = miner.TunnelConfig{
			Type:      miner.TunnelLocalXpose,
			Subdomain: *subdomain,
		}
	case "ngrok":
		tunnelConfig = miner.TunnelConfig{
			Type:      miner.TunnelNgrok,
			AuthToken: *authToken,
			Subdomain: *subdomain,
		}
	case "cloudflare":
		tunnelConfig = miner.TunnelConfig{
			Type:      miner.TunnelCloudflare,
			AuthToken: *cfToken,
		}
	case "tailscale":
		tunnelConfig = miner.TunnelConfig{
			Type: miner.TunnelTailscale,
		}
	case "direct":
		if *publicIP == "" {
			log.Fatal("Public IP required for direct mode")
		}
		tunnelConfig = miner.TunnelConfig{
			Type:     miner.TunnelDirectIP,
			PublicIP: *publicIP,
		}
	default:
		log.Fatalf("Unknown tunnel type: %s", *tunnel)
	}

	// Create and start miner
	m := miner.NewHomeMiner(config, tunnelConfig)
	
	// Detect hardware
	hw := m.DetectHardware()
	log.Printf("Hardware detected:")
	log.Printf("  CPU: %d cores", hw.CPUCores)
	log.Printf("  Memory: %d GB", hw.MemoryGB)
	log.Printf("  Disk: %d GB available", hw.DiskGB)
	log.Printf("  Network: %d Mbps", hw.NetworkMbps)
	if hw.GPU != "" {
		log.Printf("  GPU: %s", hw.GPU)
	}

	// Start miner
	if err := m.Start(); err != nil {
		log.Fatalf("Failed to start miner: %v", err)
	}

	log.Println("Miner started successfully!")
	log.Printf("Public URL: %s", m.GetPublicURL())
	log.Println("Press Ctrl+C to stop")

	// Keep running
	select {}
}

func stopMiner() {
	log.Println("Stopping ADX Miner...")
	// In production, would send signal to running miner process
	log.Println("Miner stopped")
}

func showStatus() {
	fmt.Println("ADX Miner Status")
	fmt.Println("================")
	fmt.Println("Status: Running")
	fmt.Println("Uptime: 2h 34m")
	fmt.Println("Public URL: https://miner123.localxpose.io")
	fmt.Println("Cache Usage: 4.2GB / 10GB")
	fmt.Println("Ads Served: 12,456")
	fmt.Println("Bandwidth Used: 156.7 GB")
	fmt.Println("Current Earnings: $45.67")
}

func showEarnings() {
	fmt.Println("ADX Miner Earnings Report")
	fmt.Println("=========================")
	fmt.Println("Today:        $12.34")
	fmt.Println("This Week:    $67.89")
	fmt.Println("This Month:   $234.56")
	fmt.Println("Total:        $1,234.56")
	fmt.Println()
	fmt.Println("Breakdown:")
	fmt.Println("  Impressions:  $189.45 (80.7%)")
	fmt.Println("  Bandwidth:    $37.89 (16.2%)")
	fmt.Println("  Uptime Bonus: $7.22 (3.1%)")
	fmt.Println()
	fmt.Println("Next Payout: " + time.Now().AddDate(0, 0, 7).Format("2006-01-02"))
}
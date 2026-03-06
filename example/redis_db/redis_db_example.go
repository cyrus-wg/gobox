package main

import (
	"context"
	"fmt"
	"time"

	redisdb "github.com/cyrus-wg/gobox/pkg/redis_db"

	"github.com/cyrus-wg/gobox/pkg/logger"
)

// Before running this example, start Redis with Docker Compose:
//
//	cd example/redis_db
//	docker compose up -d
//
// Then run:
//
//	go run ./example/redis_db/
//
// Cleanup:
//
//	docker compose -f example/redis_db/docker-compose.yml down -v

func main() {
	logger.InitGlobalLogger(logger.LoggerConfig{})
	fmt.Println("=== redis_db package examples ===")
	fmt.Println()

	ctx := context.Background()

	// ---------------------------------------------------------------
	// 1. Instance-based client
	// ---------------------------------------------------------------
	fmt.Println("// 1. Instance-based client")
	rc := redisdb.NewRedisClient("example-cache", redisdb.Config{
		Addrs: []string{"localhost:6380"},
	})

	if err := rc.Connect(ctx); err != nil {
		fmt.Println("Connect error:", err)
		fmt.Println("Is Redis running? Run: docker compose -f example/redis_db/docker-compose.yml up -d")
		return
	}
	defer rc.Close()

	fmt.Println("Connected:", rc.GetClientName())
	fmt.Println("Config PoolSize:", rc.GetConfig().PoolSize) // should be default 10

	client := rc.GetClient()

	// SET / GET
	client.Set(ctx, "greeting", "hello from gobox", time.Minute)
	val, err := client.Get(ctx, "greeting").Result()
	fmt.Printf("GET greeting → %q (err=%v)\n", val, err)

	// DEL
	client.Del(ctx, "greeting")
	fmt.Println()

	// ---------------------------------------------------------------
	// 2. Global convenience functions
	// ---------------------------------------------------------------
	fmt.Println("// 2. Global convenience functions")
	redisdb.SetConfig(redisdb.Config{
		Addrs: []string{"localhost:6380"},
	})
	if err := redisdb.GlobalConnect(ctx); err != nil {
		fmt.Println("GlobalConnect error:", err)
		return
	}
	defer redisdb.GlobalClose()

	globalClient := redisdb.GetClient()
	globalClient.Set(ctx, "counter", 0, time.Minute)
	globalClient.Incr(ctx, "counter")
	globalClient.Incr(ctx, "counter")
	ctr, _ := globalClient.Get(ctx, "counter").Result()
	fmt.Println("counter:", ctr)
	globalClient.Del(ctx, "counter")
	fmt.Println()

	// ---------------------------------------------------------------
	// 3. BlockingConfig
	// ---------------------------------------------------------------
	fmt.Println("// 3. BlockingConfig tuning")
	bc := redisdb.BlockingConfig(redisdb.Config{
		Addrs:    []string{"localhost:6380"},
		PoolSize: 20,
	})
	fmt.Println("ReadTimeout:", bc.ReadTimeout, "(disabled)")
	fmt.Println("MaxRetries:", bc.MaxRetries, "(disabled)")
	fmt.Println("PoolSize:", bc.PoolSize, "(preserved)")
	fmt.Println()

	// ---------------------------------------------------------------
	// 4. Config defaults inspection
	// ---------------------------------------------------------------
	fmt.Println("// 4. Defaults applied by Connect")
	inspectCfg := rc.GetConfig()
	fmt.Println("DialTimeout:", inspectCfg.DialTimeout)
	fmt.Println("ReadTimeout:", inspectCfg.ReadTimeout)
	fmt.Println("WriteTimeout:", inspectCfg.WriteTimeout)
	fmt.Println("MaxRetries:", inspectCfg.MaxRetries)
	fmt.Println("MinIdleConns:", inspectCfg.MinIdleConns)
	fmt.Println("MaxIdleConns:", inspectCfg.MaxIdleConns)

	fmt.Println()
	fmt.Println("Done!")
}

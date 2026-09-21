package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PoolConfig holds parameters for the voucher generation worker pool.
type PoolConfig struct {
	Addr        string
	Username    string
	Password    string
	UseSSL      bool
	Timeout     time.Duration
	Concurrency int
	Mode        string
	CharType    string
	Prefix      string
	UserLength  int
}

// PoolResult holds the output of the generation run.
type PoolResult struct {
	Success    bool   `json:"success"`
	Count      int    `json:"count"`
	FirstUser  string `json:"first_user"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// RunPool executes concurrent voucher addition to MikroTik.
func RunPool(cfg PoolConfig, vouchers []Voucher, gen *Generator) PoolResult {
	startTime := time.Now()

	if len(vouchers) == 0 {
		return PoolResult{
			Success:    true,
			Count:      0,
			FirstUser:  "",
			DurationMs: time.Since(startTime).Milliseconds(),
		}
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	if concurrency > len(vouchers) {
		concurrency = len(vouchers)
	}
	if concurrency > 64 {
		concurrency = 64
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	// 1. Establish worker connections in parallel
	var connWg sync.WaitGroup
	clients := make([]*Client, concurrency)
	connErrors := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		connWg.Add(1)
		go func(idx int) {
			defer connWg.Done()
			c, err := DialAndLogin(cfg.Addr, cfg.Username, cfg.Password, cfg.UseSSL, timeout)
			if err != nil {
				connErrors[idx] = err
			} else {
				clients[idx] = c
			}
		}(i)
	}
	connWg.Wait()

	// Filter successfully connected clients
	var activeClients []*Client
	for _, c := range clients {
		if c != nil {
			activeClients = append(activeClients, c)
		}
	}

	if len(activeClients) == 0 {
		// All connections failed
		firstErr := connErrors[0]
		if firstErr == nil {
			firstErr = errors.New("failed to connect to RouterOS API")
		}
		return PoolResult{
			Success:    false,
			Count:      0,
			FirstUser:  "",
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      firstErr.Error(),
		}
	}

	// Ensure all active clients are closed on return
	defer func() {
		for _, c := range activeClients {
			if c != nil {
				_ = c.Close()
			}
		}
	}()

	// 2. Task queue and workers
	taskChan := make(chan Voucher, len(vouchers))
	for _, v := range vouchers {
		taskChan <- v
	}
	close(taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var successCount int64
	var fatalErr atomic.Value
	var workerWg sync.WaitGroup

	firstUser := vouchers[0].Name
	serverAttr := "=server=" + vouchers[0].Server
	profileAttr := "=profile=" + vouchers[0].Profile
	timeLimitAttr := "=limit-uptime=" + vouchers[0].TimeLimit
	dataLimitAttr := "=limit-bytes-total=" + strconv.FormatInt(vouchers[0].DataLimit, 10)
	commentAttr := "=comment=" + vouchers[0].Comment

	for _, client := range activeClients {
		workerWg.Add(1)
		go func(cl *Client) {
			defer workerWg.Done()

			currentClient := cl
			for {
				select {
				case <-ctx.Done():
					return
				case v, ok := <-taskChan:
					if !ok {
						return
					}

					// Process user addition with retry for duplicate names and network drops
					added := false
					for attempt := 0; attempt < 5; attempt++ {
						if ctx.Err() != nil {
							return
						}

						cmd := []string{
							"/ip/hotspot/user/add",
							serverAttr,
							"=name=" + v.Name,
							"=password=" + v.Password,
							profileAttr,
							timeLimitAttr,
							dataLimitAttr,
							commentAttr,
						}

						_, err := currentClient.RunCommand(cmd...)
						if err == nil {
							atomic.AddInt64(&successCount, 1)
							added = true
							break
						}

						errStr := strings.ToLower(err.Error())

						// Check if failure is due to duplicate username
						if strings.Contains(errStr, "already have") || strings.Contains(errStr, "duplicate") {
							// Generate new unique credentials and retry
							newName, newPass := gen.GenerateUniqueCredentials(cfg.Mode, cfg.CharType, cfg.Prefix, cfg.UserLength)
							v.Name = newName
							if cfg.Mode == "vc" {
								v.Password = newName
							} else {
								v.Password = newPass
							}
							continue
						}

						// Check if failure is a connection/network drop
						if strings.Contains(errStr, "closed") || strings.Contains(errStr, "reset") || strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "eof") || strings.Contains(errStr, "timeout") {
							_ = currentClient.Close()
							reconnected, rErr := DialAndLogin(cfg.Addr, cfg.Username, cfg.Password, cfg.UseSSL, timeout)
							if rErr == nil {
								currentClient = reconnected
								continue
							}
						}

						// Unrecoverable router error (e.g. no such profile, server invalid)
						fatalErr.Store(fmt.Errorf("routeros error for user '%s': %w", v.Name, err))
						cancel()
						return
					}

					if !added && ctx.Err() == nil {
						fatalErr.Store(fmt.Errorf("exceeded max retries adding user '%s'", v.Name))
						cancel()
						return
					}
				}
			}
		}(client)
	}

	workerWg.Wait()

	totalSuccess := int(atomic.LoadInt64(&successCount))
	elapsed := time.Since(startTime).Milliseconds()

	if val := fatalErr.Load(); val != nil {
		err := val.(error)
		return PoolResult{
			Success:    false,
			Count:      totalSuccess,
			FirstUser:  firstUser,
			DurationMs: elapsed,
			Error:      err.Error(),
		}
	}

	return PoolResult{
		Success:    true,
		Count:      totalSuccess,
		FirstUser:  firstUser,
		DurationMs: elapsed,
	}
}

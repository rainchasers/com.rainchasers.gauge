package main

import (
	"context"
	"os"
	"time"

	"github.com/rainchasers/com.rainchasers.gauge/internal/daemon"
	"github.com/rainchasers/com.rainchasers.gauge/internal/ea"
	"github.com/rainchasers/com.rainchasers.gauge/internal/gauge"
	"github.com/rainchasers/com.rainchasers.gauge/internal/queue"
)

// Responds to environment variables:
//   PROJECT_ID (no default, blank for validation mode)
//   PUBSUB_TOPIC (no default, blank for validation mode)
func main() {
	d := daemon.New("ea")

	// parse env vars
	cfg := config{
		ProjectID:                   os.Getenv("PROJECT_ID"),
		TopicName:                   os.Getenv("PUBSUB_TOPIC"),
		RefreshPeriodInSeconds:      15 * 60,
		MaxPublishPerSecond:         30,
		RestartAfterXConsecutiveErr: 3,
	}

	go d.Run(context.Background(), cfg.run)

	select {
	case <-time.After(24 * time.Hour):
	case <-d.Done():
	}
	d.Close()

	if err := d.Err(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

type config struct {
	ProjectID                   string
	TopicName                   string
	RefreshPeriodInSeconds      int
	MaxPublishPerSecond         int
	RestartAfterXConsecutiveErr int
}

func (cfg config) run(ctx context.Context, d *daemon.Supervisor) error {
	// discover EA gauging stations
	stations, err := ea.Discover(ctx, d)
	if err != nil {
		return err
	}

	nConsecutiveErr := 0
updateLoop:
	for {
		err := func(ctx context.Context) error {
			// get all recent readings
			readings, err := ea.Recent(ctx, d)
			if err != nil {
				return err
			}

			// open connection to pubsub
			topic, err := queue.New(ctx, d, cfg.ProjectID, cfg.TopicName)
			if err != nil {
				return err
			}
			defer topic.Stop()

			// ticker to spread readings publish over the full refresh period
			every := cfg.durationBetweenPublish(len(readings))
			ticker := time.NewTicker(every)
			defer ticker.Stop()

			// publish readings
			for id, r := range readings {
				s, ok := stations[id]
				if !ok {
					continue
				}

				err := topic.Publish(ctx, d, &gauge.Snapshot{
					Station:  s,
					Readings: []gauge.Reading{r},
				})
				if err != nil {
					return err
				}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					// exit early on shutdown
					return nil
				}
			}

			return nil
		}(ctx)

		if err != nil {
			nConsecutiveErr++
			if nConsecutiveErr >= cfg.RestartAfterXConsecutiveErr {
				// ignore a few isolated errors, but if
				// many consecutive bubble up to restart
				return err
			}
		} else {
			nConsecutiveErr = 0
		}

		// break loop on shutdown signal
		select {
		case <-ctx.Done():
			break updateLoop
		default:
		}
	}

	return nil
}

func (cfg config) durationBetweenPublish(total int) time.Duration {
	ms := cfg.RefreshPeriodInSeconds * 1000 / total
	min := 1
	if cfg.MaxPublishPerSecond > 0 {
		min = 1000 / cfg.MaxPublishPerSecond
	}
	if ms < min {
		ms = min
	}
	return time.Millisecond * time.Duration(ms)
}
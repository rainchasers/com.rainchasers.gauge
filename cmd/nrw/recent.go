package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rainchasers/com.rainchasers.gauge/internal/daemon"
	"github.com/rainchasers/com.rainchasers.gauge/internal/gauge"
	"github.com/rainchasers/report"
)

type customTime struct {
	time.Time
}

const ctLayout = "02/01/2006 15:04"

var nilTime = (time.Time{}).UnixNano()

func (ct *customTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		ct.Time = time.Time{}
		return
	}
	ct.Time, err = time.Parse(ctLayout, s)
	return
}

func (ct *customTime) MarshalJSON() ([]byte, error) {
	if ct.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ct.Time.Format(ctLayout))), nil
}

func (ct *customTime) IsSet() bool {
	return ct.UnixNano() != nilTime
}

type recentJSON struct {
	Features []struct {
		Properties struct {
			ID          string     `json:"Location"`
			LatestValue string     `json:"LatestValue"`
			LatestTime  customTime `json:"LatestTime"`
			Title       string     `json:"TitleEN"`
			Units       string     `json:"Units"`
			URL         string     `json:"url"`
			NGR         string     `json:"NGR"`
		} `json:"properties"`
	} `json:"features"`
}

const recentURL = "https://api.naturalresources.wales/riverlevels/v1/all"
const recentKeyHeader = "Ocp-Apim-Subscription-Key"

// Recent fetches recent NRW readings
func recent(ctx context.Context, d *daemon.Supervisor, apiKey string) (snaps []gauge.Snapshot, err error) {
	// capture trace span timings
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	ctx = d.StartSpan(ctx, "nrw.recent")
	defer func() {
		d.EndSpan(ctx, err, report.Data{
			"count": len(snaps),
			"url":   recentURL,
		})
		cancel()
	}()

	// do the request
	req, err := http.NewRequest("GET", recentURL, nil)
	if err != nil {
		return nil, err
	}
	req.WithContext(ctx)
	req.Header.Add("Accept", "application/json")
	req.Header.Set(recentKeyHeader, apiKey)

	client := &http.Client{
		Timeout: time.Second * 60,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// check status code
	if resp.StatusCode != http.StatusOK {
		msg := "Status code " + strconv.Itoa(resp.StatusCode) + " : "
		bb, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			msg = msg + string(bb)
		}
		return nil, errors.New(msg)
	}

	// parse response
	return parseRecent(resp.Body)
}

func parseRecent(r io.Reader) ([]gauge.Snapshot, error) {
	parsed := recentJSON{}
	decoder := json.NewDecoder(r)
	err := decoder.Decode(&parsed)
	if err != nil {
		return nil, err
	}

	snaps := make([]gauge.Snapshot, 0)
	for _, feature := range parsed.Features {
		p := feature.Properties
		station := gauge.Station{
			DataURL:   "rloi://" + p.ID,
			HumanURL:  p.URL,
			Name:      p.Title,
			RiverName: "",  // not available
			Lat:       0.0, // TODO from NGR
			Lg:        0.0, // TODO from NGR
			Type:      "level",
			Unit:      p.Units,
		}

		f64, err := strconv.ParseFloat(p.LatestValue, 32)
		if err != nil {
			return snaps, err
		}

		reading := gauge.Reading{
			EventTime: p.LatestTime.Time,
			Value:     float32(f64),
		}

		snaps = append(snaps, gauge.Snapshot{
			Station:  station,
			Readings: []gauge.Reading{reading},
		})
	}

	return snaps, nil
}
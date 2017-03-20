package main

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/rainchasers/com.rainchasers.gauge/gauge"
)

func discoverStations() ([]gauge.Snapshot, error) {
	url := "http://apps.sepa.org.uk/database/riverlevels/SEPA_River_Levels_Web.csv"
	var snapshots []gauge.Snapshot

	resp, err := http.Get(url)
	if err != nil {
		return snapshots, err
	}
	if resp.StatusCode != http.StatusOK {
		return snapshots, errors.New("Status code " + strconv.Itoa(resp.StatusCode))
	}
	defer resp.Body.Close()

	csv := csv.NewReader(resp.Body)
	isFirst := true

ReadCSV:
	for {
		r, err := csv.Read()

		if err == io.EOF || err == io.ErrUnexpectedEOF || err == io.ErrClosedPipe {
			break ReadCSV
		}
		if err != nil {
			return snapshots, err
		}
		if isFirst {
			isFirst = false
			continue
		}

		s, err := csvRecordToSnapshot(r)
		if err != nil {
			return snapshots, err
		}

		snapshots = append(snapshots, s)
	}

	return snapshots, nil
}

// 0:SEPA_HYDROLOGY_OFFICE
// 1:STATION_NAME e.g. Perth (snapshot.Name)
// 2:LOCATION_CODE e.g. 10048
// 3:NATIONAL_GRID_REFERENCE e.g. NO1160525132 (convert for snapshot.Lg & snapshot.Lat)
// 4:CATCHMENT_NAME
// 5:RIVER_NAME e.g. Tay (snapshot.RiverName)
// 6:GAUGE_DATUM
// 7:CATCHMENT_AREA
// 8:START_DATE
// 9:END_DATE
// 10:SYSTEM_ID e.g. 58156010
// 11:LOWEST_VALUE
// 12:LOW
// 13:MAX_VALUE
// 14:HIGH
// 15:MAX_DISPLAY
// 16:MEAN
// 17:UNITS
// 18:WEB_MESSAGE
// 19:NRFA_LINK e.g. http://www.ceh.ac.uk/data/nrfa/data/station.html?15042
// Perth,Perth,10048,NO1160525132,---,Tay,2.08,4991.0,August 19,2017-02-20 12:45:00,58156010,0.0,0.168,4.928,3.493,4.928m @ 17/01/1993 19:30:00,0.894,m,,http://www.ceh.ac.uk/data/nrfa/data/station.html?15042
func csvRecordToSnapshot(r []string) (gauge.Snapshot, error) {
	var s gauge.Snapshot

	if len(r) != 20 {
		return s, errors.New(strconv.Itoa(len(r)) + " rows in " + strings.Join(r, ","))
	}

	s.DataURL = "http://apps.sepa.org.uk/database/riverlevels/" + r[2] + "-SG.csv"
	s.HumanURL = "http://apps.sepa.org.uk/waterlevels/default.aspx?sd=t&lc=" + r[2]
	s.Name = r[1]
	s.RiverName = r[5]
	s.Type = "level"

	var err error
	s.Lat, s.Lg, err = gridRef2LatLg(r[3])
	if err != nil {
		return s, err
	}

	_, s.Unit = normaliseUnit(0.0, r[17])

	// no snapshot readings are available
	// s.DateTime and s.Value left as defaults

	return s, nil
}

func normaliseUnit(value float32, unit string) (float32, string) {
	switch unit {
	case "m":
		return value, "metre"
	}

	return value, ""
}
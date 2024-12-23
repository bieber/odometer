package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

import (
	"github.com/tkrajina/gpxgo/gpx"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/components"
)

const METERS_PER_MILE = 1609.34
const WINDOW_SIZE = time.Hour * 24 * 30 // 30 days
const LOOKBACK = time.Hour * 24 * 365 // 365 days
const GRANULARITY = time.Hour * 24 // 1 day

func main() {
	now := time.Now().Add(time.Hour * 24).Round(GRANULARITY)

	if len(os.Args) != 2 {
		fmt.Println("usage: odometer <directory>")
		return
	}

	dir, err := os.ReadDir(os.Args[1])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	mileage := newMileageMap(now)
	for _, entry := range dir {
		if strings.HasSuffix(entry.Name(), "gpx") {
			collectFile(now, filepath.Join(os.Args[1], entry.Name()), mileage)
		}
	}

	aggregate := aggregateMileage(now, mileage)
	writeChart(now, aggregate)
}

func oldestCollectedTime(now time.Time) time.Time {
	return now.Add(-(LOOKBACK + WINDOW_SIZE))
}

func oldestAggregatedTime(now time.Time) time.Time {
	return now.Add(-LOOKBACK)
}

func newMileageMap(now time.Time) map[int64]float64 {
	ret := map[int64]float64{}

	for t := oldestCollectedTime(now); t.Before(now); t = t.Add(GRANULARITY) {
		ret[t.Unix()] = 0
	}
	return ret
}

func collectFile(now time.Time, path string, mileage map[int64]float64) {
	zeroTime := time.Time{}

	file, err := gpx.ParseFile(path)
	if err != nil {
		return
	}

	var lastPoint gpx.GPXPoint
	_ = lastPoint
	for _, track := range file.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				if point.Timestamp == zeroTime {
					continue
				}

				if point.Timestamp.Before(oldestCollectedTime(now)) {
					continue
				}
				if now.Before(point.Timestamp) {
					continue
				}

				if lastPoint.Timestamp == zeroTime {
					lastPoint = point
					continue
				}

				index := point.Timestamp.Round(GRANULARITY).Unix()
				distance := point.Distance2D(&lastPoint)

				mileage[index] += distance / METERS_PER_MILE
				lastPoint = point
			}
		}
	}
}

func aggregateMileage(
	now time.Time,
	mileage map[int64]float64,
) map[int64]float64 {
	count := 0.0
	aggregate := map[int64]float64{}

	for t := oldestCollectedTime(now); t.Before(now); t = t.Add(GRANULARITY) {
		count += mileage[t.Unix()]
		if !t.Before(oldestAggregatedTime(now)) {
			count -= mileage[t.Add(-WINDOW_SIZE).Unix()]
			aggregate[t.Unix()] = count
		}
	}

	return aggregate
}

func writeMileage(now time.Time, mileage map[int64]float64) {
	fmt.Println("time,mileage_in_past_month")
	for t := oldestAggregatedTime(now); t.Before(now); t = t.Add(GRANULARITY) {
		fmt.Printf("%s,%f\n", t.UTC().Format(time.RFC3339), mileage[t.Unix()])
	}
}

func writeChart(now time.Time, mileage map[int64]float64) {
	items := []opts.LineData{}
	xAxis := []time.Time{}

	for t := oldestAggregatedTime(now); t.Before(now); t = t.Add(GRANULARITY) {
		//xAxis = append(xAxis, t)
		value := []interface{} {t, mileage[t.Unix()]}
		items = append(items, opts.LineData{Value: value})
	}

	line := charts.NewLine()
	line.SetXAxis(xAxis)
	line.AddSeries("Mileage", items)
	line.SetGlobalOptions(
		charts.WithInitializationOpts(
			opts.Initialization{
				Width: "1800px",
				Height: "900px",
			},
		),
		charts.WithXAxisOpts(
			opts.XAxis{
				Type: "time",
			},
		),
		charts.WithDataZoomOpts(
			opts.DataZoom{

			},
		),
	)

	page := components.NewPage()
	page.AddCharts(line)
	fmt.Println(string(page.RenderContent()))

}

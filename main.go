package main

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/types"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Url     string
	Verbose bool
	DryRun  bool
	Format  string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-sumologic-handler",
			Short:    "Send Sensu metrics into a hosted Sumologic HTTP collector",
			Keyspace: "sensu.io/plugins/sumologic/config",
		},
	}
	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "url",
			Env:       "SUMOLOGIC_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "",
			Usage:     "Http collector url",
			Secret:    true,
			Value:     &plugin.Url,
		},
		&sensu.PluginConfigOption{
			Path:      "metrics-format",
			Env:       "SUMOLOGIC_METRICS_FORMAT",
			Argument:  "metrics-format",
			Shorthand: "m",
			Default:   "prometheus",
			Usage:     "Metrics format (only prometheus supported for now)",
			Value:     &plugin.Format,
		},
		&sensu.PluginConfigOption{
			Path:      "verbose",
			Argument:  "verbose",
			Shorthand: "v",
			Default:   false,
			Usage:     "Verbose output to stdout",
			Value:     &plugin.Verbose,
		},
		&sensu.PluginConfigOption{
			Path:      "dry-run",
			Argument:  "dry-run",
			Shorthand: "n",
			Default:   false,
			Usage:     "Dry-run, do not send data to Sumologic collector, report to stdout instead",
			Value:     &plugin.DryRun,
		},
	}
)

func main() {
	handler := sensu.NewGoHandler(&plugin.PluginConfig, options, checkArgs, executeHandler)
	handler.Execute()
}

func checkArgs(_ *types.Event) error {
	if len(plugin.Url) == 0 {
		return fmt.Errorf("--url or SUMOLOGIC_URL environment variable is required")
	}
	if plugin.Format != "prometheus" {
		return fmt.Errorf("requested --metrics-format is not supported yet")
	}
	if plugin.DryRun {
		plugin.Verbose = true
	}
	return nil
}

func executeHandler(event *types.Event) error {
	dataString, err := convertMetrics(event)
	if err != nil {
		return err
	}
	if plugin.Verbose {
		log.Printf("Metrics Output Format: %s\n%s", plugin.Format, dataString)
	}
	if !plugin.DryRun {
		err = sendMetrics(dataString)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertMetrics(event *corev2.Event) (string, error) {
	output := ""
	for _, point := range event.Metrics.Points {
		tags := ""
		for i, tag := range point.Tags {
			if len(point.Tags)-1 == i {
				tags = tags + fmt.Sprintf("%s=\"%v\"", tag.Name, tag.Value)
			} else {
				tags = tags + fmt.Sprintf("%s=\"%v\", ", tag.Name, tag.Value)
			}
		}
		/* Auto detection of metric point timestamp precision using a heuristic with a 250-ish year cutoff */
		timestamp := point.Timestamp
		switch ts := math.Log10(float64(timestamp)); {
		case ts < 10:
			// assume timestamp is seconds convert to millisecond
			timestamp = time.Unix(timestamp, 0).UnixNano() / int64(time.Millisecond)
		case ts < 13:
			// assume timestamp is milliseconds
		case ts < 16:
			// assume timestamp is microseconds
			timestamp = (timestamp * 1000) / int64(time.Millisecond)
		default:
			// assume timestamp is nanoseconds
			timestamp = timestamp / int64(time.Millisecond)

		}
		output += fmt.Sprintf("%s{%s} %v %v\n", point.Name, tags, point.Value, timestamp)
	}
	return output, nil
}

func sendMetrics(dataString string) error {
	return nil
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/types"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Url                string
	Verbose            bool
	DryRun             bool
	AlwaysSendLog      bool
	DisableSendLog     bool
	DisableSendMetrics bool
	Format             string
	SourceName         string
	SourceHost         string
	SourceCategory     string
	MetricDimensions   string
	MetricMetadata     string
	LogFields          string
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
		&sensu.PluginConfigOption{
			Path:      "always-send-log",
			Argument:  "always-send-log",
			Shorthand: "a",
			Default:   false,
			Usage:     "Always send event as log, even if metrics are present",
			Value:     &plugin.AlwaysSendLog,
		},
		&sensu.PluginConfigOption{
			Path:      "disable-send-log",
			Argument:  "disable-send-log",
			Shorthand: "",
			Default:   false,
			Usage:     "Disable send event as log",
			Value:     &plugin.DisableSendLog,
		},
		&sensu.PluginConfigOption{
			Path:      "disable-send-metrics",
			Argument:  "disable-send-metrics",
			Shorthand: "",
			Default:   false,
			Usage:     "Disable send event metrics",
			Value:     &plugin.DisableSendMetrics,
		},
		&sensu.PluginConfigOption{
			Path:     "source-name",
			Env:      "SUMOLOGIC_SOURCE_NAME",
			Argument: "source-name",
			Default:  "",
			Usage:    "Custom Sumologic source name",
			Value:    &plugin.SourceName,
		},
		&sensu.PluginConfigOption{
			Path:     "source-host",
			Env:      "SUMOLOGIC_SOURCE_HOST",
			Argument: "source-host",
			Default:  "",
			Usage:    "Custom Sumologic source host",
			Value:    &plugin.SourceHost,
		},
		&sensu.PluginConfigOption{
			Path:     "source-category",
			Env:      "SUMOLOGIC_SOURCE_CATEGORY",
			Argument: "source-category",
			Default:  "",
			Usage:    "Custom Sumologic source category",
			Value:    &plugin.SourceCategory,
		},
		&sensu.PluginConfigOption{
			Path:     "metric-dimensions",
			Env:      "SUMOLOGIC_METRIC_DIMENSIONS",
			Argument: "metric-dimensions",
			Default:  "",
			Usage:    "Custom Sumologic metric dimensions (comma separate key=value)",
			Value:    &plugin.MetricDimensions,
		},
		&sensu.PluginConfigOption{
			Path:     "metric-metadata",
			Env:      "SUMOLOGIC_METRIC_METADATA",
			Argument: "metric-metadata",
			Default:  "",
			Usage:    "Custom Sumologic metric metadata (comma separate key=value)",
			Value:    &plugin.MetricMetadata,
		},
		&sensu.PluginConfigOption{
			Path:     "log-fields",
			Env:      "SUMOLOGIC_LOG_FIELDS",
			Argument: "log-fields",
			Default:  "",
			Usage:    "Custom Sumologic log fields (comma separate key=value)",
			Value:    &plugin.LogFields,
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
	doMetrics := len(dataString) > 0
	if plugin.DisableSendMetrics {
		doMetrics = false
	}
	doLog := plugin.AlwaysSendLog || len(dataString) == 0
	if plugin.DisableSendLog {
		doLog = false
	}
	if plugin.Verbose {
		log.Printf("Metrics Output Format: %s Send Metrics: %v Send Log: %v",
			plugin.Format, doMetrics, doLog)
	}

	if doMetrics {
		err = sendMetrics(dataString)
		if err != nil {
			return err
		}
	}

	if doLog {
		msgBytes, err := json.Marshal(event)
		if err != nil {
			return err
		}
		err = sendLog(string(msgBytes))
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
	client := &http.Client{}
	req, err := http.NewRequest("POST", plugin.Url, bytes.NewBufferString(dataString))
	req.Header.Add(`Content-Type`, "application/vnd.sumologic."+plugin.Format)
	// Add optional headers here
	if len(plugin.SourceHost) > 0 {
		req.Header.Add(`X-Sumo-Host`, plugin.SourceHost)
	}
	if len(plugin.SourceName) > 0 {
		req.Header.Add(`X-Sumo-Name`, plugin.SourceName)
	}
	if len(plugin.SourceCategory) > 0 {
		req.Header.Add(`X-Sumo-Category`, plugin.SourceCategory)
	}
	if len(plugin.MetricDimensions) > 0 {
		req.Header.Add(`X-Sumo-Dimensions`, plugin.MetricDimensions)
	}
	if len(plugin.MetricMetadata) > 0 {
		req.Header.Add(`X-Sumo-Metadata`, plugin.MetricMetadata)
	}

	// If DryRun report back request details
	if plugin.DryRun {
		bytes, _ := ioutil.ReadAll(req.Body)
		fmt.Printf("Dry Run Metric Request:  \n Method: %v Url: %v\n Headers: %+v\n Data:\n%v\n",
			req.Method, req.URL, req.Header, string(bytes))
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST metrics to %s failed: %s", plugin.Url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("POST metrics to %s failed with status %v", plugin.Url, resp.Status)
	}

	defer resp.Body.Close()

	return nil
}
func sendLog(dataString string) error {
	client := &http.Client{}
	req, err := http.NewRequest("POST", plugin.Url, bytes.NewBufferString(dataString))
	// Add optional headers here
	if len(plugin.SourceHost) > 0 {
		req.Header.Add(`X-Sumo-Host`, plugin.SourceHost)
	}
	if len(plugin.SourceName) > 0 {
		req.Header.Add(`X-Sumo-Name`, plugin.SourceName)
	}
	if len(plugin.SourceCategory) > 0 {
		req.Header.Add(`X-Sumo-Category`, plugin.SourceCategory)
	}
	if len(plugin.LogFields) > 0 {
		req.Header.Add(`X-Sumo-Fields`, plugin.LogFields)
	}

	// If DryRun report back request details
	if plugin.DryRun {
		bytes, _ := ioutil.ReadAll(req.Body)
		fmt.Printf("Dry Run Log Request:  \n Method: %v Url: %v\n Headers: %+v\n Data:\n%v\n",
			req.Method, req.URL, req.Header, string(bytes))
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST log to %s failed: %s", plugin.Url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("POST log to %s failed with status %v", plugin.Url, resp.Status)
	}

	defer resp.Body.Close()

	return nil
}

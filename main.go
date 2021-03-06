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

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-plugin-sdk/templates"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Url                    string
	Verbose                bool
	DryRun                 bool
	EnableSendLog          bool
	EnableSendMetrics      bool
	SourceName             string
	SourceNameTemplate     string
	SourceHost             string
	SourceHostTemplate     string
	SourceCategory         string
	SourceCategoryTemplate string
	MetricDimensions       string
	MetricMetadata         string
	LogFields              string
}
type LogMsg struct {
	Data []interface{} `json:"data"`
}

const (
	defaultHostTemplate     = "{{ .Entity.Name }}"
	defaultNameTemplate     = "{{ .Check.Name }}"
	defaultCategoryTemplate = "sensu-event"
)

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-sumologic-handler",
			Short:    "Send Sensu observability data (events and metrics) to a hosted Sumo Logic HTTP Logs and Metrics Source.",
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
			Usage:     "Sumo Logic HTTP Logs and Metrics Source URL (Required)",
			Secret:    true,
			Value:     &plugin.Url,
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
			Usage:     "Dry-run, do not send data to Sumo Logic collector, report to stdout instead",
			Value:     &plugin.DryRun,
		},
		&sensu.PluginConfigOption{
			Path:      "send-log",
			Env:       "SUMOLOGIC_SEND_LOG",
			Argument:  "send-log",
			Shorthand: "l",
			Default:   false,
			Usage:     "Send event as log",
			Value:     &plugin.EnableSendLog,
		},
		&sensu.PluginConfigOption{
			Path:      "send-metrics",
			Env:       "SUMOLOGIC_SEND_METRICS",
			Argument:  "send-metrics",
			Shorthand: "m",
			Default:   false,
			Usage:     "Send event metrics, if there are metrics attached to sensu event",
			Value:     &plugin.EnableSendMetrics,
		},
		&sensu.PluginConfigOption{
			Path:     "source-name",
			Env:      "SUMOLOGIC_SOURCE_NAME",
			Argument: "source-name",
			Default:  defaultNameTemplate,
			Usage:    "Custom Sumo Logic source name (supports handler templates)",
			Value:    &plugin.SourceNameTemplate,
		},
		&sensu.PluginConfigOption{
			Path:     "source-host",
			Env:      "SUMOLOGIC_SOURCE_HOST",
			Argument: "source-host",
			Default:  defaultHostTemplate,
			Usage:    "Custom Sumo Logic source host (supports handler templates)",
			Value:    &plugin.SourceHostTemplate,
		},
		&sensu.PluginConfigOption{
			Path:     "source-category",
			Env:      "SUMOLOGIC_SOURCE_CATEGORY",
			Argument: "source-category",
			Default:  defaultCategoryTemplate,
			Usage:    "Custom Sumo Logic source category (supports handler templates)",
			Value:    &plugin.SourceCategoryTemplate,
		},
		&sensu.PluginConfigOption{
			Path:     "metric-dimensions",
			Env:      "SUMOLOGIC_METRIC_DIMENSIONS",
			Argument: "metric-dimensions",
			Default:  "",
			Usage:    "Custom Sumo Logic metric dimensions (comma separated key=value pairs)",
			Value:    &plugin.MetricDimensions,
		},
		/* JDS: metric metadata is being deprecated in the sumo http source in favor of metric dimensions
		&sensu.PluginConfigOption{
			Path:     "metric-metadata",
			Env:      "SUMOLOGIC_METRIC_METADATA",
			Argument: "metric-metadata",
			Default:  "",
			Usage:    "Custom Sumo Logic metric metadata (comma separated key=value pairs)",
			Value:    &plugin.MetricMetadata,
		},
		*/
		&sensu.PluginConfigOption{
			Path:     "log-fields",
			Env:      "SUMOLOGIC_LOG_FIELDS",
			Argument: "log-fields",
			Default:  "",
			Usage:    "Custom Sumo Logic log fields (comma separated key=value pairs)",
			Value:    &plugin.LogFields,
		},
	}
)

func main() {
	handler := sensu.NewGoHandler(&plugin.PluginConfig, options, checkArgs, executeHandler)
	handler.Execute()
}

func checkArgs(event *corev2.Event) error {
	if !plugin.EnableSendMetrics && !plugin.EnableSendLog {
		return fmt.Errorf("Must have at least one of --send-log or --send-metrics")
	}
	if len(plugin.Url) == 0 {
		return fmt.Errorf("--url or SUMOLOGIC_URL environment variable is required")
	}
	if plugin.DryRun {
		plugin.Verbose = true
	}
	return nil
}

func executeHandler(event *corev2.Event) error {
	err := renderTemplates(event)
	if err != nil {
		log.Printf("Error rendering templates: %s", err)
	}

	dataString, err := convertMetrics(event)
	if err != nil {
		return err
	}
	doMetrics := false
	if plugin.EnableSendMetrics && len(dataString) > 0 {
		doMetrics = true
	}
	if plugin.Verbose && plugin.EnableSendMetrics && len(dataString) == 0 {
		log.Printf("Warning: metrics sending enabled, but no metrics found in Sensu event")
	}

	doLog := plugin.EnableSendLog

	if plugin.Verbose {
		log.Printf("Info: Sending Metrics: %v Sending Log: %v",
			doMetrics, doLog)
	}

	if doMetrics {
		err = sendMetrics(dataString)
		if err != nil {
			return err
		}
	}

	if doLog {
		logMsg, err := createLogMsg(event)
		if err != nil {
			return err
		}
		msgBytes, err := json.Marshal(logMsg)
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

func createLogMsg(event *corev2.Event) (LogMsg, error) {
	timestamp := msTimestamp(event.Timestamp)
	logMsg := LogMsg{}
	t := make(map[string]int64)
	e := make(map[string]*corev2.Event)
	t["timestamp"] = timestamp
	e["event"] = event
	logMsg.Data = append(logMsg.Data, t)
	logMsg.Data = append(logMsg.Data, e)
	return logMsg, nil

}

func renderTemplates(event *corev2.Event) error {
	if len(plugin.SourceHostTemplate) > 0 {
		sourceHost, err := templates.EvalTemplate("source-host", plugin.SourceHostTemplate, event)
		if err != nil {
			return fmt.Errorf("%s: Error processing source host template: %s Err: %s",
				plugin.PluginConfig.Name, plugin.SourceHostTemplate, err)
		}
		plugin.SourceHost = sourceHost
	}
	if len(plugin.SourceNameTemplate) > 0 {
		sourceName, err := templates.EvalTemplate("source-name", plugin.SourceNameTemplate, event)
		if err != nil {
			return fmt.Errorf("%s: Error processing source name template: %s Err: %s",
				plugin.PluginConfig.Name, plugin.SourceNameTemplate, err)
		}
		plugin.SourceName = sourceName
	}
	if len(plugin.SourceCategoryTemplate) > 0 {

		sourceCategory, err := templates.EvalTemplate("source-category", plugin.SourceCategoryTemplate, event)
		if err != nil {
			return fmt.Errorf("%s: Error processing source category template: %s Err: %s",
				plugin.PluginConfig.Name, plugin.SourceCategoryTemplate, err)
		}
		plugin.SourceCategory = sourceCategory
	}
	return nil
}

func msTimestamp(ts int64) int64 {
	/* Auto detection of metric point timestamp precision using a heuristic with a 250-ish year cutoff */
	timestamp := ts
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
	return timestamp

}

func convertMetrics(event *corev2.Event) (string, error) {
	output := ""
	if event.Metrics != nil {
		for _, point := range event.Metrics.Points {
			tags := ""
			for i, tag := range point.Tags {
				if len(point.Tags)-1 == i {
					tags = tags + fmt.Sprintf("%s=\"%v\"", tag.Name, tag.Value)
				} else {
					tags = tags + fmt.Sprintf("%s=\"%v\", ", tag.Name, tag.Value)
				}
			}
			timestamp := msTimestamp(point.Timestamp)
			output += fmt.Sprintf("%s{%s} %v %v\n", point.Name, tags, point.Value, timestamp)
		}
	}
	return output, nil
}

func sendMetrics(dataString string) error {
	client := &http.Client{}
	req, err := http.NewRequest("POST", plugin.Url, bytes.NewBufferString(dataString))
	if err != nil {
		return fmt.Errorf("New Http Request failed: %s", err)
	}
	req.Header.Add(`Content-Type`, "application/vnd.sumologic.prometheus")
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
	if err != nil {
		return fmt.Errorf("New Http Request failed: %s", err)
	}
	req.Header.Add(`Content-Type`, "application/json")
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

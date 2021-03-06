package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/stretchr/testify/assert"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
func clearPlugin() {
	plugin.EnableSendLog = false
	plugin.Url = ""
	plugin.DryRun = false
}

func TestLogMsgTimestampLocation(t *testing.T) {
	event := corev2.FixtureEvent("entity1", "check1")
	expectedTimestamp := msTimestamp(event.Timestamp)
	logMsg, err := createLogMsg(event)
	assert.NoError(t, err)
	data := logMsg.Data[0].(map[string]int64)
	assert.Equal(t, data["timestamp"], expectedTimestamp)
}

func TestCheckArgs(t *testing.T) {
	err := checkArgs(nil)
	assert.Error(t, err)
	plugin.EnableSendLog = true
	err = checkArgs(nil)
	assert.Error(t, err)
	plugin.Url = "test"
	err = checkArgs(nil)
	assert.NoError(t, err)
	clearPlugin()
}

func TestConvertMetric(t *testing.T) {
	nsStamp := int64(1624376039373111122)
	usStamp := int64(1624376039373111)
	msStamp := int64(1624376039373)
	sStamp := int64(1624376039)
	msSecond := int64(1624376039000)
	expectedData := `answer{foo="bar", hey="there"} 42 `
	a := [4]int64{msStamp, usStamp, nsStamp, sStamp}
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = corev2.FixtureMetrics()
	for _, p := range event.Metrics.Points {
		p.Tags = append(p.Tags, &corev2.MetricTag{Name: "hey", Value: "there"})
	}
	for _, stamp := range a {
		for _, p := range event.Metrics.Points {
			p.Timestamp = stamp
		}
		dataString, err := convertMetrics(event)
		assert.NoError(t, err)
		msTime := expectedData + fmt.Sprintf("%v\n", msStamp)
		if stamp < msStamp {
			msTime = expectedData + fmt.Sprintf("%v\n", msSecond)
		}
		usTime := expectedData + fmt.Sprintf("%v\n", usStamp)
		nsTime := expectedData + fmt.Sprintf("%v\n", nsStamp)
		sTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v\n", sStamp)
		assert.Equal(t, msTime, dataString)
		assert.NotEqual(t, usTime, dataString)
		assert.NotEqual(t, nsTime, dataString)
		assert.NotEqual(t, sTime, dataString)
	}
}
func TestConvertMetricWithNilTags(t *testing.T) {
	nsStamp := int64(1624376039373111122)
	usStamp := int64(1624376039373111)
	msStamp := int64(1624376039373)
	sStamp := int64(1624376039)
	msSecond := int64(1624376039000)
	a := [4]int64{msStamp, usStamp, nsStamp, sStamp}
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = corev2.FixtureMetrics()
	for _, stamp := range a {
		for _, p := range event.Metrics.Points {
			p.Timestamp = stamp
			p.Tags = nil
		}
		dataString, err := convertMetrics(event)
		assert.NoError(t, err)
		msTime := `answer{} 42 ` + fmt.Sprintf("%v\n", msStamp)
		if stamp < msStamp {
			msTime = `answer{} 42 ` + fmt.Sprintf("%v\n", msSecond)
		}
		usTime := `answer{} 42 ` + fmt.Sprintf("%v\n", usStamp)
		nsTime := `answer{} 42 ` + fmt.Sprintf("%v\n", nsStamp)
		sTime := `answer{} 42 ` + fmt.Sprintf("%v\n", sStamp)
		assert.Equal(t, msTime, dataString)
		assert.NotEqual(t, usTime, dataString)
		assert.NotEqual(t, nsTime, dataString)
		assert.NotEqual(t, sTime, dataString)
	}
}

func TestConvertMetricWithNilMetrics(t *testing.T) {
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = nil
	_, err := convertMetrics(event)
	assert.NoError(t, err)
}
func TestConvertMetricWithNilMetricsPoints(t *testing.T) {
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = corev2.FixtureMetrics()
	event.Metrics.Points = nil
	_, err := convertMetrics(event)
	assert.NoError(t, err)
}

func TestSendMetrics(t *testing.T) {
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = corev2.FixtureMetrics()
	msStamp := int64(1624376039373)
	nsStamp := int64(1624376039373111122)
	msTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v", msStamp)
	for _, p := range event.Metrics.Points {
		p.Timestamp = nsStamp
	}

	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody := msTime
		assert.Equal(t, expectedBody, strings.Trim(string(body), "\n"))
		w.WriteHeader(http.StatusOK)
	}))

	url, err := url.ParseRequestURI(test.URL)
	assert.NoError(t, err)
	plugin.Url = url.String()
	dataString, err := convertMetrics(event)
	assert.NoError(t, err)
	assert.NoError(t, sendMetrics(dataString))
}
func TestSendMetricsDryRun(t *testing.T) {
	plugin.DryRun = true
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = corev2.FixtureMetrics()
	msStamp := int64(1624376039373)
	nsStamp := int64(1624376039373111122)
	msTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v", msStamp)
	for _, p := range event.Metrics.Points {
		p.Timestamp = nsStamp
	}

	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody := msTime
		assert.Equal(t, expectedBody, strings.Trim(string(body), "\n"))
		w.WriteHeader(http.StatusOK)
	}))

	url, err := url.ParseRequestURI(test.URL)
	assert.NoError(t, err)
	plugin.Url = url.String()
	dataString, err := convertMetrics(event)
	assert.NoError(t, err)
	assert.NoError(t, sendMetrics(dataString))
	clearPlugin()
}

func TestSendLog(t *testing.T) {
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = nil
	msgBytes, err := json.Marshal(event)
	assert.NoError(t, err)

	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody := string(msgBytes)
		assert.Equal(t, expectedBody, strings.Trim(string(body), "\n"))
		w.WriteHeader(http.StatusOK)
	}))

	url, err := url.ParseRequestURI(test.URL)
	assert.NoError(t, err)
	plugin.Url = url.String()
	assert.NoError(t, sendLog(string(msgBytes)))
}
func TestSendLogDryRun(t *testing.T) {
	plugin.DryRun = true
	event := corev2.FixtureEvent("entity1", "check1")
	event.Check = nil
	event.Metrics = nil
	msgBytes, err := json.Marshal(event)
	assert.NoError(t, err)

	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		expectedBody := string(msgBytes)
		assert.Equal(t, expectedBody, strings.Trim(string(body), "\n"))
		w.WriteHeader(http.StatusOK)
	}))

	url, err := url.ParseRequestURI(test.URL)
	assert.NoError(t, err)
	plugin.Url = url.String()
	assert.NoError(t, sendLog(string(msgBytes)))
	clearPlugin()
}

func TestMsTimestamp(t *testing.T) {
	event := corev2.FixtureEvent("entity1", "check1")
	event.Metrics = corev2.FixtureMetrics()
	msStamp := int64(1624376039373)
	nsStamp := int64(1624376039373111122)
	secStamp := int64(1624376039)
	expectedStamp := msStamp
	finalStamp := msTimestamp(nsStamp)
	assert.Equal(t, expectedStamp, finalStamp)
	expectedStamp = secStamp * 1000
	finalStamp = msTimestamp(secStamp)
	assert.Equal(t, expectedStamp, finalStamp)
}

func TestExecuteHandler(t *testing.T) {
	plugin.MetricDimensions = `hey=now,this=that`
	plugin.MetricMetadata = `you=me,here=there`
	plugin.LogFields = `near=far,in=out`
	plugin.SourceNameTemplate = defaultNameTemplate
	plugin.SourceHostTemplate = defaultHostTemplate
	plugin.SourceCategoryTemplate = defaultCategoryTemplate

	event := corev2.FixtureEvent("entity1", "check1")
	event.Metrics = corev2.FixtureMetrics()
	msStamp := int64(1624376039373)
	nsStamp := int64(1624376039373111122)
	msTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v", msStamp)
	for _, p := range event.Metrics.Points {
		p.Timestamp = nsStamp
	}
	event.Timestamp = msTimestamp(event.Timestamp)
	logMsg, err := createLogMsg(event)
	assert.NoError(t, err)
	expectedBytes, err := json.Marshal(logMsg)
	assert.NoError(t, err)
	plugin.EnableSendLog = true
	plugin.EnableSendMetrics = true
	results := 0
	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		switch {
		case contains(r.Header["Content-Type"], "application/vnd.sumologic.prometheus"):
			// recieved metrics with Content-Type header set
			expectedBody := msTime
			assert.Equal(t, expectedBody, strings.Trim(string(body), "\n"))
			assert.Equal(t, plugin.MetricDimensions, r.Header["X-Sumo-Dimensions"][0])
			assert.Equal(t, plugin.MetricMetadata, r.Header["X-Sumo-Metadata"][0])
			results++
		case contains(r.Header["Content-Type"], "application/json"):
			// recieved log with Content-Type header unset
			expectedBody := string(expectedBytes)
			assert.Equal(t, expectedBody, strings.Trim(string(body), "\n"))
			assert.Equal(t, plugin.LogFields, r.Header["X-Sumo-Fields"][0])
			results++
		default:
			assert.FailNow(t, "No Content-Type Header")

		}
		if len(plugin.SourceNameTemplate) > 0 {
			assert.Equal(t, plugin.SourceName, r.Header["X-Sumo-Name"][0])
		} else {
			assert.Nil(t, r.Header["X-Sumo-Name"])
		}
		if len(plugin.SourceHostTemplate) > 0 {
			assert.Equal(t, plugin.SourceHost, r.Header["X-Sumo-Host"][0])
		} else {
			assert.Nil(t, r.Header["X-Sumo-Host"])
		}
		if len(plugin.SourceCategoryTemplate) > 0 {
			assert.Equal(t, plugin.SourceCategory, r.Header["X-Sumo-Category"][0])
		} else {
			assert.Nil(t, r.Header["X-Sumo-Category"])
		}
		w.WriteHeader(http.StatusOK)
	}))

	url, err := url.ParseRequestURI(test.URL)
	assert.NoError(t, err)
	plugin.Url = url.String()
	err = executeHandler(event)
	assert.Equal(t, results, 2)
	assert.NoError(t, err)
}

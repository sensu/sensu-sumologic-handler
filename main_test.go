package main

import (
	"fmt"
	"testing"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/stretchr/testify/assert"
)

func TestConvertMetric(t *testing.T) {
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
		}
		dataString, err := convertMetrics(event)
		assert.NoError(t, err)
		msTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v\n", msStamp)
		if stamp < msStamp {
			msTime = `answer{foo="bar"} 42 ` + fmt.Sprintf("%v\n", msSecond)
		}
		usTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v\n", usStamp)
		nsTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v\n", nsStamp)
		sTime := `answer{foo="bar"} 42 ` + fmt.Sprintf("%v\n", sStamp)
		assert.Equal(t, msTime, dataString)
		assert.NotEqual(t, usTime, dataString)
		assert.NotEqual(t, nsTime, dataString)
		assert.NotEqual(t, sTime, dataString)
	}
}

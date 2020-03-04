package main

import (
	"context"
	"github.com/PagerDuty/go-pagerduty"
	"sync"
	"time"
)

type CollectorGeneral struct {
	CollectorBase
	Processor CollectorProcessorGeneralInterface

	PagerDutyClient *pagerduty.Client
	errorCounter int
}

func (m *CollectorGeneral) Run(scrapeTime time.Duration) {
	m.SetScrapeTime(scrapeTime)

	m.Processor.Setup(m)
	go func() {
		for {
			go func() {
				m.Collect()
			}()
			m.sleepUntilNextCollection()
		}
	}()
}

func (m *CollectorGeneral) Collect() {
	defer func() {
		if r := recover(); r != nil {
			m.errorCounter++

			daemonLogger.Error(r)
			if m.errorCounter > CollectorErrorThreshold {
				panic("Error threshold reached, stopping exporter")
			}
		}
	}()

	var wg sync.WaitGroup
	var wgCallback sync.WaitGroup

	ctx := context.Background()

	callbackChannel := make(chan func())

	m.collectionStart()

	// collect metrics (callbacks) and proceses them
	wgCallback.Add(1)
	go func() {
		defer wgCallback.Done()
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		// reset metric values
		m.Processor.Reset()

		// process callbacks (set metrics)
		for _, callback := range callbackList {
			callback()
		}
	}()

	m.Processor.Collect(ctx, callbackChannel)

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
	wgCallback.Wait()

	m.collectionFinish()
	m.errorCounter = 0
}

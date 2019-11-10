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
	var wg sync.WaitGroup
	var wgCallback sync.WaitGroup

	ctx := context.Background()

	callbackChannel := make(chan func())

	m.collectionStart()

	wg.Add(1)
	go func(ctx context.Context, callback chan<- func()) {
		defer wg.Done()
		m.Processor.Collect(ctx, callbackChannel)
	}(ctx, callbackChannel)

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

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
	wgCallback.Wait()

	m.collectionFinish()
}

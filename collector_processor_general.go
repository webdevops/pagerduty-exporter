package main

import (
	"context"
	log "github.com/sirupsen/logrus"
)

type CollectorProcessorGeneralInterface interface {
	Setup(collector *CollectorGeneral)
	Reset()
	Collect(ctx context.Context, callback chan<- func())
}

type CollectorProcessorGeneral struct {
	CollectorProcessorGeneralInterface
	CollectorReference *CollectorGeneral
}

func NewCollectorGeneral(name string, processor CollectorProcessorGeneralInterface) *CollectorGeneral {
	base := CollectorBase{
		Name: name,
	}
	base.Init()

	collector := CollectorGeneral{
		CollectorBase: base,
		Processor:     processor,
	}

	return &collector
}

func (c *CollectorProcessorGeneral) logger() *log.Entry {
	return c.CollectorReference.logger
}

package graylog

import (
	"strings"

	"sync/atomic"

	"github.com/pborman/uuid"

	"github.com/mozilla-services/heka/pipeline"
	"github.com/mozilla-services/heka/message"

	"github.com/Graylog2/go-gelf/gelf"
)

type GraylogInputConfig struct{
	Address string `toml:"address"`
}

type GraylogInput struct {
	config *GraylogInputConfig
	reader *gelf.Reader

	stopChan chan bool

	processMessageCount int64
	processMessageFailures int64
}

func (g *GraylogInput) ConfigStruct() interface{} {
	return &GraylogInputConfig{
	}
}

func (g *GraylogInput) Init(config interface{}) (err error) {
	g.config = config.(*GraylogInputConfig)	
	g.stopChan = make(chan bool)
	g.reader,err = gelf.NewReader(g.config.Address)
	if err != nil {
		return
	}

	return
}

func (g *GraylogInput) Run(ir pipeline.InputRunner, h pipeline.PluginHelper) (err error) {
	GRAYLOG_READ_LOOP:
	for {
		select {
		case <- g.stopChan:
			break GRAYLOG_READ_LOOP
		default:
			msg,err := g.reader.ReadMessage()
			if err != nil {
				if err.Error() == "out-of-band message (not chunked)" {
					ir.LogError(err)
					atomic.AddInt64(&g.processMessageFailures, 1)
					err = nil
					continue
				} else {
					break GRAYLOG_READ_LOOP
				}
			}
			atomic.AddInt64(&g.processMessageCount, 1)

			pack := <-ir.InChan()
			pack.Message.SetUuid(uuid.NewRandom())
			pack.Message.SetTimestamp(int64(msg.TimeUnix) * 1000000000)
			pack.Message.SetType("graylog")
			pack.Message.SetHostname(msg.Host)
			pack.Message.SetSeverity(msg.Level)
			pack.Message.SetLogger(g.config.Address)

			if msg.Full != "" {
				pack.Message.SetPayload(msg.Full)
			} else {
				pack.Message.SetPayload(msg.Short)
			}
			for k,v := range msg.Extra {
				cleanedK := cleanKeyForKibana(k)
				field,err := message.NewField(cleanedK, v, "")
				if err != nil {
					ir.LogError(err)
					continue
				}
				pack.Message.AddField(field)
			}

			ir.Deliver(pack)
		}
	}

	return
}

func (g *GraylogInput) ReportMsg(msg *message.Message) error {
	message.NewInt64Field(msg, "ProcessMessageCount",
		atomic.LoadInt64(&g.processMessageCount), "count")
	message.NewInt64Field(msg, "ProcessMessageFailures",
		atomic.LoadInt64(&g.processMessageFailures), "count")
	return nil
}

func cleanKeyForKibana(k string) (output string) {
	return strings.TrimPrefix(k, "_")
}

func (g *GraylogInput) Stop() {
	close(g.stopChan)
}

func init() {
	pipeline.RegisterPlugin("GraylogInput", func() interface{} {
		return new(GraylogInput)
	})
}
package main

import (
	"C"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"unsafe"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/fluent/fluent-bit-go/output"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var mLogs = stats.Int64("logCount", "Number of lines of log output", "1")

type LogExporterInstance struct {
	labelMapper map[string]tag.Key
	stats       *stats.Int64Measure
}

type LogExporter struct {
	mtx       sync.Mutex
	listen    string
	server    *http.Server
	instances []*LogExporterInstance
}

func (e *LogExporter) defaultListenAddress() string {
	if e.listen != "" {
		return e.listen
	}
	return "0.0.0.0:8681"
}

func (e *LogExporter) Start(listen string) error {
	if listen == "" {
		listen = e.defaultListenAddress()
	}
	if e.server == nil {
		e.listen = listen
		pe, err := prometheus.NewExporter(prometheus.Options{
			Namespace: "logexporter",
		})
		if err != nil {
			return fmt.Errorf("initialize prometheus instance: %w", err)
		}
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		mux.Handle("/health", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("OK"))
		}))
		e.server = &http.Server{
			Addr:    e.listen,
			Handler: mux,
		}
		listener, err := net.Listen("tcp", e.listen) // adjust arguments as needed.
		if err != nil {
			return fmt.Errorf("listen error(%s): %w", e.listen, err)
		}
		log.Printf("logexporter is listening on %s", e.listen)
		go func() {
			if err := e.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("[ERROR] failed to serve logexporter: %v", err)
			}
		}()
	} else if e.listen != listen {
		return fmt.Errorf("several different listen ddresses have been set(first=%s, second=%s)", e.listen, listen)
	}
	return nil
}

func (e *LogExporter) Stop(ctx context.Context) error {
	if e.server != nil {
		if err := e.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("stop http server: %w", err)
		}
	}
	return nil
}

func (e *LogExporter) CreateInstance(labels string, viewName string) (int, error) {
	keys := make([]tag.Key, 0)
	labelMapper := make(map[string]tag.Key)
	for _, s := range strings.Split(labels, ",") {
		if s != "" {
			values := strings.Split(s, "=")
			labelMapper[values[1]] = tag.MustNewKey(values[0])
			keys = append(keys, labelMapper[values[1]])
		}
	}
	if viewName == "" {
		viewName = "log_count"
	}
	mLogs := stats.Int64("logCount", "Number of lines of log output", "1")
	logView := &view.View{
		Name:        viewName,
		Measure:     mLogs,
		Description: "Number of lines of log output",
		TagKeys:     keys,
		Aggregation: view.Sum(),
	}
	if err := view.Register(logView); err != nil {
		return 0, fmt.Errorf("register log view: %w", err)
	}
	e.instances = append(e.instances, &LogExporterInstance{
		labelMapper: labelMapper,
		stats:       mLogs,
	})
	return len(e.instances) - 1, nil
}

var exporter LogExporter

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(def, "logexporter", "Export log count as Prometheus Metrics")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	listen := output.FLBPluginConfigKey(plugin, "listen")
	if err := exporter.Start(listen); err != nil {
		log.Printf("[ERROR] Failed to init logexporter: %+v", err)
		return output.FLB_ERROR
	}

	labels := output.FLBPluginConfigKey(plugin, "labels")
	viewName := output.FLBPluginConfigKey(plugin, "view_name")

	if id, err := exporter.CreateInstance(labels, viewName); err != nil {
		log.Printf("failed to register log view: %+v", err)
		return output.FLB_ERROR
	} else {
		output.FLBPluginSetContext(plugin, id)
	}
	return output.FLB_OK
}

func recordToStr(v interface{}) string {
	return fmt.Sprint(v)
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tagName *C.char) int {
	id := output.FLBPluginGetContext(ctx).(int)
	instance := exporter.instances[id]
	// Create Fluent Bit decoder
	dec := output.NewDecoder(data, int(length))
	for {
		// Extract Record
		ret, _, record := output.GetRecord(dec)
		if ret != 0 {
			break
		}

		mutators := make([]tag.Mutator, 0)
		for k, v := range record {
			if tagKey, ok := instance.labelMapper[recordToStr(k)]; ok {
				mutators = append(mutators, tag.Upsert(tagKey, recordToStr(v)))
			}
		}
		ctx, err := tag.New(context.Background(), mutators...)
		if err != nil {
			log.Printf("failed to record stats: %+v", err)
		} else {
			stats.Record(ctx, mLogs.M(1))
		}
	}
	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	if err := exporter.Stop(context.Background()); err != nil {
		log.Printf("[ERROR] Failed to stop logexporter: %+v", err)
		return output.FLB_ERROR
	}
	return output.FLB_OK
}

func main() {
}

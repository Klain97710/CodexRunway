//go:build darwin

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

var widgetRequests sync.Map
var widgetRequestID atomic.Uint64
var widgetCallbackOnce sync.Once
var widgetCallback uintptr

func loadWidgetNative() (*widgetNative, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(exe), "..", "Frameworks", "libQuosparrWidgetBridge.dylib")
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("WidgetKit bridge unavailable in this build: %w", err)
	}
	// Retain the library for the lifetime of the process: WidgetKit may deliver
	// an asynchronous callback after the coordinator has been stopped.
	for _, name := range []string{"quosparr_widget_bridge_version", "quosparr_widget_query", "quosparr_widget_reload"} {
		if _, err := purego.Dlsym(h, name); err != nil {
			return nil, err
		}
	}
	var version func() int32
	var query func(uint64, uintptr)
	var reload func(string) int32
	purego.RegisterLibFunc(&version, h, "quosparr_widget_bridge_version")
	purego.RegisterLibFunc(&query, h, "quosparr_widget_query")
	purego.RegisterLibFunc(&reload, h, "quosparr_widget_reload")
	if version() != 1 {
		return nil, errors.New("WidgetKit bridge has an unsupported ABI version")
	}
	widgetCallbackOnce.Do(func() {
		widgetCallback = purego.NewCallback(func(id uint64, ptr *byte, length uintptr) {
			value, ok := widgetRequests.Load(id)
			if !ok {
				return
			}
			if ptr == nil || length > 1<<20 {
				value.(chan []byte) <- []byte(`{"error":"WidgetKit bridge response exceeds the size limit"}`)
				return
			}
			bytes := unsafe.Slice(ptr, int(length))
			value.(chan []byte) <- append([]byte(nil), bytes...)
		})
	})
	return &widgetNative{
		configurations: func(ctx context.Context) ([]widgetConfiguration, error) {
			id := widgetRequestID.Add(1)
			result := make(chan []byte, 1)
			widgetRequests.Store(id, result)
			defer widgetRequests.Delete(id)
			query(id, widgetCallback)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case data := <-result:
				var reply struct {
					Configurations []widgetConfiguration `json:"configurations"`
					Error          string                `json:"error"`
				}
				if err := json.Unmarshal(data, &reply); err != nil {
					return nil, err
				}
				if reply.Error != "" {
					return nil, errors.New(reply.Error)
				}
				return reply.Configurations, nil
			}
		},
		reload: func(kinds []string) error {
			data, err := json.Marshal(kinds)
			if err != nil {
				return err
			}
			if code := reload(string(data)); code != 0 {
				return fmt.Errorf("WidgetKit timeline reload failed (%d)", code)
			}
			return nil
		},
	}, nil
}

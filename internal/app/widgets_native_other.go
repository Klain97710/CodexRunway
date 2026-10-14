//go:build !darwin

package app

import "errors"

func loadWidgetNative() (*widgetNative, error) {
	return nil, errors.New("WidgetKit is unavailable on this platform")
}

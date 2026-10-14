//go:build darwin

package netx

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CFNetwork is the proxy source URLSession uses, so this resolver keeps the
// Swift app's behaviour: manual HTTP/HTTPS/SOCKS proxies, the exceptions list,
// and automatic configuration (PAC), either from a URL or an inline script.

const (
	cfStringEncodingUTF8 = 0x08000100
	pacTimeout           = 10 * time.Second
	pacPollSeconds       = 0.05
)

type cfProxyAPI struct {
	copySettings func() uintptr
	copyForURL   func(uintptr, uintptr) uintptr
	execPACURL   func(pac, target, callback uintptr, ctx *cfStreamClientContext) uintptr
	execPACJS    func(script, target, callback uintptr, ctx *cfStreamClientContext) uintptr
	urlWithBytes func(uintptr, unsafe.Pointer, int64, uint32, uintptr) uintptr
	urlString    func(uintptr) uintptr
	arrayCount   func(uintptr) int64
	arrayValue   func(uintptr, int64) uintptr
	dictValue    func(uintptr, uintptr) uintptr
	equal        func(uintptr, uintptr) uint8
	cfString     func(uintptr, unsafe.Pointer, int64, uint32) uint8
	number       func(uintptr, int64, unsafe.Pointer) uint8
	retain       func(uintptr) uintptr
	release      func(uintptr)

	runLoopCurrent func() uintptr
	runLoopAdd     func(rl, source, mode uintptr)
	runLoopRemove  func(rl, source, mode uintptr)
	runLoopRun     func(mode uintptr, seconds float64, returnAfterSource uint8) int32
	sourceInvalid  func(source uintptr)

	typeKey, hostKey, portKey                uintptr
	pacURLKey, pacScriptKey                  uintptr
	typeNone, typeHTTP, typeHTTPS, typeSOCKS uintptr
	typePACURL, typePACJavaScript            uintptr
	defaultMode                              uintptr
	pacCallback                              uintptr
}

// cfStreamClientContext is CFStreamClientContext.
type cfStreamClientContext struct {
	version                          int64
	info                             uintptr
	retain, release, copyDescription uintptr
}

var (
	cfProxyOnce sync.Once
	cfProxy     cfProxyAPI
	cfProxyErr  error

	pacMu      sync.Mutex
	pacNextID  uintptr
	pacPending = map[uintptr]*pacState{}
)

type pacState struct {
	done    bool
	proxies uintptr // retained list, valid when err is false
	failed  bool
}

func osSystemProxy(target *url.URL) (*url.URL, error) {
	api, err := loadCFProxyAPI()
	if err != nil {
		return nil, err
	}
	settings := api.copySettings()
	if settings == 0 {
		return nil, errors.New("system proxy: CFNetwork returned no proxy settings")
	}
	defer api.release(settings)
	return api.proxyForSettings(target, settings)
}

func (api *cfProxyAPI) proxyForSettings(target *url.URL, settings uintptr) (*url.URL, error) {
	cfURL, err := api.newURL(target.String())
	if err != nil {
		return nil, err
	}
	defer api.release(cfURL)
	proxies := api.copyForURL(cfURL, settings)
	if proxies == 0 {
		return nil, errors.New("system proxy: CFNetwork could not resolve the proxy")
	}
	defer api.release(proxies)
	return api.firstProxy(proxies, cfURL)
}

// firstProxy takes the first entry of a proxy list. Failover to later entries
// is not attempted: a dead first proxy fails the request instead of silently
// trying another route.
func (api *cfProxyAPI) firstProxy(proxies, targetURL uintptr) (*url.URL, error) {
	if api.arrayCount(proxies) == 0 {
		return nil, errors.New("system proxy: CFNetwork returned an empty proxy list")
	}
	dict := api.arrayValue(proxies, 0)
	kind := api.dictValue(dict, api.typeKey)
	switch {
	case kind == 0:
		return nil, errors.New("system proxy: proxy type is missing")
	case api.equal(kind, api.typePACURL) != 0:
		pac := api.dictValue(dict, api.pacURLKey)
		if pac == 0 {
			return nil, errors.New("system proxy: PAC URL is missing")
		}
		return api.runPAC(pac, 0, targetURL)
	case api.equal(kind, api.typePACJavaScript) != 0:
		script := api.dictValue(dict, api.pacScriptKey)
		if script == 0 {
			return nil, errors.New("system proxy: PAC script is missing")
		}
		return api.runPAC(0, script, targetURL)
	}
	return api.proxyURL(dict)
}

func (api *cfProxyAPI) proxyURL(dict uintptr) (*url.URL, error) {
	kind := api.dictValue(dict, api.typeKey)
	if kind == 0 {
		return nil, errors.New("system proxy: proxy type is missing")
	}
	if api.equal(kind, api.typeNone) != 0 {
		return nil, nil
	}
	scheme := ""
	switch {
	case api.equal(kind, api.typeHTTP) != 0, api.equal(kind, api.typeHTTPS) != 0:
		scheme = "http"
	case api.equal(kind, api.typeSOCKS) != 0:
		scheme = "socks5"
	default:
		return nil, errors.New("system proxy: CFNetwork returned an unsupported proxy type")
	}
	host, err := api.stringValue(api.dictValue(dict, api.hostKey))
	if err != nil {
		return nil, err
	}
	var port int64
	if number := api.dictValue(dict, api.portKey); number == 0 || api.number(number, 4, unsafe.Pointer(&port)) == 0 {
		return nil, errors.New("system proxy: proxy port is missing or invalid")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("system proxy: invalid proxy port %d", port)
	}
	return &url.URL{Scheme: scheme, Host: net.JoinHostPort(host, fmt.Sprint(port))}, nil
}

// runPAC evaluates a PAC (URL or script) for targetURL on a run loop of the
// calling thread. The PAC result callback is delivered inside the run loop, so
// the goroutine is locked to its thread for the duration.
func (api *cfProxyAPI) runPAC(pacURL, script, targetURL uintptr) (*url.URL, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pacMu.Lock()
	pacNextID++
	id := pacNextID
	state := &pacState{}
	pacPending[id] = state
	pacMu.Unlock()
	defer func() {
		pacMu.Lock()
		delete(pacPending, id)
		pacMu.Unlock()
	}()

	ctx := &cfStreamClientContext{info: id}
	var source uintptr
	if pacURL != 0 {
		source = api.execPACURL(pacURL, targetURL, api.pacCallback, ctx)
	} else {
		source = api.execPACJS(script, targetURL, api.pacCallback, ctx)
	}
	if source == 0 {
		return nil, errors.New("system proxy: cannot start PAC evaluation")
	}
	loop := api.runLoopCurrent()
	api.runLoopAdd(loop, source, api.defaultMode)
	deadline := time.Now().Add(pacTimeout)
	for {
		pacMu.Lock()
		done := state.done
		pacMu.Unlock()
		if done || time.Now().After(deadline) {
			break
		}
		api.runLoopRun(api.defaultMode, pacPollSeconds, 1)
	}
	api.runLoopRemove(loop, source, api.defaultMode)
	api.sourceInvalid(source)
	api.release(source)

	pacMu.Lock()
	done, failed, proxies := state.done, state.failed, state.proxies
	state.proxies = 0
	pacMu.Unlock()
	switch {
	case !done:
		return nil, errors.New("system proxy: PAC evaluation timed out")
	case failed || proxies == 0:
		return nil, errors.New("system proxy: PAC evaluation failed")
	}
	defer api.release(proxies)
	if api.arrayCount(proxies) == 0 {
		return nil, errors.New("system proxy: PAC returned an empty proxy list")
	}
	return api.proxyURL(api.arrayValue(proxies, 0))
}

// pacResult is the CFProxyAutoConfigurationResultCallback. Its arguments are
// borrowed, so the list is retained before the run loop returns.
func pacResult(client, proxies, cfError uintptr) uintptr {
	pacMu.Lock()
	defer pacMu.Unlock()
	state := pacPending[client]
	if state == nil {
		return 0
	}
	state.done = true
	if cfError != 0 || proxies == 0 {
		state.failed = true
		return 0
	}
	state.proxies = cfProxy.retain(proxies)
	return 0
}

func (api *cfProxyAPI) newURL(value string) (uintptr, error) {
	raw := []byte(value)
	if len(raw) == 0 {
		return 0, errors.New("system proxy: empty target URL")
	}
	cfURL := api.urlWithBytes(0, unsafe.Pointer(&raw[0]), int64(len(raw)), cfStringEncodingUTF8, 0)
	if cfURL == 0 {
		return 0, errors.New("system proxy: cannot create the target URL")
	}
	return cfURL, nil
}

func (api *cfProxyAPI) stringValue(value uintptr) (string, error) {
	if value == 0 {
		return "", errors.New("system proxy: proxy host is missing")
	}
	buf := make([]byte, 4096)
	if api.cfString(value, unsafe.Pointer(&buf[0]), int64(len(buf)), cfStringEncodingUTF8) == 0 {
		return "", errors.New("system proxy: proxy host is invalid")
	}
	for i, b := range buf {
		if b == 0 {
			if i == 0 {
				return "", errors.New("system proxy: proxy host is empty")
			}
			return string(buf[:i]), nil
		}
	}
	return "", errors.New("system proxy: proxy host is too long")
}

// evaluatePACURL resolves target through the PAC at pacURL. It is the same
// path a system PAC setting takes and is also what the tests drive.
func evaluatePACURL(pacURL string, target *url.URL) (*url.URL, error) {
	api, err := loadCFProxyAPI()
	if err != nil {
		return nil, err
	}
	pac, err := api.newURL(pacURL)
	if err != nil {
		return nil, err
	}
	defer api.release(pac)
	targetURL, err := api.newURL(target.String())
	if err != nil {
		return nil, err
	}
	defer api.release(targetURL)
	return api.runPAC(pac, 0, targetURL)
}

func loadCFProxyAPI() (*cfProxyAPI, error) {
	cfProxyOnce.Do(func() {
		cfNetwork, err := purego.Dlopen("/System/Library/Frameworks/CFNetwork.framework/CFNetwork", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err != nil {
			cfProxyErr = fmt.Errorf("system proxy: load CFNetwork: %w", err)
			return
		}
		coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err != nil {
			cfProxyErr = fmt.Errorf("system proxy: load CoreFoundation: %w", err)
			return
		}
		for _, fn := range []struct {
			dst  any
			lib  uintptr
			name string
		}{
			{&cfProxy.copySettings, cfNetwork, "CFNetworkCopySystemProxySettings"},
			{&cfProxy.copyForURL, cfNetwork, "CFNetworkCopyProxiesForURL"},
			{&cfProxy.execPACURL, cfNetwork, "CFNetworkExecuteProxyAutoConfigurationURL"},
			{&cfProxy.execPACJS, cfNetwork, "CFNetworkExecuteProxyAutoConfigurationScript"},
			{&cfProxy.urlWithBytes, coreFoundation, "CFURLCreateWithBytes"},
			{&cfProxy.urlString, coreFoundation, "CFURLGetString"},
			{&cfProxy.arrayCount, coreFoundation, "CFArrayGetCount"},
			{&cfProxy.arrayValue, coreFoundation, "CFArrayGetValueAtIndex"},
			{&cfProxy.dictValue, coreFoundation, "CFDictionaryGetValue"},
			{&cfProxy.equal, coreFoundation, "CFEqual"},
			{&cfProxy.cfString, coreFoundation, "CFStringGetCString"},
			{&cfProxy.number, coreFoundation, "CFNumberGetValue"},
			{&cfProxy.retain, coreFoundation, "CFRetain"},
			{&cfProxy.release, coreFoundation, "CFRelease"},
			{&cfProxy.runLoopCurrent, coreFoundation, "CFRunLoopGetCurrent"},
			{&cfProxy.runLoopAdd, coreFoundation, "CFRunLoopAddSource"},
			{&cfProxy.runLoopRemove, coreFoundation, "CFRunLoopRemoveSource"},
			{&cfProxy.runLoopRun, coreFoundation, "CFRunLoopRunInMode"},
			{&cfProxy.sourceInvalid, coreFoundation, "CFRunLoopSourceInvalidate"},
		} {
			if cfProxyErr = registerCFFunction(fn.dst, fn.lib, fn.name); cfProxyErr != nil {
				return
			}
		}
		for _, value := range []struct {
			dst  *uintptr
			lib  uintptr
			name string
		}{
			{&cfProxy.typeKey, cfNetwork, "kCFProxyTypeKey"}, {&cfProxy.hostKey, cfNetwork, "kCFProxyHostNameKey"},
			{&cfProxy.portKey, cfNetwork, "kCFProxyPortNumberKey"}, {&cfProxy.typeNone, cfNetwork, "kCFProxyTypeNone"},
			{&cfProxy.typeHTTP, cfNetwork, "kCFProxyTypeHTTP"}, {&cfProxy.typeHTTPS, cfNetwork, "kCFProxyTypeHTTPS"},
			{&cfProxy.typeSOCKS, cfNetwork, "kCFProxyTypeSOCKS"}, {&cfProxy.typePACURL, cfNetwork, "kCFProxyTypeAutoConfigurationURL"},
			{&cfProxy.typePACJavaScript, cfNetwork, "kCFProxyTypeAutoConfigurationJavaScript"},
			{&cfProxy.pacURLKey, cfNetwork, "kCFProxyAutoConfigurationURLKey"},
			{&cfProxy.pacScriptKey, cfNetwork, "kCFProxyAutoConfigurationJavaScriptKey"},
			{&cfProxy.defaultMode, coreFoundation, "kCFRunLoopDefaultMode"},
		} {
			if *value.dst, cfProxyErr = cfStringConstant(value.lib, value.name); cfProxyErr != nil {
				return
			}
		}
		cfProxy.pacCallback = purego.NewCallback(pacResult)
	})
	return &cfProxy, cfProxyErr
}

func registerCFFunction(dst any, lib uintptr, name string) error {
	symbol, err := purego.Dlsym(lib, name)
	if err != nil {
		return fmt.Errorf("system proxy: load %s: %w", name, err)
	}
	purego.RegisterFunc(dst, symbol)
	return nil
}

func cfStringConstant(lib uintptr, name string) (uintptr, error) {
	symbol, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0, fmt.Errorf("system proxy: load %s: %w", name, err)
	}
	return **(**uintptr)(unsafe.Pointer(&symbol)), nil
}

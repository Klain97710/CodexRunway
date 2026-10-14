package netx

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"runtime"
	"strings"
)

// SystemProxyFunc resolves the proxy for target from the operating system
// settings. A nil URL with a nil error means "connect directly" because the
// system says so (no proxy configured, or the host is on the bypass list). Any
// error stops the request: it never turns into a direct connection.
type SystemProxyFunc func(target *url.URL) (*url.URL, error)

// DefaultSystemProxy is the system proxy source of this platform.
//
//   - macOS: CFNetwork system settings (manual HTTP/HTTPS/SOCKS, exceptions and
//     PAC), the same source URLSession uses. Environment variables are not
//     consulted, like the Swift app.
//   - Windows: HTTP(S)_PROXY / NO_PROXY first, then the WinHTTP current-user
//     Internet settings (manual proxy, bypass list, auto-detect, PAC).
//   - Linux: HTTP(S)_PROXY / ALL_PROXY / NO_PROXY first, then the GNOME
//     gsettings or KDE kioslaverc manual proxy settings. PAC is not supported
//     and is reported as an error.
func DefaultSystemProxy() SystemProxyFunc {
	if runtime.GOOS == "darwin" {
		return osSystemProxy
	}
	return systemProxyWithEnvironment(osSystemProxy)
}

// systemProxyWithEnvironment gives the environment priority over the OS
// settings, as command line tools expect.
func systemProxyWithEnvironment(system SystemProxyFunc) SystemProxyFunc {
	return func(target *url.URL) (*url.URL, error) {
		if target == nil || target.Hostname() == "" {
			return nil, fmt.Errorf("system proxy: target URL is missing a host")
		}
		if raw := environmentProxyValue(target.Scheme); raw != "" {
			if isLoopbackHost(target.Hostname()) || noProxyMatch(target) {
				return nil, nil
			}
			return parseEnvironmentProxy(raw)
		}
		if noProxyMatch(target) {
			return nil, nil
		}
		return system(target)
	}
}

func environmentProxyValue(scheme string) string {
	names := []string{"HTTP_PROXY", "http_proxy"}
	if scheme == "https" {
		names = []string{"HTTPS_PROXY", "https_proxy"}
	}
	names = append(names, "ALL_PROXY", "all_proxy")
	for _, name := range names {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return v
		}
	}
	return ""
}

func parseEnvironmentProxy(raw string) (*url.URL, error) {
	value := raw
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	proxy, err := url.Parse(value)
	// The message must not repeat raw: it may carry a password.
	if err != nil || proxy.Hostname() == "" {
		return nil, fmt.Errorf("system proxy: invalid proxy address in the environment")
	}
	switch proxy.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("system proxy: unsupported proxy scheme %q in the environment", proxy.Scheme)
	}
	if port := proxy.Port(); port == "" && proxy.Scheme != "http" && proxy.Scheme != "https" {
		return nil, fmt.Errorf("system proxy: the SOCKS proxy in the environment has no port")
	}
	return proxy, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// noProxyMatch applies NO_PROXY / no_proxy to target.
func noProxyMatch(target *url.URL) bool {
	value := os.Getenv("NO_PROXY")
	if value == "" {
		value = os.Getenv("no_proxy")
	}
	host := strings.ToLower(target.Hostname())
	port := target.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[target.Scheme]
	}
	ip, ipErr := netip.ParseAddr(host)
	for _, raw := range strings.Split(value, ",") {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "" {
			continue
		}
		if entry == "*" {
			return true
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil && ipErr == nil && prefix.Contains(ip) {
			return true
		}
		entryHost, entryPort := splitProxyHostPort(entry)
		if entryPort != "" && entryPort != port {
			continue
		}
		if parsed, err := netip.ParseAddr(entryHost); err == nil {
			if ipErr == nil && parsed == ip {
				return true
			}
			continue
		}
		matchHost := !strings.HasPrefix(entryHost, ".") && !strings.HasPrefix(entryHost, "*.")
		entryHost = strings.TrimPrefix(entryHost, "*")
		if !strings.HasPrefix(entryHost, ".") {
			entryHost = "." + entryHost
		}
		if strings.HasSuffix(host, entryHost) || matchHost && host == entryHost[1:] {
			return true
		}
	}
	return false
}

func splitProxyHostPort(value string) (string, string) {
	if host, port, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]"), port
	}
	return strings.Trim(value, "[]"), ""
}

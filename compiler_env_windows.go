//go:build windows

package main

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func compilerEnvironment() []string {
	env := os.Environ()
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" {
		return env
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return env
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return env
	}
	proxy, _, err := key.GetStringValue("ProxyServer")
	if err != nil || proxy == "" {
		return env
	}
	if strings.Contains(proxy, "=") {
		for _, item := range strings.Split(proxy, ";") {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) == 2 && (parts[0] == "https" || parts[0] == "http") {
				proxy = parts[1]
				break
			}
		}
	}
	if !strings.Contains(proxy, "://") {
		proxy = "http://" + proxy
	}
	return append(env, "HTTP_PROXY="+proxy, "HTTPS_PROXY="+proxy)
}

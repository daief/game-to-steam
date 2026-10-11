//go:build !windows

package network

import "net/http"

var systemProxy = http.ProxyFromEnvironment

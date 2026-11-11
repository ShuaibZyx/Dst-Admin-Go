package global

import (
	"dst-admin-go/collect"
	"dst-admin-go/config"
	"net/http/httputil"
	"net/url"
)

type Route struct {
	Proxy *httputil.ReverseProxy
	Url   *url.URL
}

var RoutingTable = make(map[string]*Route)

var Config *config.Config

const ClusterToken = "pds-g^KU_qE7e8rv1^VVrVXd/01kBDicd7UO5LeL+uYZH1+geZlrutzItvOaw="

// DefaultSteamAPIKey 内置默认 Steam Web API key（未配置 steamAPIKey 时回退）
const DefaultSteamAPIKey = "73DF9F781D195DFD3D19DED1CB72EEE6"

// SteamAPIKey 返回可用的 Steam API key：优先配置项，未配置回退内置默认（A5 后续）
func SteamAPIKey() string {
	if Config != nil && Config.SteamAPIKey != "" {
		return Config.SteamAPIKey
	}
	return DefaultSteamAPIKey
}

var CollectMap = collect.NewCollectMap()

var Collect *collect.Collect

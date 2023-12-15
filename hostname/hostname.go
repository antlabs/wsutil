package hostname

import (
	"fmt"
	"net/url"
	"strings"
)

func GetHostName(u *url.URL) (hostName string) {
	hostName = u.Hostname()
	if u.Port() == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			hostName += ":443"
		case "http":
			hostName += ":80"
		default:
			panic(fmt.Sprintf("unknown scheme:%s", u.Scheme))
		}
		return
	}

	return u.Host
}

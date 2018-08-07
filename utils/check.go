package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/wrfly/gus-proxy/prox"
	"github.com/wrfly/gus-proxy/types"
)

// CheckProxyAvailable checks if the proxy is available
func CheckProxyAvailable(host *types.ProxyHost) error {
	logrus.Debugf("CheckProxyAvailable [%s]", host.Addr)
	proxyURL, err := url.Parse(host.Addr)
	if proxyURL.Scheme == "direct" {
		return nil
	}
	if err != nil {
		logrus.Debugf("Proxy [%s] is not available, error: %s", host.Addr, err)
		return err
	}

	conn, err := net.DialTimeout("tcp", proxyURL.Host, 1*time.Second)
	if err != nil {
		logrus.Debugf("Proxy [%s] is not available, error: %s", host.Addr, err)
		return err
	}
	defer conn.Close()

	oHosts := []*types.ProxyHost{
		{
			Available: true,
			Addr:      host.Addr,
		},
	}
	p, err := prox.New(oHosts)
	if err != nil || len(p) != 1 {
		logrus.Debugf("Proxy [%s] is not available, error: %s", host.Addr, err)
		return fmt.Errorf("Check proxy error: %s", err)
	}
	clnt := &http.Client{
		Transport: p[0].GoProxy.Tr,
		Timeout:   3 * time.Second,
	}

	_, err = clnt.Get("https://www.baidu.com/home/msg/data/personalcontent")
	if err != nil {
		logrus.Debugf("Proxy [%s] is not available, error: %s", host.Addr, err)
		return err
	}

	return nil
}

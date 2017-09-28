package config

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/wrfly/gus-proxy/round"
	"github.com/wrfly/gus-proxy/types"
	"github.com/wrfly/gus-proxy/utils"
)

// Config ...
type Config struct {
	ProxyHostsFile string
	Scheduler      string
	ListenPort     string
	UA             string
	ProxyHosts     []*types.ProxyHost
}

// Validate the config
func (c *Config) Validate() bool {
	_, err := os.Open(c.ProxyHostsFile)
	if err != nil && os.IsNotExist(err) {
		logrus.Errorf("Hostfile [%s] not exist", c.ProxyHostsFile)
		return false
	}
	switch c.Scheduler {
	case round.ROUND_ROBIN:
	case round.RANDOM:
	case round.PING:
	default:
		return false
	}

	// listen port
	c.ListenPort = fmt.Sprintf(":%s", c.ListenPort)
	l, err := net.Listen("tcp4", c.ListenPort)
	if err != nil {
		logrus.Errorf("Can not bind this port: %s", err)
		return false
	}
	defer l.Close()
	return true
}

// LoadHosts returns the proxy hosts
func (c *Config) LoadHosts() ([]*types.ProxyHost, error) {
	proxyHosts := []*types.ProxyHost{}
	f, _ := os.Open(c.ProxyHostsFile)
	r := bufio.NewReader(f)
	for {
		s, err := r.ReadString('\n')
		if err != nil && s == "" {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// verify hosts
		s = strings.TrimRight(s, "\n")
		_, err = url.Parse(s)
		if err != nil {
			return nil, err
		}

		host := &types.ProxyHost{
			Addr: s,
		}
		proxyHosts = append(proxyHosts, host)
	}

	c.ProxyHosts = proxyHosts
	return proxyHosts, nil
}

// UpdateProxys update proxy's attr
func (c *Config) UpdateProxys() {
	var wg sync.WaitGroup
	availableProxy := struct {
		Num  int
		Lock sync.Mutex
	}{}

	for _, proxy := range c.ProxyHosts {
		wg.Add(1)
		go func(proxy *types.ProxyHost) {
			defer wg.Done()
			proxy.Available = false
			if utils.CheckProxyAvailable(proxy) == nil {
				availableProxy.Lock.Lock()
				availableProxy.Num++
				availableProxy.Lock.Unlock()
				proxy.Available = true
			}
			proxy.Ping = utils.GetProxyPing(proxy)
			logrus.Debugf("Proxy: %s, Available: %v, Ping: %f",
				proxy.Addr, proxy.Available, proxy.Ping)
		}(proxy)
	}
	wg.Wait()

	availableNum := availableProxy.Num
	totalNum := len(c.ProxyHosts)
	switch { // mast in this order (small to big)
	case availableNum*4 <= totalNum:
		logrus.Errorf("Not enough available proxys, available: [%d] total: [%d], I'm angry!",
			availableNum, totalNum)
		// some alert
	case availableNum*2 <= totalNum:
		logrus.Warnf("Half of the proxys was down, available: [%d] total: [%d], I'm worried...",
			availableNum, totalNum)
	}
}

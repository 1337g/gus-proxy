package config

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"github.com/wrfly/gus-proxy/types"
	"github.com/wrfly/gus-proxy/utils"
)

// Config ...
type Config struct {
	Debug               bool
	ProxyFilePath       string
	NoProxyCIDR         []*net.IPNet
	Scheduler           string
	ListenPort          string
	DebugPort           string
	RandomUA            bool
	ProxyUpdateInterval int
	DBFilePath          string

	proxyHostsHash      string
	proxyAliveHash      string
	proxyFilePathIsURL  bool
	proxyHosts          types.ProxyHosts
	oldHosts            []string
	availableProxyHosts types.ProxyHosts

	m sync.RWMutex
}

// Validate the config
func (c *Config) Validate() error {
	logrus.Debugf("get proxyfile [%s]", c.ProxyFilePath)
	_, err := os.Open(c.ProxyFilePath)
	if err != nil && os.IsNotExist(err) {
		resp, err := http.DefaultClient.Get(c.ProxyFilePath)
		if err != nil {
			return fmt.Errorf("get host %s error: %s", c.ProxyFilePath, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Hostfile [%s] not found", c.ProxyFilePath)
		}
		c.proxyFilePathIsURL = true
	}

	switch c.Scheduler {
	case types.ROUND_ROBIN:
	case types.RANDOM:
	case types.PING:
	default:
		return fmt.Errorf("Unknown scheduler: %s", c.Scheduler)
	}

	// listen port
	l, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%s", c.ListenPort))
	if err != nil {
		return fmt.Errorf("Bind port error: %s", err)
	}
	l.Close()

	logrus.Debug("validate ok")
	return nil
}

// LoadHosts returns the proxy hosts
func (c *Config) loadHosts() error {
	logrus.Debug("load proxy hosts")
	var (
		proxyfile  io.ReadCloser
		proxyHosts types.ProxyHosts
		newHosts   []string
		err        error
	)

	if c.proxyFilePathIsURL {
		resp, err := http.DefaultClient.Get(c.ProxyFilePath)
		if err != nil {
			return fmt.Errorf("get host %s error: %s", c.ProxyFilePath, err)
		}
		proxyfile = resp.Body
	} else {
		proxyfile, err = os.Open(c.ProxyFilePath)
		if err != nil {
			return err
		}
	}
	defer proxyfile.Close()
	lines := bufio.NewReader(proxyfile)
	var lnum int
	for {
		lnum++
		s, err := lines.ReadString('\n')
		if err != nil && s == "" {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read line error: %s", err)
		}

		if s[0] == '#' {
			continue
		}

		// verify hosts
		s = strings.TrimRight(s, "\n")
		logrus.Debugf("validate proxy format: %s", s)
		proxyline, err := url.Parse(s)
		if err != nil {
			logrus.Error(err)
			continue
		}
		if !proxyline.IsAbs() {
			logrus.Errorf("Proxy has a empty scheme: %s,file: %s,line %d",
				proxyline, c.ProxyFilePath, lnum)
			continue
		}

		newHosts = append(newHosts, s)
	}
	if c.proxyHostsHash == utils.HashSlice(newHosts) {
		return nil
	}
	c.proxyHostsHash = utils.HashSlice(newHosts)

	c.m.RLock()
	oldHostsMap := make(map[string]bool, len(newHosts))
	for _, host := range c.oldHosts {
		oldHostsMap[host] = true
	}
	var newProxyWG sync.WaitGroup
	for i, host := range newHosts {
		if oldHostsMap[host] {
			if p := c.proxyHosts.Host(i); p != nil {
				proxyHosts.Add(p)
			}
		} else {
			newProxyWG.Add(1)
			go func(host string) {
				defer newProxyWG.Done()

				proxyhost := &types.ProxyHost{
					Addr: host,
				}
				if err := proxyhost.Init(); err != nil {
					logrus.Errorf("init proxy [%s] error: %s", host, err)
					return
				}
				proxyHosts.Add(proxyhost)
			}(host)
		}
	}
	newProxyWG.Wait()

	c.m.RUnlock()

	c.m.Lock()
	c.proxyHosts = proxyHosts
	c.m.Unlock()

	return nil
}

// UpdateProxies update proxy's attr
func (c *Config) UpdateProxies() {
	err := c.loadHosts()
	if err != nil {
		logrus.Errorf("load proxy error: %s", err)
		return
	}

	var (
		wg             sync.WaitGroup
		availableProxy int32
	)
	for _, proxy := range c.proxyHosts.Hosts() {
		wg.Add(1)
		go func(proxy *types.ProxyHost) {
			defer wg.Done()
			if err := proxy.CheckAvaliable(); err != nil {
				proxy.Available = false
			} else {
				atomic.AddInt32(&availableProxy, 1)
			}
			logrus.Debugf("proxy: %s, Available: %t",
				proxy.Addr, proxy.Available)
		}(proxy)
	}
	wg.Wait()

	totalNum := c.proxyHosts.Len()
	// mast in this order (small to big)
	switch {
	case availableProxy*4 <= totalNum:
		logrus.Errorf("Not enough available proxies, available: [%d] total: [%d]",
			availableProxy, totalNum)
	case availableProxy*2 <= totalNum:
		logrus.Warnf("Half of the proxies was down, available: [%d] total: [%d]",
			availableProxy, totalNum)
	}

	oldHosts := make([]string, 0, c.proxyHosts.Len())
	for _, host := range c.proxyHosts.Hosts() {
		if host.Available {
			oldHosts = append(oldHosts, host.Addr)
		}
	}
	if c.proxyAliveHash == utils.HashSlice(oldHosts) {
		logrus.Debugf("alive proxy not changed, continue updating")
		return
	}
	c.proxyAliveHash = utils.HashSlice(oldHosts)

	c.m.Lock()
	c.availableProxyHosts = types.ProxyHosts{}
	for _, ph := range c.proxyHosts.Hosts() {
		if ph.Available {
			c.availableProxyHosts.Add(ph)
		}
	}
	logrus.Infof("update %d available proxies", c.availableProxyHosts.Len())
	c.m.Unlock()

}

// ProxyHosts returns all the proxy hosts get from URL or a static file
func (c *Config) ProxyHosts() []*types.ProxyHost {
	return c.availableProxyHosts.Hosts()
}

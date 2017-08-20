package round

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/wrfly/gus-proxy/prox"
	"github.com/wrfly/gus-proxy/types"
)

func TestRoundProxy(t *testing.T) {
	log.SetOutput(os.Stdout)

	ava := true
	hosts := []*types.ProxyHost{
		&types.ProxyHost{
			Addr:      "http://61.130.97.212:8099",
			Available: ava,
		},
		&types.ProxyHost{
			Addr:      "socks5://127.0.0.1:1080",
			Available: ava,
		},
	}

	proxys, err := prox.New(hosts)
	assert.NoError(t, err)
	assert.NotNil(t, proxys)

	logrus.Info(proxys)

	l, err := net.Listen("tcp4", "127.0.0.1:8080")
	assert.NoError(t, err)
	go http.Serve(l, New(proxys))

	time.Sleep(10 * time.Second)
}

func TestCurlIPWithProxy(t *testing.T) {
	log.SetOutput(os.Stdout)

	ava := false
	localProxy := "127.0.0.1:8080"
	hosts := []*types.ProxyHost{
		&types.ProxyHost{
			Addr:      "http://61.130.97.212:8099",
			Available: ava,
		},
		&types.ProxyHost{
			Addr:      "socks5://127.0.0.1:1080",
			Available: ava,
		},
	}

	proxys, err := prox.New(hosts)
	assert.NoError(t, err)
	assert.NotNil(t, proxys)

	l, err := net.Listen("tcp4", localProxy)
	assert.NoError(t, err)
	go http.Serve(l, New(proxys))

	proxyURL, _ := url.Parse("http://localhost:8080")
	clnt := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 3 * time.Second,
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := clnt.Get("http://ip.chinaz.com/getip.aspx")
			assert.NoError(t, err)
			if resp == nil {
				return
			}
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				logrus.Error(err)
			}
			fmt.Printf("%s\n", body)
			resp.Body.Close()
		}()
	}
	wg.Wait()
}

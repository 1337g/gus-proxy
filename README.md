# Gus-Proxy

"gus - 绝命毒师里的大毒枭"

[![Build Status](https://travis-ci.org/wrfly/gus-proxy.svg?branch=master)](https://travis-ci.org/wrfly/gus-proxy)

---

## Thoughts

> 打一枪换一个地方

1. 每次请求都从代理池中选取一个代理
1. 但是这样会不会触发server端的验证，即session与IP匹配
1. 但是如果server端有这种IP验证的话，就没必要用这东西了
1. 要解决的是server限制某一IP访问频率的问题

没问题。

嗯……如果后端全是HTTP Proxy或者Socks5 Proxy，即代理类型统一的话，其实可以用Nginx的TCP reverse proxy，这样一想，我这个东西就有点鸡肋了,除了替换UA，dig域名这两个功能。
（就当写着玩吧233

## Design

1. 程序对上层表现为一个HTTP代理
1. 程序加载一个代理列表（HTTP/Socks5） [或者默认配置一个代理列表]
1. 每次的请求都从代理列表中选取一个
1. 选取的算法可能是轮询、随机、或其他目前没想到的
1. 要验证proxy的可用性
1. 每次请求替换UA
1. 请求资源的时候，查询目标资源地址全部的IP，随机

## Show off

![Gus-Running](img/gus-run.png)
![Curl-test](img/gus-curl.png)

## Run

```bash
sudo docker run --rm -ti -p 8080:8080 wrfly/gus-proxy
```
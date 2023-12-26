package client

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	QSize    int
	Interval time.Duration
	Count    int

	q chan ReqChan
}

type ReqChan struct {
	req *http.Request
	ch  chan RespErr
}

type RespErr struct {
	Resp *http.Response
	Err  error
}

type RateLimitedRequester struct {
	rl     RateLimiter
	client http.Client

	mx sync.RWMutex
	c  int
}

func NewRateLimitedRequester(qSize int, interval time.Duration, count int) *RateLimitedRequester {
	return &RateLimitedRequester{
		rl: RateLimiter{
			QSize:    qSize,
			Interval: interval,
			q:        make(chan ReqChan),
			Count:    count,
		},
		client: http.Client{},
	}
}

func (rlr RateLimitedRequester) Start() {
	for {
		go func() {
			req := <-rlr.rl.q

			rlr.mx.RLock()
			rlr.c++
			rlr.mx.RUnlock()

			resp, err := rlr.client.Do(req.req)
			respErr := RespErr{
				Resp: resp,
				Err:  err,
			}
			req.ch <- respErr
		}()

		if rlr.c == rlr.rl.Count {
			time.Sleep(rlr.rl.Interval)
		}
	}
}

func (rlr *RateLimitedRequester) DoReq(req http.Request) {
	rlr.mx.RLock()
	defer rlr.mx.RUnlock()

	rlr.c++
	rlr.client.Do(&req)
}

func (rlr RateLimitedRequester) Send(r *http.Request) (*http.Response, error) {
	if rlr.rl.QSize == len(rlr.rl.q) {
		return nil, fmt.Errorf("queue is full")
	}

	respCh := make(chan RespErr)
	rlr.rl.q <- ReqChan{
		req: r,
		ch:  respCh,
	}

	respErr := <-respCh
	close(respCh)

	if respErr.Err != nil {
		return nil, respErr.Err
	}

	return respErr.Resp, nil
}

func main() {
	requester := NewRateLimitedRequester(5, 1*time.Minute, 2)

	requester.Start()

	for {
		go func() {
			r, err := http.NewRequest("GET", "http://localhost:8081", nil)
			if err != nil {
				log.Println(err)
			}

			response, err := requester.Send(r)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(response)
		}()
		time.Sleep(time.Second)
	}
}

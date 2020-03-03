package http3

import "net/http"

type roundRobinRequestScheduler struct {
	// TODO: 实现该类型
}

func newRoundRobinRequestScheduler(info *clientInfo) *roundRobinRequestScheduler {
	// TODO: 实现该方法
	return &roundRobinRequestScheduler{}
}

func (scheduler *roundRobinRequestScheduler) addAndWait(req *http.Request) (*http.Response, error) {
	// TODO: 实现该方法
	return nil, nil
}

func (scheuler *roundRobinRequestScheduler) close() error {
	// TODO: 实现该方法
	return nil
}

func (scheduler *roundRobinRequestScheduler) run() {
	// TODO: 实现该方法
}

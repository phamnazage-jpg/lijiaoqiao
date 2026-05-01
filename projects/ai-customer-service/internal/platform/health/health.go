package health

import "sync/atomic"

type Probe struct {
	live  atomic.Bool
	ready atomic.Bool
}

func NewProbe() *Probe {
	p := &Probe{}
	p.live.Store(true)
	p.ready.Store(false)
	return p
}

func (p *Probe) IsLive() bool {
	return p.live.Load()
}

func (p *Probe) IsReady() bool {
	return p.ready.Load()
}

func (p *Probe) SetLive(live bool) {
	p.live.Store(live)
}

func (p *Probe) SetReady(ready bool) {
	p.ready.Store(ready)
}

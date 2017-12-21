package p2p

import (
	"sync"

	"code.aliyun.com/chain33/chain33/queue"
	"code.aliyun.com/chain33/chain33/types"
)

type NodeInfo struct {
	mtx              sync.Mutex
	pubKey           []byte      `json:"pub_key"`
	network          string      `json:"network"`
	externalAddr     *NetAddress `json:"remote_addr"`
	listenAddr       *NetAddress `json:"listen_addr"`
	version          string      `json:"version"`
	monitorChan      chan *peer
	versionChan      chan struct{}
	p2pBroadcastChan chan interface{}
	cfg              *types.P2P
	q                *queue.Queue
	qclient          queue.IClient
	other            []string `json:"other"` // other application specific data
}

func (nf *NodeInfo) Set(n *NodeInfo) {
	nf.mtx.Lock()
	defer nf.mtx.Unlock()
	nf = n
}

func (nf *NodeInfo) Get() *NodeInfo {
	nf.mtx.Lock()
	defer nf.mtx.Unlock()
	return nf
}
func (nf *NodeInfo) SetExternalAddr(addr *NetAddress) {
	nf.mtx.Lock()
	defer nf.mtx.Unlock()
	nf.externalAddr = addr
}

func (nf *NodeInfo) GetExternalAddr() *NetAddress {
	nf.mtx.Lock()
	defer nf.mtx.Unlock()
	return nf.externalAddr
}

func (nf *NodeInfo) SetListenAddr(addr *NetAddress) {
	nf.mtx.Lock()
	defer nf.mtx.Unlock()
	nf.listenAddr = addr
}

func (nf *NodeInfo) GetListenAddr() *NetAddress {
	nf.mtx.Lock()
	defer nf.mtx.Unlock()
	return nf.listenAddr
}

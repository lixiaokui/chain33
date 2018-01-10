package ticket

//database opeartion for execs ticket
import (
	"fmt"

	"code.aliyun.com/chain33/chain33/account"
	"code.aliyun.com/chain33/chain33/common"
	dbm "code.aliyun.com/chain33/chain33/common/db"
	"code.aliyun.com/chain33/chain33/types"
	log "github.com/inconshreveable/log15"
)

var tlog = log.New("module", "ticket.db")
var genesisKey = []byte("mavl-acc-genesis")
var addrSeed = []byte("address seed bytes for public key")

type Ticket struct {
	types.Ticket
}

func NewTicket(id, minerAddress, returnWallet string, blocktime int64, isGenesis bool) *Ticket {
	t := &Ticket{}
	t.TicketId = id
	t.MinerAddress = minerAddress
	t.ReturnAddress = returnWallet
	t.CreateTime = blocktime
	t.Status = 1
	t.IsGenesis = isGenesis
	return t
}

func (t *Ticket) GetReceiptLog() *types.ReceiptLog {
	log := &types.ReceiptLog{}
	if t.Status == 1 {
		log.Ty = types.TyLogNewTicket
		r := &types.ReceiptTicket{}
		r.TicketId = t.TicketId
		log.Log = types.Encode(r)
	} else if t.Status == 2 {
		log.Ty = types.TyLogMinerTicket
		r := &types.ReceiptTicket{}
		r.TicketId = t.TicketId
		log.Log = types.Encode(r)
	} else if t.Status == 3 {
		log.Ty = types.TyLogCloseTicket
		r := &types.ReceiptTicket{}
		r.TicketId = t.TicketId
		log.Log = types.Encode(r)
	}
	return log
}

func (t *Ticket) GetKVSet() (kvset []*types.KeyValue) {
	value := types.Encode(&t.Ticket)
	if t.Status == 3 {
		value = nil //delete ticket
	}
	kvset = append(kvset, &types.KeyValue{TicketKey(t.TicketId), value})
	return kvset
}

func (t *Ticket) Save(db dbm.KVDB) {
	set := t.GetKVSet()
	for i := 0; i < len(set); i++ {
		db.Set(set[i].GetKey(), set[i].Value)
	}
}

//address to save key
func TicketKey(id string) (key []byte) {
	key = append(key, []byte("mavl-ticket-")...)
	key = append(key, []byte(id)...)
	return key
}

type TicketAction struct {
	db        dbm.KVDB
	txhash    []byte
	fromaddr  string
	blocktime int64
	height    int64
	execaddr  string
}

func NewTicketAction(db dbm.KVDB, tx *types.Transaction, execaddr string, blocktime, height int64) *TicketAction {
	hash := tx.Hash()
	fromaddr := account.PubKeyToAddress(tx.GetSignature().GetPubkey()).String()
	return &TicketAction{db, hash, fromaddr, blocktime, height, execaddr}
}

func (action *TicketAction) GenesisInit(genesis *types.TicketGenesis) (*types.Receipt, error) {
	prefix := common.ToHex(action.txhash)
	prefix = genesis.MinerAddress + ":" + prefix + ":"
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	for i := 0; i < int(genesis.Count); i++ {
		id := prefix + fmt.Sprintf("%010d", i)
		t := NewTicket(id, genesis.MinerAddress, genesis.ReturnAddress, action.blocktime, true)

		//冻结子账户资金
		receipt, err := account.ExecFrozen(action.db, genesis.ReturnAddress, action.execaddr, 1000*types.Coin)
		if err != nil {
			tlog.Error("GenesisInit.Frozen", "addr", genesis.ReturnAddress, "execaddr", action.execaddr)
			panic(err)
		}
		t.Save(action.db)
		logs = append(logs, receipt.Logs...)
		kv = append(kv, receipt.KV...)
		logs = append(logs, t.GetReceiptLog())
		kv = append(kv, t.GetKVSet()...)
	}
	receipt := &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func (action *TicketAction) TicketOpen(topen *types.TicketOpen) (*types.Receipt, error) {
	prefix := common.ToHex(action.txhash)
	prefix = topen.MinerAddress + ":" + prefix + ":"
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	for i := 0; i < int(topen.Count); i++ {
		id := prefix + fmt.Sprintf("%010d", i)
		t := NewTicket(id, topen.MinerAddress, action.fromaddr, action.blocktime, false)

		//冻结子账户资金
		receipt, err := account.ExecFrozen(action.db, action.fromaddr, action.execaddr, 1000*types.Coin)
		if err != nil {
			tlog.Error("TicketOpen.Frozen", "addr", action.fromaddr, "execaddr", action.execaddr)
			return nil, err
		}
		t.Save(action.db)
		logs = append(logs, receipt.Logs...)
		kv = append(kv, receipt.KV...)
		logs = append(logs, t.GetReceiptLog())
		kv = append(kv, t.GetKVSet()...)
	}
	receipt := &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func readTicket(db dbm.KVDB, id string) (*types.Ticket, error) {
	data, err := db.Get(TicketKey(id))
	if err != nil {
		return nil, err
	}
	var ticket types.Ticket
	//decode
	err = types.Decode(data, &ticket)
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (action *TicketAction) TicketMiner(miner *types.TicketMiner, index int) (*types.Receipt, error) {
	if index != 0 {
		return nil, types.ErrCoinBaseIndex
	}
	ticket, err := readTicket(action.db, miner.TicketId)
	if err != nil {
		return nil, err
	}
	if ticket.Status != 1 {
		return nil, types.ErrCoinBaseTicketStatus
	}
	if !ticket.IsGenesis {
		if action.blocktime-ticket.GetCreateTime() < 86400*10 {
			return nil, types.ErrTime
		}
	}
	//check from address
	if action.fromaddr != ticket.MinerAddress && action.fromaddr != ticket.ReturnAddress {
		return nil, types.ErrFromAddr
	}
	ticket.Status = 2
	ticket.MinerValue = miner.Reward
	t := &Ticket{*ticket}
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	receipt, err := account.ExecDepositFrozen(action.db, t.ReturnAddress, action.execaddr, 1000*types.Coin)
	if err != nil {
		tlog.Error("TicketOpen.ExecActive", "addr", t.ReturnAddress, "execaddr", action.execaddr)
		return nil, err
	}
	t.Save(action.db)
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)
	logs = append(logs, t.GetReceiptLog())
	kv = append(kv, t.GetKVSet()...)
	return &types.Receipt{types.ExecOk, kv, logs}, nil
}

func (action *TicketAction) TicketClose(tclose *types.TicketClose) (*types.Receipt, error) {
	var tickets []*Ticket
	for i := 0; i < len(tclose.TicketId); i++ {
		ticket, err := readTicket(action.db, tclose.TicketId[i])
		if err != nil {
			return nil, err
		}
		if ticket.Status != 2 {
			return nil, types.ErrNotMinered
		}
		if !ticket.IsGenesis {
			if action.blocktime-ticket.GetMinerTime() < 86400*10 {
				return nil, types.ErrTime
			}
		}
		//check from address
		if action.fromaddr != ticket.MinerAddress && action.fromaddr != ticket.ReturnAddress {
			return nil, types.ErrFromAddr
		}
		ticket.Status = 3
		tickets[i] = &Ticket{*ticket}
	}
	var logs []*types.ReceiptLog
	var kv []*types.KeyValue
	for i := 0; i < len(tickets); i++ {
		t := tickets[i]
		receipt, err := account.ExecActive(action.db, t.ReturnAddress, action.execaddr, 1000*types.Coin)
		if err != nil {
			tlog.Error("TicketOpen.ExecActive", "addr", t.ReturnAddress, "execaddr", action.execaddr)
			return nil, err
		}
		t.Save(action.db)
		logs = append(logs, receipt.Logs...)
		kv = append(kv, receipt.KV...)
		logs = append(logs, t.GetReceiptLog())
		kv = append(kv, t.GetKVSet()...)
	}
	receipt := &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func TicketList(db dbm.KVDB, tlist *types.TicketList) (*types.Receipt, error) {
	return nil, nil
}

func TicketInfos(db dbm.KVDB, tinfos *types.TicketInfos) ([]*types.Ticket, error) {
	var tickets []*types.Ticket
	for i := 0; i < len(tinfos.TicketIds); i++ {
		id := tinfos.TicketIds[i]
		ticket, err := readTicket(db, id)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, nil
}


//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//

package swap

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/swarm/log"
)

//
//
//

//
//
type Profile struct {
BuyAt  *big.Int //
SellAt *big.Int //
PayAt  uint     //
DropAt uint     //
}

//
//
type Strategy struct {
AutoCashInterval     time.Duration //
AutoCashThreshold    *big.Int      //
AutoDepositInterval  time.Duration //
AutoDepositThreshold *big.Int      //
AutoDepositBuffer    *big.Int      //
}

//
//
type Params struct {
	*Profile
	*Strategy
}

//
//
//
type Promise interface{}

//
type Protocol interface {
Pay(int, Promise) //
	Drop()
	String() string
}

//
type OutPayment interface {
	Issue(amount *big.Int) (promise Promise, err error)
	AutoDeposit(interval time.Duration, threshold, buffer *big.Int)
	Stop()
}

//
type InPayment interface {
	Receive(promise Promise) (*big.Int, error)
	AutoCash(cashInterval time.Duration, maxUncashed *big.Int)
	Stop()
}

//
//
type Swap struct {
lock    sync.Mutex //
balance int        //
local   *Params    //
remote  *Profile   //
proto   Protocol   //
	Payment
}

//
type Payment struct {
Out         OutPayment //
In          InPayment  //
	Buys, Sells bool
}

//
func New(local *Params, pm Payment, proto Protocol) (swap *Swap, err error) {

	swap = &Swap{
		local:   local,
		Payment: pm,
		proto:   proto,
	}

	swap.SetParams(local)

	return
}

//
func (swap *Swap) SetRemote(remote *Profile) {
	defer swap.lock.Unlock()
	swap.lock.Lock()

	swap.remote = remote
	if swap.Sells && (remote.BuyAt.Sign() <= 0 || swap.local.SellAt.Sign() <= 0 || remote.BuyAt.Cmp(swap.local.SellAt) < 0) {
		swap.Out.Stop()
		swap.Sells = false
	}
	if swap.Buys && (remote.SellAt.Sign() <= 0 || swap.local.BuyAt.Sign() <= 0 || swap.local.BuyAt.Cmp(swap.remote.SellAt) < 0) {
		swap.In.Stop()
		swap.Buys = false
	}

	log.Debug(fmt.Sprintf("<%v> remote profile set: pay at: %v, drop at: %v, buy at: %v, sell at: %v", swap.proto, remote.PayAt, remote.DropAt, remote.BuyAt, remote.SellAt))

}

//
func (swap *Swap) SetParams(local *Params) {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	swap.local = local
	swap.setParams(local)
}

//
func (swap *Swap) setParams(local *Params) {

	if swap.Sells {
		swap.In.AutoCash(local.AutoCashInterval, local.AutoCashThreshold)
		log.Info(fmt.Sprintf("<%v> set autocash to every %v, max uncashed limit: %v", swap.proto, local.AutoCashInterval, local.AutoCashThreshold))
	} else {
		log.Info(fmt.Sprintf("<%v> autocash off (not selling)", swap.proto))
	}
	if swap.Buys {
		swap.Out.AutoDeposit(local.AutoDepositInterval, local.AutoDepositThreshold, local.AutoDepositBuffer)
		log.Info(fmt.Sprintf("<%v> set autodeposit to every %v, pay at: %v, buffer: %v", swap.proto, local.AutoDepositInterval, local.AutoDepositThreshold, local.AutoDepositBuffer))
	} else {
		log.Info(fmt.Sprintf("<%v> autodeposit off (not buying)", swap.proto))
	}
}

//
//
//
func (swap *Swap) Add(n int) error {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	swap.balance += n
	if !swap.Sells && swap.balance > 0 {
		log.Trace(fmt.Sprintf("<%v> remote peer cannot have debt (balance: %v)", swap.proto, swap.balance))
		swap.proto.Drop()
		return fmt.Errorf("[SWAP] <%v> remote peer cannot have debt (balance: %v)", swap.proto, swap.balance)
	}
	if !swap.Buys && swap.balance < 0 {
		log.Trace(fmt.Sprintf("<%v> we cannot have debt (balance: %v)", swap.proto, swap.balance))
		return fmt.Errorf("[SWAP] <%v> we cannot have debt (balance: %v)", swap.proto, swap.balance)
	}
	if swap.balance >= int(swap.local.DropAt) {
		log.Trace(fmt.Sprintf("<%v> remote peer has too much debt (balance: %v, disconnect threshold: %v)", swap.proto, swap.balance, swap.local.DropAt))
		swap.proto.Drop()
		return fmt.Errorf("[SWAP] <%v> remote peer has too much debt (balance: %v, disconnect threshold: %v)", swap.proto, swap.balance, swap.local.DropAt)
	} else if swap.balance <= -int(swap.remote.PayAt) {
		swap.send()
	}
	return nil
}

//
func (swap *Swap) Balance() int {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	return swap.balance
}

//
//
//
func (swap *Swap) send() {
	if swap.local.BuyAt != nil && swap.balance < 0 {
		amount := big.NewInt(int64(-swap.balance))
		amount.Mul(amount, swap.remote.SellAt)
		promise, err := swap.Out.Issue(amount)
		if err != nil {
			log.Warn(fmt.Sprintf("<%v> cannot issue cheque (amount: %v, channel: %v): %v", swap.proto, amount, swap.Out, err))
		} else {
			log.Warn(fmt.Sprintf("<%v> cheque issued (amount: %v, channel: %v)", swap.proto, amount, swap.Out))
			swap.proto.Pay(-swap.balance, promise)
			swap.balance = 0
		}
	}
}

//
//
func (swap *Swap) Receive(units int, promise Promise) error {
	if units <= 0 {
		return fmt.Errorf("invalid units: %v <= 0", units)
	}

	price := new(big.Int).SetInt64(int64(units))
	price.Mul(price, swap.local.SellAt)

	amount, err := swap.In.Receive(promise)

	if err != nil {
		err = fmt.Errorf("invalid promise: %v", err)
	} else if price.Cmp(amount) != 0 {
//
		return fmt.Errorf("invalid amount: %v = %v * %v (units sent in msg * agreed sale unit price) != %v (signed in cheque)", price, units, swap.local.SellAt, amount)
	}
	if err != nil {
		log.Trace(fmt.Sprintf("<%v> invalid promise (amount: %v, channel: %v): %v", swap.proto, amount, swap.In, err))
		return err
	}

//
	swap.Add(-units)
	log.Trace(fmt.Sprintf("<%v> received promise (amount: %v, channel: %v): %v", swap.proto, amount, swap.In, promise))

	return nil
}

//
//
func (swap *Swap) Stop() {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	if swap.Buys {
		swap.Out.Stop()
	}
	if swap.Sells {
		swap.In.Stop()
	}
}


//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package api

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/ens"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/swarm/services/swap"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	DefaultHTTPListenAddr = "127.0.0.1"
	DefaultHTTPPort       = "8500"
)

//
//
type Config struct {
//
	*storage.FileStoreParams
	*storage.LocalStoreParams
	*network.HiveParams
	Swap *swap.LocalProfile
	Pss  *pss.PssParams
 /*
 
 
 
 
 
 
 
 
 
 
 
 
 
 
 
 
 
 
 





 
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
 

 






 
 
 
 
  
  
 

 
 
 

 
 
 

 
  
 

 
 
 

 



 
  
  
 
 


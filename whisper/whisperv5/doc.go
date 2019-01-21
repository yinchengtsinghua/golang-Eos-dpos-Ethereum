
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

/*











*/

package whisperv5

import (
	"fmt"
	"time"
)

const (
	EnvelopeVersion    = uint64(0)
	ProtocolVersion    = uint64(5)
	ProtocolVersionStr = "5.0"
	ProtocolName       = "shh"

statusCode           = 0 //
messagesCode         = 1 //
p2pCode              = 2 //
p2pRequestCode       = 3 //
	NumberOfMessageCodes = 64

	paddingMask   = byte(3)
	signatureFlag = byte(4)

	TopicLength     = 4
	signatureLength = 65
	aesKeyLength    = 32
	AESNonceLength  = 12
	keyIdSize       = 32

MaxMessageSize        = uint32(10 * 1024 * 1024) //
	DefaultMaxMessageSize = uint32(1024 * 1024)
	DefaultMinimumPoW     = 0.2

padSizeLimit      = 256 //
	messageQueueLimit = 1024

	expirationCycle   = time.Second
	transmissionCycle = 300 * time.Millisecond

DefaultTTL     = 50 //
SynchAllowance = 10 //
)

type unknownVersionError uint64

func (e unknownVersionError) Error() string {
	return fmt.Sprintf("invalid envelope version %d", uint64(e))
}

//
//
//
//
//
//
type MailServer interface {
	Archive(env *Envelope)
	DeliverMail(whisperPeer *Peer, request *Envelope)
}

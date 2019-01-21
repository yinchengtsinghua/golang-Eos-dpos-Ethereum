
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package clique

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

type testerVote struct {
	signer string
	voted  string
	auth   bool
}

//TestAccountPool是一个用于维护当前活动的测试人员帐户的池，
//从下面测试中使用的文本名称映射到实际的以太坊私有
//能够签署事务的密钥。
type testerAccountPool struct {
	accounts map[string]*ecdsa.PrivateKey
}

func newTesterAccountPool() *testerAccountPool {
	return &testerAccountPool{
		accounts: make(map[string]*ecdsa.PrivateKey),
	}
}

func (ap *testerAccountPool) sign(header *types.Header, signer string) {
//确保我们有签名者的持久密钥
	if ap.accounts[signer] == nil {
		ap.accounts[signer], _ = crypto.GenerateKey()
	}
//在头上签名并将签名嵌入额外的数据中
	sig, _ := crypto.Sign(sigHash(header).Bytes(), ap.accounts[signer])
	copy(header.Extra[len(header.Extra)-65:], sig)
}

func (ap *testerAccountPool) address(account string) common.Address {
//确保我们有帐户的持久密钥
	if ap.accounts[account] == nil {
		ap.accounts[account], _ = crypto.GenerateKey()
	}
//解析并返回以太坊地址
	return crypto.PubkeyToAddress(ap.accounts[account].PublicKey)
}

//TestChainReader实现consension.chainReader以访问Genesis
//块。所有其他方法和请求都会恐慌。
type testerChainReader struct {
	db ethdb.Database
}

func (r *testerChainReader) Config() *params.ChainConfig                 { return params.AllCliqueProtocolChanges }
func (r *testerChainReader) CurrentHeader() *types.Header                { panic("not supported") }
func (r *testerChainReader) GetHeader(common.Hash, uint64) *types.Header { panic("not supported") }
func (r *testerChainReader) GetBlock(common.Hash, uint64) *types.Block   { panic("not supported") }
func (r *testerChainReader) GetHeaderByHash(common.Hash) *types.Header   { panic("not supported") }
func (r *testerChainReader) GetHeaderByNumber(number uint64) *types.Header {
	if number == 0 {
		return rawdb.ReadHeader(r.db, rawdb.ReadCanonicalHash(r.db, 0), 0)
	}
	return nil
}

//测试在各种简单和复杂的情况下是否正确评估投票。
func TestVoting(t *testing.T) {
//
	tests := []struct {
		epoch   uint64
		signers []string
		votes   []testerVote
		results []string
	}{
		{
//单签名人，无投票权
			signers: []string{"A"},
			votes:   []testerVote{{signer: "A"}},
			results: []string{"A"},
		}, {
//单个签名人，投票添加两个其他人（只接受第一个，第二个需要2票）
			signers: []string{"A"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: true},
				{signer: "B"},
				{signer: "A", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//两个签名者，投票加三个（只接受前两个，第三个已经需要3票）
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B", voted: "C", auth: true},
				{signer: "A", voted: "D", auth: true},
				{signer: "B", voted: "D", auth: true},
				{signer: "C"},
				{signer: "A", voted: "E", auth: true},
				{signer: "B", voted: "E", auth: true},
			},
			results: []string{"A", "B", "C", "D"},
		}, {
//单个签名者，放弃自己（很奇怪，但明确允许这样做的话就少了一个死角）
			signers: []string{"A"},
			votes: []testerVote{
				{signer: "A", voted: "A", auth: false},
			},
			results: []string{},
		}, {
//
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//两个签名者，实际上需要双方同意放弃其中一个（满足）
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: false},
				{signer: "B", voted: "B", auth: false},
			},
			results: []string{"A"},
		}, {
//三个签名者，其中两个决定放弃第三个
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//四个签名者，两个的共识不足以让任何人放弃
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
			},
			results: []string{"A", "B", "C", "D"},
		}, {
//四个签名者，三个人的共识已经足够让某人离开
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
			},
			results: []string{"A", "B", "C"},
		}, {
//每个签名者对每个目标的授权计数一次
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//允许同时授权多个帐户
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
				{signer: "A", voted: "D", auth: true},
				{signer: "B"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: true},
				{signer: "A"},
				{signer: "B", voted: "C", auth: true},
			},
			results: []string{"A", "B", "C", "D"},
		}, {
//每个目标的每个签名者对取消授权计数一次
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "B", auth: false},
				{signer: "B"},
				{signer: "A", voted: "B", auth: false},
				{signer: "B"},
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//允许同时解除多个帐户的授权
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
				{signer: "A"},
				{signer: "B", voted: "C", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//取消授权签名者的投票将立即被丢弃（取消授权投票）
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "C", voted: "B", auth: false},
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//来自未授权签名者的投票将立即丢弃（授权投票）
			signers: []string{"A", "B", "C"},
			votes: []testerVote{
				{signer: "C", voted: "B", auth: false},
				{signer: "A", voted: "C", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "A", voted: "B", auth: false},
			},
			results: []string{"A", "B"},
		}, {
//不允许级联更改，只有被投票的帐户才可以更改
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
			},
			results: []string{"A", "B", "C"},
		}, {
//达成共识的变化超出范围（通过deauth）触摸执行
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
				{signer: "A"},
				{signer: "C", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		}, {
//达成共识的变化（通过deauth）可能会在第一次接触时失去共识。
			signers: []string{"A", "B", "C", "D"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: false},
				{signer: "B"},
				{signer: "C"},
				{signer: "A", voted: "D", auth: false},
				{signer: "B", voted: "C", auth: false},
				{signer: "C"},
				{signer: "A"},
				{signer: "B", voted: "D", auth: false},
				{signer: "C", voted: "D", auth: false},
				{signer: "A"},
				{signer: "B", voted: "C", auth: true},
			},
			results: []string{"A", "B", "C"},
		}, {
//确保挂起的投票不会在授权状态更改后继续有效。这个
//只有快速添加、删除签名者，然后
//阅读（或相反），而其中一个最初的选民投了。如果A
//过去的投票被保存在系统中的某个位置，这将干扰
//最终签名者结果。
			signers: []string{"A", "B", "C", "D", "E"},
			votes: []testerVote{
{signer: "A", voted: "F", auth: true}, //授权F，需要3票
				{signer: "B", voted: "F", auth: true},
				{signer: "C", voted: "F", auth: true},
{signer: "D", voted: "F", auth: false}, //取消F的授权，需要4票（保持A以前的投票“不变”）。
				{signer: "E", voted: "F", auth: false},
				{signer: "B", voted: "F", auth: false},
				{signer: "C", voted: "F", auth: false},
{signer: "D", voted: "F", auth: true}, //几乎授权F，需要2/3票
				{signer: "E", voted: "F", auth: true},
{signer: "B", voted: "A", auth: false}, //取消授权A，需要3票
				{signer: "C", voted: "A", auth: false},
				{signer: "D", voted: "A", auth: false},
{signer: "B", voted: "F", auth: true}, //完成授权F，需要3/3票
			},
			results: []string{"B", "C", "D", "E", "F"},
		}, {
//epoch转换重置所有投票以允许链检查点
			epoch:   3,
			signers: []string{"A", "B"},
			votes: []testerVote{
				{signer: "A", voted: "C", auth: true},
				{signer: "B"},
{signer: "A"}, //检查点块（不要在这里投票，它是在快照之外验证的）
				{signer: "B", voted: "C", auth: true},
			},
			results: []string{"A", "B"},
		},
	}
//运行场景并测试它们
	for i, tt := range tests {
//创建帐户池并生成初始签名者集
		accounts := newTesterAccountPool()

		signers := make([]common.Address, len(tt.signers))
		for j, signer := range tt.signers {
			signers[j] = accounts.address(signer)
		}
		for j := 0; j < len(signers); j++ {
			for k := j + 1; k < len(signers); k++ {
				if bytes.Compare(signers[j][:], signers[k][:]) > 0 {
					signers[j], signers[k] = signers[k], signers[j]
				}
			}
		}
//使用初始签名者集创建Genesis块
		genesis := &core.Genesis{
			ExtraData: make([]byte, extraVanity+common.AddressLength*len(signers)+extraSeal),
		}
		for j, signer := range signers {
			copy(genesis.ExtraData[extraVanity+j*common.AddressLength:], signer[:])
		}
//创建一个原始的区块链，注入Genesis
		db := ethdb.NewMemDatabase()
		genesis.Commit(db)

//
		headers := make([]*types.Header, len(tt.votes))
		for j, vote := range tt.votes {
			headers[j] = &types.Header{
				Number:   big.NewInt(int64(j) + 1),
				Time:     big.NewInt(int64(j) * 15),
				Coinbase: accounts.address(vote.voted),
				Extra:    make([]byte, extraVanity+extraSeal),
			}
			if j > 0 {
				headers[j].ParentHash = headers[j-1].Hash()
			}
			if vote.auth {
				copy(headers[j].Nonce[:], nonceAuthVote)
			}
			accounts.sign(headers[j], vote.signer)
		}
//把所有的头条都传给小集团，确保理货成功。
		head := headers[len(headers)-1]

		snap, err := New(&params.CliqueConfig{Epoch: tt.epoch}, db).snapshot(&testerChainReader{db: db}, head.Number.Uint64(), head.Hash(), headers)
		if err != nil {
			t.Errorf("test %d: failed to create voting snapshot: %v", i, err)
			continue
		}
//验证签名者的最终列表与预期列表
		signers = make([]common.Address, len(tt.results))
		for j, signer := range tt.results {
			signers[j] = accounts.address(signer)
		}
		for j := 0; j < len(signers); j++ {
			for k := j + 1; k < len(signers); k++ {
				if bytes.Compare(signers[j][:], signers[k][:]) > 0 {
					signers[j], signers[k] = signers[k], signers[j]
				}
			}
		}
		result := snap.signers()
		if len(result) != len(signers) {
			t.Errorf("test %d: signers mismatch: have %x, want %x", i, result, signers)
			continue
		}
		for j := 0; j < len(result); j++ {
			if !bytes.Equal(result[j][:], signers[j][:]) {
				t.Errorf("test %d, signer %d: signer mismatch: have %x, want %x", i, j, result[j], signers[j])
			}
		}
	}
}

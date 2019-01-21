
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

//
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum/cmd/utils"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/storage/mru"
	"gopkg.in/urfave/cli.v1"
)

func NewGenericSigner(ctx *cli.Context) mru.Signer {
	return mru.NewGenericSigner(getPrivKey(ctx))
}

//Swarm资源创建
//
//swarm resource info<manifest address or ens domain>

func resourceCreate(ctx *cli.Context) {
	args := ctx.Args()

	var (
		bzzapi      = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		client      = swarm.NewClient(bzzapi)
		multihash   = ctx.Bool(SwarmResourceMultihashFlag.Name)
		initialData = ctx.String(SwarmResourceDataOnCreateFlag.Name)
		name        = ctx.String(SwarmResourceNameFlag.Name)
	)

	if len(args) < 1 {
		fmt.Println("Incorrect number of arguments")
		cli.ShowCommandHelpAndExit(ctx, "create", 1)
		return
	}
	signer := NewGenericSigner(ctx)
	frequency, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		fmt.Printf("Frequency formatting error: %s\n", err.Error())
		cli.ShowCommandHelpAndExit(ctx, "create", 1)
		return
	}

	metadata := mru.ResourceMetadata{
		Name:      name,
		Frequency: frequency,
		Owner:     signer.Address(),
	}

	var newResourceRequest *mru.Request
	if initialData != "" {
		initialDataBytes, err := hexutil.Decode(initialData)
		if err != nil {
			fmt.Printf("Error parsing data: %s\n", err.Error())
			cli.ShowCommandHelpAndExit(ctx, "create", 1)
			return
		}
		newResourceRequest, err = mru.NewCreateUpdateRequest(&metadata)
		if err != nil {
			utils.Fatalf("Error creating new resource request: %s", err)
		}
		newResourceRequest.SetData(initialDataBytes, multihash)
		if err = newResourceRequest.Sign(signer); err != nil {
			utils.Fatalf("Error signing resource update: %s", err.Error())
		}
	} else {
		newResourceRequest, err = mru.NewCreateRequest(&metadata)
		if err != nil {
			utils.Fatalf("Error creating new resource request: %s", err)
		}
	}

	manifestAddress, err := client.CreateResource(newResourceRequest)
	if err != nil {
		utils.Fatalf("Error creating resource: %s", err.Error())
		return
	}
fmt.Println(manifestAddress) //

}

func resourceUpdate(ctx *cli.Context) {
	args := ctx.Args()

	var (
		bzzapi    = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		client    = swarm.NewClient(bzzapi)
		multihash = ctx.Bool(SwarmResourceMultihashFlag.Name)
	)

	if len(args) < 2 {
		fmt.Println("Incorrect number of arguments")
		cli.ShowCommandHelpAndExit(ctx, "update", 1)
		return
	}
	signer := NewGenericSigner(ctx)
	manifestAddressOrDomain := args[0]
	data, err := hexutil.Decode(args[1])
	if err != nil {
		utils.Fatalf("Error parsing data: %s", err.Error())
		return
	}

//
	updateRequest, err := client.GetResourceMetadata(manifestAddressOrDomain)
	if err != nil {
		utils.Fatalf("Error retrieving resource status: %s", err.Error())
	}

//设置新数据
	updateRequest.SetData(data, multihash)

//
	if err = updateRequest.Sign(signer); err != nil {
		utils.Fatalf("Error signing resource update: %s", err.Error())
	}

//更新后
	err = client.UpdateResource(updateRequest)
	if err != nil {
		utils.Fatalf("Error updating resource: %s", err.Error())
		return
	}
}

func resourceInfo(ctx *cli.Context) {
	var (
		bzzapi = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		client = swarm.NewClient(bzzapi)
	)
	args := ctx.Args()
	if len(args) < 1 {
		fmt.Println("Incorrect number of arguments.")
		cli.ShowCommandHelpAndExit(ctx, "info", 1)
		return
	}
	manifestAddressOrDomain := args[0]
	metadata, err := client.GetResourceMetadata(manifestAddressOrDomain)
	if err != nil {
		utils.Fatalf("Error retrieving resource metadata: %s", err.Error())
		return
	}
	encodedMetadata, err := metadata.MarshalJSON()
	if err != nil {
		utils.Fatalf("Error encoding metadata to JSON for display:%s", err)
	}
	fmt.Println(string(encodedMetadata))
}

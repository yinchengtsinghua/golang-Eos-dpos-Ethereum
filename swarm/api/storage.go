
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

package api

import (
	"context"
	"path"

	"github.com/ethereum/go-ethereum/swarm/storage"
)

type Response struct {
	MimeType string
	Status   int
	Size     int64
//
	Content string
}

//
//
//
type Storage struct {
	api *API
}

func NewStorage(api *API) *Storage {
	return &Storage{api}
}

//
//
//
//
func (s *Storage) Put(ctx context.Context, content string, contentType string, toEncrypt bool) (storage.Address, func(context.Context) error, error) {
	return s.api.Put(ctx, content, contentType, toEncrypt)
}

//
//
//
//
//
//
//
//
func (s *Storage) Get(ctx context.Context, bzzpath string) (*Response, error) {
	uri, err := Parse(path.Join("bzz:/", bzzpath))
	if err != nil {
		return nil, err
	}
	addr, err := s.api.Resolve(ctx, uri.Addr)
	if err != nil {
		return nil, err
	}
	reader, mimeType, status, _, err := s.api.Get(ctx, nil, addr, uri.Path)
	if err != nil {
		return nil, err
	}
	quitC := make(chan bool)
	expsize, err := reader.Size(ctx, quitC)
	if err != nil {
		return nil, err
	}
	body := make([]byte, expsize)
	size, err := reader.Read(body)
	if int64(size) == expsize {
		err = nil
	}
	return &Response{mimeType, status, expsize, string(body[:size])}, err
}

//
//
//
//
func (s *Storage) Modify(ctx context.Context, rootHash, path, contentHash, contentType string) (newRootHash string, err error) {
	uri, err := Parse("bzz:/" + rootHash)
	if err != nil {
		return "", err
	}
	addr, err := s.api.Resolve(ctx, uri.Addr)
	if err != nil {
		return "", err
	}
	addr, err = s.api.Modify(ctx, addr, path, contentHash, contentType)
	if err != nil {
		return "", err
	}
	return addr.Hex(), nil
}


//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package term

import "golang.org/x/sys/unix"

//如果给定的文件描述符是终端，则istty返回true。
func IsTty(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETA)
	return err == nil
}

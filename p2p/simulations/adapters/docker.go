
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

package adapters

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/pkg/reexec"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

var (
	ErrLinuxOnly = errors.New("DockerAdapter can only be used on Linux as it uses the current binary (which must be a Linux binary)")
)

//Dockeradapter是在Docker中运行模拟节点的节点适配器。
//容器。
//
//建立了一个包含当前二进制at/bin/p2p节点的Docker映像。
//执行时运行基础服务（请参见说明
//有关详细信息，请参阅execp2pnode函数）
type DockerAdapter struct {
	ExecAdapter
}

//newdockeradapter构建包含当前
//二进制并返回dockeradapter
func NewDockerAdapter() (*DockerAdapter, error) {
//因为Docker容器在Linux上运行，而这个适配器运行
//当前容器中的二进制文件，必须为Linux编译。
//
//要求这样做是合理的，因为打电话的人可以
//在Docker容器中编译当前二进制文件。
	if runtime.GOOS != "linux" {
		return nil, ErrLinuxOnly
	}

	if err := buildDockerImage(); err != nil {
		return nil, err
	}

	return &DockerAdapter{
		ExecAdapter{
			nodes: make(map[discover.NodeID]*ExecNode),
		},
	}, nil
}

//name返回用于日志记录的适配器的名称
func (d *DockerAdapter) Name() string {
	return "docker-adapter"
}

//newnode使用给定的配置返回一个新的dockernode
func (d *DockerAdapter) NewNode(config *NodeConfig) (Node, error) {
	if len(config.Services) == 0 {
		return nil, errors.New("node must have at least one service")
	}
	for _, service := range config.Services {
		if _, exists := serviceFuncs[service]; !exists {
			return nil, fmt.Errorf("unknown node service %q", service)
		}
	}

//生成配置
	conf := &execNodeConfig{
		Stack: node.DefaultConfig,
		Node:  config,
	}
	conf.Stack.DataDir = "/data"
	conf.Stack.WSHost = "0.0.0.0"
	conf.Stack.WSOrigins = []string{"*"}
	conf.Stack.WSExposeAll = true
	conf.Stack.P2P.EnableMsgEvents = false
	conf.Stack.P2P.NoDiscovery = true
	conf.Stack.P2P.NAT = nil
	conf.Stack.NoUSB = true

//监听给定端口上的所有接口，当我们
//初始化nodeconfig（通常是随机端口）
	conf.Stack.P2P.ListenAddr = fmt.Sprintf(":%d", config.Port)

	node := &DockerNode{
		ExecNode: ExecNode{
			ID:      config.ID,
			Config:  conf,
			adapter: &d.ExecAdapter,
		},
	}
	node.newCmd = node.dockerCommand
	d.ExecAdapter.nodes[node.ID] = &node.ExecNode
	return node, nil
}

//dockernode包装execnode，但exec的是docker中的当前二进制文件
//容器而不是本地
type DockerNode struct {
	ExecNode
}

//docker command返回一个命令，exec是docker中的二进制文件
//容器。
//
//它使用了一个shell，这样我们就可以通过
//使用--env标志将变量转换为容器。
func (n *DockerNode) dockerCommand() *exec.Cmd {
	return exec.Command(
		"sh", "-c",
		fmt.Sprintf(
			`exec docker run --interactive --env _P2P_NODE_CONFIG="${_P2P_NODE_CONFIG}" %s p2p-node %s %s`,
			dockerImage, strings.Join(n.Config.Node.Services, ","), n.ID.String(),
		),
	)
}

//DockerImage是为运行
//仿真节点
const dockerImage = "p2p-node"

//buildDockerImage构建用于运行模拟的Docker映像
//Docker容器中的节点。
//
//它将当前二进制文件添加为“p2p node”，以便运行execp2pnode
//执行时。
func buildDockerImage() error {
//创建用作生成上下文的目录
	dir, err := ioutil.TempDir("", "p2p-docker")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

//将当前二进制文件复制到生成上下文中
	bin, err := os.Open(reexec.Self())
	if err != nil {
		return err
	}
	defer bin.Close()
	dst, err := os.OpenFile(filepath.Join(dir, "self.bin"), os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, bin); err != nil {
		return err
	}

//创建dockerfile
	dockerfile := []byte(`
FROM ubuntu:16.04
RUN mkdir /data
ADD self.bin /bin/p2p-node
	`)
	if err := ioutil.WriteFile(filepath.Join(dir, "Dockerfile"), dockerfile, 0644); err != nil {
		return err
	}

//运行“docker build”
	cmd := exec.Command("docker", "build", "-t", dockerImage, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error building docker image: %s", err)
	}

	return nil
}

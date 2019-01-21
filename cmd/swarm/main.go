
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

package main

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/swarm"
	bzzapi "github.com/ethereum/go-ethereum/swarm/api"
	swarmmetrics "github.com/ethereum/go-ethereum/swarm/metrics"
	"github.com/ethereum/go-ethereum/swarm/tracing"
	sv "github.com/ethereum/go-ethereum/swarm/version"

	"gopkg.in/urfave/cli.v1"
)

const clientIdentifier = "swarm"
const helpTemplate = `NAME:
{{.HelpName}} - {{.Usage}}

USAGE:
{{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Category}}

CATEGORY:
{{.Category}}{{end}}{{if .Description}}

DESCRIPTION:
{{.Description}}{{end}}{{if .VisibleFlags}}

OPTIONS:
{{range .VisibleFlags}}{{.}}
{{end}}{{end}}
`

var (
gitCommit string //git sha1提交发布的哈希（通过链接器标志设置）
)

var (
	ChequebookAddrFlag = cli.StringFlag{
		Name:   "chequebook",
		Usage:  "chequebook contract address",
		EnvVar: SWARM_ENV_CHEQUEBOOK_ADDR,
	}
	SwarmAccountFlag = cli.StringFlag{
		Name:   "bzzaccount",
		Usage:  "Swarm account key file",
		EnvVar: SWARM_ENV_ACCOUNT,
	}
	SwarmListenAddrFlag = cli.StringFlag{
		Name:   "httpaddr",
		Usage:  "Swarm HTTP API listening interface",
		EnvVar: SWARM_ENV_LISTEN_ADDR,
	}
	SwarmPortFlag = cli.StringFlag{
		Name:   "bzzport",
		Usage:  "Swarm local http api port",
		EnvVar: SWARM_ENV_PORT,
	}
	SwarmNetworkIdFlag = cli.IntFlag{
		Name:   "bzznetworkid",
		Usage:  "Network identifier (integer, default 3=swarm testnet)",
		EnvVar: SWARM_ENV_NETWORK_ID,
	}
	SwarmSwapEnabledFlag = cli.BoolFlag{
		Name:   "swap",
		Usage:  "Swarm SWAP enabled (default false)",
		EnvVar: SWARM_ENV_SWAP_ENABLE,
	}
	SwarmSwapAPIFlag = cli.StringFlag{
		Name:   "swap-api",
		Usage:  "URL of the Ethereum API provider to use to settle SWAP payments",
		EnvVar: SWARM_ENV_SWAP_API,
	}
	SwarmSyncDisabledFlag = cli.BoolTFlag{
		Name:   "nosync",
		Usage:  "Disable swarm syncing",
		EnvVar: SWARM_ENV_SYNC_DISABLE,
	}
	SwarmSyncUpdateDelay = cli.DurationFlag{
		Name:   "sync-update-delay",
		Usage:  "Duration for sync subscriptions update after no new peers are added (default 15s)",
		EnvVar: SWARM_ENV_SYNC_UPDATE_DELAY,
	}
	SwarmLightNodeEnabled = cli.BoolFlag{
		Name:   "lightnode",
		Usage:  "Enable Swarm LightNode (default false)",
		EnvVar: SWARM_ENV_LIGHT_NODE_ENABLE,
	}
	SwarmDeliverySkipCheckFlag = cli.BoolFlag{
		Name:   "delivery-skip-check",
		Usage:  "Skip chunk delivery check (default false)",
		EnvVar: SWARM_ENV_DELIVERY_SKIP_CHECK,
	}
	EnsAPIFlag = cli.StringSliceFlag{
		Name:   "ens-api",
		Usage:  "ENS API endpoint for a TLD and with contract address, can be repeated, format [tld:][contract-addr@]url",
		EnvVar: SWARM_ENV_ENS_API,
	}
	SwarmApiFlag = cli.StringFlag{
		Name:  "bzzapi",
		Usage: "Swarm HTTP endpoint",
Value: "http://
	}
	SwarmRecursiveFlag = cli.BoolFlag{
		Name:  "recursive",
		Usage: "Upload directories recursively",
	}
	SwarmWantManifestFlag = cli.BoolTFlag{
		Name:  "manifest",
		Usage: "Automatic manifest upload (default true)",
	}
	SwarmUploadDefaultPath = cli.StringFlag{
		Name:  "defaultpath",
		Usage: "path to file served for empty url path (none)",
	}
	SwarmAccessGrantKeyFlag = cli.StringFlag{
		Name:  "grant-key",
		Usage: "grants a given public key access to an ACT",
	}
	SwarmAccessGrantKeysFlag = cli.StringFlag{
		Name:  "grant-keys",
		Usage: "grants a given list of public keys in the following file (separated by line breaks) access to an ACT",
	}
	SwarmUpFromStdinFlag = cli.BoolFlag{
		Name:  "stdin",
		Usage: "reads data to be uploaded from stdin",
	}
	SwarmUploadMimeType = cli.StringFlag{
		Name:  "mime",
		Usage: "Manually specify MIME type",
	}
	SwarmEncryptedFlag = cli.BoolFlag{
		Name:  "encrypt",
		Usage: "use encrypted upload",
	}
	SwarmAccessPasswordFlag = cli.StringFlag{
		Name:   "password",
		Usage:  "Password",
		EnvVar: SWARM_ACCESS_PASSWORD,
	}
	SwarmDryRunFlag = cli.BoolFlag{
		Name:  "dry-run",
		Usage: "dry-run",
	}
	CorsStringFlag = cli.StringFlag{
		Name:   "corsdomain",
		Usage:  "Domain on which to send Access-Control-Allow-Origin header (multiple domains can be supplied separated by a ',')",
		EnvVar: SWARM_ENV_CORS,
	}
	SwarmStorePath = cli.StringFlag{
		Name:   "store.path",
		Usage:  "Path to leveldb chunk DB (default <$GETH_ENV_DIR>/swarm/bzz-<$BZZ_KEY>/chunks)",
		EnvVar: SWARM_ENV_STORE_PATH,
	}
	SwarmStoreCapacity = cli.Uint64Flag{
		Name:   "store.size",
		Usage:  "Number of chunks (5M is roughly 20-25GB) (default 5000000)",
		EnvVar: SWARM_ENV_STORE_CAPACITY,
	}
	SwarmStoreCacheCapacity = cli.UintFlag{
		Name:   "store.cache.size",
		Usage:  "Number of recent chunks cached in memory (default 5000)",
		EnvVar: SWARM_ENV_STORE_CACHE_CAPACITY,
	}
	SwarmResourceMultihashFlag = cli.BoolFlag{
		Name:  "multihash",
		Usage: "Determines how to interpret data for a resource update. If not present, data will be interpreted as raw, literal data that will be included in the resource",
	}
	SwarmResourceNameFlag = cli.StringFlag{
		Name:  "name",
		Usage: "User-defined name for the new resource",
	}
	SwarmResourceDataOnCreateFlag = cli.StringFlag{
		Name:  "data",
		Usage: "Initializes the resource with the given hex-encoded data. Data must be prefixed by 0x",
	}
)

//声明几个常量错误消息，对以后测试中的错误检查比较很有用
var (
	SWARM_ERR_NO_BZZACCOUNT   = "bzzaccount option is required but not set; check your config file, command line or environment variables"
	SWARM_ERR_SWAP_SET_NO_API = "SWAP is enabled but --swap-api is not set"
)

//
var defaultSubcommandHelp = cli.Command{
	Action:             func(ctx *cli.Context) { cli.ShowCommandHelpAndExit(ctx, "", 1) },
	CustomHelpTemplate: helpTemplate,
	Name:               "help",
	Usage:              "shows this help",
	Hidden:             true,
}

var defaultNodeConfig = node.DefaultConfig

//
func init() {
	defaultNodeConfig.Name = clientIdentifier
	defaultNodeConfig.Version = sv.VersionWithCommit(gitCommit)
	defaultNodeConfig.P2P.ListenAddr = ":30399"
	defaultNodeConfig.IPCPath = "bzzd.ipc"
//
	utils.ListenPortFlag.Value = 30399
}

var app = utils.NewApp(gitCommit, "Ethereum Swarm")

//这个init函数创建cli.app。
func init() {
	app.Action = bzzd
app.HideVersion = true //我们有打印版本的命令
	app.Copyright = "Copyright 2013-2016 The go-ethereum Authors"
	app.Commands = []cli.Command{
		{
			Action:             version,
			CustomHelpTemplate: helpTemplate,
			Name:               "version",
			Usage:              "Print version numbers",
			Description:        "The output of this command is supposed to be machine-readable",
		},
		{
			Action:             upload,
			CustomHelpTemplate: helpTemplate,
			Name:               "up",
			Usage:              "uploads a file or directory to swarm using the HTTP API",
			ArgsUsage:          "<file>",
			Flags:              []cli.Flag{SwarmEncryptedFlag},
			Description:        "uploads a file or directory to swarm using the HTTP API and prints the root hash",
		},
		{
			CustomHelpTemplate: helpTemplate,
			Name:               "access",
			Usage:              "encrypts a reference and embeds it into a root manifest",
			ArgsUsage:          "<ref>",
			Description:        "encrypts a reference and embeds it into a root manifest",
			Subcommands: []cli.Command{
				{
					CustomHelpTemplate: helpTemplate,
					Name:               "new",
					Usage:              "encrypts a reference and embeds it into a root manifest",
					ArgsUsage:          "<ref>",
					Description:        "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
					Subcommands: []cli.Command{
						{
							Action:             accessNewPass,
							CustomHelpTemplate: helpTemplate,
							Flags: []cli.Flag{
								utils.PasswordFileFlag,
								SwarmDryRunFlag,
							},
							Name:        "pass",
							Usage:       "encrypts a reference with a password and embeds it into a root manifest",
							ArgsUsage:   "<ref>",
							Description: "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
						},
						{
							Action:             accessNewPK,
							CustomHelpTemplate: helpTemplate,
							Flags: []cli.Flag{
								utils.PasswordFileFlag,
								SwarmDryRunFlag,
								SwarmAccessGrantKeyFlag,
							},
							Name:        "pk",
							Usage:       "encrypts a reference with the node's private key and a given grantee's public key and embeds it into a root manifest",
							ArgsUsage:   "<ref>",
							Description: "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
						},
						{
							Action:             accessNewACT,
							CustomHelpTemplate: helpTemplate,
							Flags: []cli.Flag{
								SwarmAccessGrantKeysFlag,
								SwarmDryRunFlag,
							},
							Name:        "act",
							Usage:       "encrypts a reference with the node's private key and a given grantee's public key and embeds it into a root manifest",
							ArgsUsage:   "<ref>",
							Description: "encrypts a reference and embeds it into a root access manifest and prints the resulting manifest",
						},
					},
				},
			},
		},
		{
			CustomHelpTemplate: helpTemplate,
			Name:               "resource",
			Usage:              "(Advanced) Create and update Mutable Resources",
			ArgsUsage:          "<create|update|info>",
			Description:        "Works with Mutable Resource Updates",
			Subcommands: []cli.Command{
				{
					Action:             resourceCreate,
					CustomHelpTemplate: helpTemplate,
					Name:               "create",
					Usage:              "creates a new Mutable Resource",
					ArgsUsage:          "<frequency>",
					Description:        "creates a new Mutable Resource",
					Flags:              []cli.Flag{SwarmResourceNameFlag, SwarmResourceDataOnCreateFlag, SwarmResourceMultihashFlag},
				},
				{
					Action:             resourceUpdate,
					CustomHelpTemplate: helpTemplate,
					Name:               "update",
					Usage:              "updates the content of an existing Mutable Resource",
					ArgsUsage:          "<Manifest Address or ENS domain> <0x Hex data>",
					Description:        "updates the content of an existing Mutable Resource",
					Flags:              []cli.Flag{SwarmResourceMultihashFlag},
				},
				{
					Action:             resourceInfo,
					CustomHelpTemplate: helpTemplate,
					Name:               "info",
					Usage:              "obtains information about an existing Mutable Resource",
					ArgsUsage:          "<Manifest Address or ENS domain>",
					Description:        "obtains information about an existing Mutable Resource",
				},
			},
		},
		{
			Action:             list,
			CustomHelpTemplate: helpTemplate,
			Name:               "ls",
			Usage:              "list files and directories contained in a manifest",
			ArgsUsage:          "<manifest> [<prefix>]",
			Description:        "Lists files and directories contained in a manifest",
		},
		{
			Action:             hash,
			CustomHelpTemplate: helpTemplate,
			Name:               "hash",
			Usage:              "print the swarm hash of a file or directory",
			ArgsUsage:          "<file>",
			Description:        "Prints the swarm hash of file or directory",
		},
		{
			Action:      download,
			Name:        "down",
			Flags:       []cli.Flag{SwarmRecursiveFlag, SwarmAccessPasswordFlag},
			Usage:       "downloads a swarm manifest or a file inside a manifest",
			ArgsUsage:   " <uri> [<dir>]",
			Description: `Downloads a swarm bzz uri to the given dir. When no dir is provided, working directory is assumed. --recursive flag is expected when downloading a manifest with multiple entries.`,
		},
		{
			Name:               "manifest",
			CustomHelpTemplate: helpTemplate,
			Usage:              "perform operations on swarm manifests",
			ArgsUsage:          "COMMAND",
			Description:        "Updates a MANIFEST by adding/removing/updating the hash of a path.\nCOMMAND could be: add, update, remove",
			Subcommands: []cli.Command{
				{
					Action:             manifestAdd,
					CustomHelpTemplate: helpTemplate,
					Name:               "add",
					Usage:              "add a new path to the manifest",
					ArgsUsage:          "<MANIFEST> <path> <hash>",
					Description:        "Adds a new path to the manifest",
				},
				{
					Action:             manifestUpdate,
					CustomHelpTemplate: helpTemplate,
					Name:               "update",
					Usage:              "update the hash for an already existing path in the manifest",
					ArgsUsage:          "<MANIFEST> <path> <newhash>",
					Description:        "Update the hash for an already existing path in the manifest",
				},
				{
					Action:             manifestRemove,
					CustomHelpTemplate: helpTemplate,
					Name:               "remove",
					Usage:              "removes a path from the manifest",
					ArgsUsage:          "<MANIFEST> <path>",
					Description:        "Removes a path from the manifest",
				},
			},
		},
		{
			Name:               "fs",
			CustomHelpTemplate: helpTemplate,
			Usage:              "perform FUSE operations",
			ArgsUsage:          "fs COMMAND",
			Description:        "Performs FUSE operations by mounting/unmounting/listing mount points. This assumes you already have a Swarm node running locally. For all operation you must reference the correct path to bzzd.ipc in order to communicate with the node",
			Subcommands: []cli.Command{
				{
					Action:             mount,
					CustomHelpTemplate: helpTemplate,
					Name:               "mount",
					Flags:              []cli.Flag{utils.IPCPathFlag},
					Usage:              "mount a swarm hash to a mount point",
					ArgsUsage:          "swarm fs mount --ipcpath <path to bzzd.ipc> <manifest hash> <mount point>",
					Description:        "Mounts a Swarm manifest hash to a given mount point. This assumes you already have a Swarm node running locally. You must reference the correct path to your bzzd.ipc file",
				},
				{
					Action:             unmount,
					CustomHelpTemplate: helpTemplate,
					Name:               "unmount",
					Flags:              []cli.Flag{utils.IPCPathFlag},
					Usage:              "unmount a swarmfs mount",
					ArgsUsage:          "swarm fs unmount --ipcpath <path to bzzd.ipc> <mount point>",
					Description:        "Unmounts a swarmfs mount residing at <mount point>. This assumes you already have a Swarm node running locally. You must reference the correct path to your bzzd.ipc file",
				},
				{
					Action:             listMounts,
					CustomHelpTemplate: helpTemplate,
					Name:               "list",
					Flags:              []cli.Flag{utils.IPCPathFlag},
					Usage:              "list swarmfs mounts",
					ArgsUsage:          "swarm fs list --ipcpath <path to bzzd.ipc>",
					Description:        "Lists all mounted swarmfs volumes. This assumes you already have a Swarm node running locally. You must reference the correct path to your bzzd.ipc file",
				},
			},
		},
		{
			Name:               "db",
			CustomHelpTemplate: helpTemplate,
			Usage:              "manage the local chunk database",
			ArgsUsage:          "db COMMAND",
			Description:        "Manage the local chunk database",
			Subcommands: []cli.Command{
				{
					Action:             dbExport,
					CustomHelpTemplate: helpTemplate,
					Name:               "export",
					Usage:              "export a local chunk database as a tar archive (use - to send to stdout)",
					ArgsUsage:          "<chunkdb> <file>",
					Description: `
Export a local chunk database as a tar archive (use - to send to stdout).

    swarm db export ~/.ethereum/swarm/bzz-KEY/chunks chunks.tar

The export may be quite large, consider piping the output through the Unix
pv(1) tool to get a progress bar:

    swarm db export ~/.ethereum/swarm/bzz-KEY/chunks - | pv > chunks.tar
`,
				},
				{
					Action:             dbImport,
					CustomHelpTemplate: helpTemplate,
					Name:               "import",
					Usage:              "import chunks from a tar archive into a local chunk database (use - to read from stdin)",
					ArgsUsage:          "<chunkdb> <file>",
					Description: `Import chunks from a tar archive into a local chunk database (use - to read from stdin).

    swarm db import ~/.ethereum/swarm/bzz-KEY/chunks chunks.tar

The import may be quite large, consider piping the input through the Unix
pv(1) tool to get a progress bar:

    pv chunks.tar | swarm db import ~/.ethereum/swarm/bzz-KEY/chunks -`,
				},
				{
					Action:             dbClean,
					CustomHelpTemplate: helpTemplate,
					Name:               "clean",
					Usage:              "remove corrupt entries from a local chunk database",
					ArgsUsage:          "<chunkdb>",
					Description:        "Remove corrupt entries from a local chunk database",
				},
			},
		},

//参见CONG.GO
		DumpConfigCommand,
	}

//
//
	addDefaultHelpSubcommands(app.Commands)

	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = []cli.Flag{
		utils.IdentityFlag,
		utils.DataDirFlag,
		utils.BootnodesFlag,
		utils.KeyStoreDirFlag,
		utils.ListenPortFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.MaxPeersFlag,
		utils.NATFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.PasswordFileFlag,
//
		CorsStringFlag,
		EnsAPIFlag,
		SwarmTomlConfigPathFlag,
		SwarmSwapEnabledFlag,
		SwarmSwapAPIFlag,
		SwarmSyncDisabledFlag,
		SwarmSyncUpdateDelay,
		SwarmLightNodeEnabled,
		SwarmDeliverySkipCheckFlag,
		SwarmListenAddrFlag,
		SwarmPortFlag,
		SwarmAccountFlag,
		SwarmNetworkIdFlag,
		ChequebookAddrFlag,
//上传标志
		SwarmApiFlag,
		SwarmRecursiveFlag,
		SwarmWantManifestFlag,
		SwarmUploadDefaultPath,
		SwarmUpFromStdinFlag,
		SwarmUploadMimeType,
//存储标志
		SwarmStorePath,
		SwarmStoreCapacity,
		SwarmStoreCacheCapacity,
	}
	rpcFlags := []cli.Flag{
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
	}
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, debug.Flags...)
	app.Flags = append(app.Flags, swarmmetrics.Flags...)
	app.Flags = append(app.Flags, tracing.Flags...)
	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		if err := debug.Setup(ctx, ""); err != nil {
			return err
		}
		swarmmetrics.Setup(ctx)
		tracing.Setup(ctx)
		return nil
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func version(ctx *cli.Context) error {
	fmt.Println(strings.Title(clientIdentifier))
	fmt.Println("Version:", sv.VersionWithMeta)
	if gitCommit != "" {
		fmt.Println("Git Commit:", gitCommit)
	}
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("OS:", runtime.GOOS)
	return nil
}

func bzzd(ctx *cli.Context) error {
//
//

	bzzconfig, err := buildConfig(ctx)
	if err != nil {
		utils.Fatalf("unable to configure swarm: %v", err)
	}

	cfg := defaultNodeConfig

//
	cfg.WSModules = append(cfg.WSModules, "pss")

//
//为了在Swarm中保持一致，如果我们通过环境变量传递--datadir
//
	if _, err := os.Stat(bzzconfig.Path); err == nil {
		cfg.DataDir = bzzconfig.Path
	}

//
	setSwarmBootstrapNodes(ctx, &cfg)
//
	utils.SetNodeConfig(ctx, &cfg)
	stack, err := node.New(&cfg)
	if err != nil {
		utils.Fatalf("can't create node: %v", err)
	}

//
//
	initSwarmNode(bzzconfig, stack, ctx)
//在以太坊节点中将bzz注册为node.service
	registerBzzService(bzzconfig, stack)
//
	utils.StartNode(stack)

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got sigterm, shutting swarm down...")
		stack.Stop()
	}()

	stack.Wait()
	return nil
}

func registerBzzService(bzzconfig *bzzapi.Config, stack *node.Node) {
//定义Swarm服务引导功能
	boot := func(_ *node.ServiceContext) (node.Service, error) {
//
		return swarm.NewSwarm(bzzconfig, nil)
	}
//
	if err := stack.Register(boot); err != nil {
		utils.Fatalf("Failed to register the Swarm service: %v", err)
	}
}

func getAccount(bzzaccount string, ctx *cli.Context, stack *node.Node) *ecdsa.PrivateKey {
//
	if bzzaccount == "" {
		utils.Fatalf(SWARM_ERR_NO_BZZACCOUNT)
	}
//尝试将arg作为十六进制密钥文件加载。
	if key, err := crypto.LoadECDSA(bzzaccount); err == nil {
		log.Info("Swarm account key loaded", "address", crypto.PubkeyToAddress(key.PublicKey))
		return key
	}
//
	am := stack.AccountManager()
	ks := am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	return decryptStoreAccount(ks, bzzaccount, utils.MakePasswordList(ctx))
}

//getprivkey返回指定bzzaccount的私钥
//
func getPrivKey(ctx *cli.Context) *ecdsa.PrivateKey {
//像在bzzd操作中一样启动群节点
	bzzconfig, err := buildConfig(ctx)
	if err != nil {
		utils.Fatalf("unable to configure swarm: %v", err)
	}
	cfg := defaultNodeConfig
	if _, err := os.Stat(bzzconfig.Path); err == nil {
		cfg.DataDir = bzzconfig.Path
	}
	utils.SetNodeConfig(ctx, &cfg)
	stack, err := node.New(&cfg)
	if err != nil {
		utils.Fatalf("can't create node: %v", err)
	}
	return getAccount(bzzconfig.BzzAccount, ctx, stack)
}

func decryptStoreAccount(ks *keystore.KeyStore, account string, passwords []string) *ecdsa.PrivateKey {
	var a accounts.Account
	var err error
	if common.IsHexAddress(account) {
		a, err = ks.Find(accounts.Account{Address: common.HexToAddress(account)})
	} else if ix, ixerr := strconv.Atoi(account); ixerr == nil && ix > 0 {
		if accounts := ks.Accounts(); len(accounts) > ix {
			a = accounts[ix]
		} else {
			err = fmt.Errorf("index %d higher than number of accounts %d", ix, len(accounts))
		}
	} else {
		utils.Fatalf("Can't find swarm account key %s", account)
	}
	if err != nil {
		utils.Fatalf("Can't find swarm account key: %v - Is the provided bzzaccount(%s) from the right datadir/Path?", err, account)
	}
	keyjson, err := ioutil.ReadFile(a.URL.Path)
	if err != nil {
		utils.Fatalf("Can't load swarm account key: %v", err)
	}
	for i := 0; i < 3; i++ {
		password := getPassPhrase(fmt.Sprintf("Unlocking swarm account %s [%d/3]", a.Address.Hex(), i+1), i, passwords)
		key, err := keystore.DecryptKey(keyjson, password)
		if err == nil {
			return key.PrivateKey
		}
	}
	utils.Fatalf("Can't decrypt swarm account key")
	return nil
}

//getpassphrase通过获取与bzz帐户关联的密码
//
func getPassPhrase(prompt string, i int, passwords []string) string {
//非交互式
	if len(passwords) > 0 {
		if i < len(passwords) {
			return passwords[i]
		}
		return passwords[len(passwords)-1]
	}

//回退到交互模式
	if prompt != "" {
		fmt.Println(prompt)
	}
	password, err := console.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	return password
}

//adddefaulthelpsubcommand通过定义的cli命令扫描并添加
//
//
func addDefaultHelpSubcommands(commands []cli.Command) {
	for i := range commands {
		cmd := &commands[i]
		if cmd.Subcommands != nil {
			cmd.Subcommands = append(cmd.Subcommands, defaultSubcommandHelp)
			addDefaultHelpSubcommands(cmd.Subcommands)
		}
	}
}

func setSwarmBootstrapNodes(ctx *cli.Context, cfg *node.Config) {
	if ctx.GlobalIsSet(utils.BootnodesFlag.Name) || ctx.GlobalIsSet(utils.BootnodesV4Flag.Name) {
		return
	}

	cfg.P2P.BootstrapNodes = []*discover.Node{}

	for _, url := range SwarmBootnodes {
		node, err := discover.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
		}
		cfg.P2P.BootstrapNodes = append(cfg.P2P.BootstrapNodes, node)
	}
	log.Debug("added default swarm bootnodes", "length", len(cfg.P2P.BootstrapNodes))
}

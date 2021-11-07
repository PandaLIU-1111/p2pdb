package main

// This is a CLI that lets you join a global permissionless CRDT-based
// database using CRDTs and IPFS.

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	crdt "github.com/ipfs/go-ds-crdt"
	logging "github.com/ipfs/go-log/v2"

	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/mitchellh/go-homedir"

	multiaddr "github.com/multiformats/go-multiaddr"
)

var (
	logger    = logging.Logger("p2pdb")
	listen, _ = multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/33123")
	topicName = "p2pdb-example"
	netTopic  = "p2pdb-example-net"
	config    = "p2pdb-example"
)

func main() {
	// Bootstrappers are using 1024 keys. See:
	// 启动节点哟过 1024 keys
	// https://github.com/ipfs/infra/issues/378
	crypto.MinRsaKeyBits = 1024

	//设置日志级别
	logging.SetLogLevel("*", "error")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//获取用户的主目录
	dir, err := homedir.Dir()
	if err != nil {
		logger.Fatal(err)
	}
	//config=globaldb-example
	data := filepath.Join(dir, config)

	store, err := ipfslite.BadgerDatastore(data)
	if err != nil {
		logger.Fatal(err)
	}
	defer store.Close()

	// filepath=home/user/globaldb-example/key
	keyPath := filepath.Join(data, "key")
	var priv crypto.PrivKey
	_, err = os.Stat(keyPath)
	if os.IsNotExist(err) {
		priv, _, err = crypto.GenerateKeyPair(crypto.Ed25519, 1)
		if err != nil {
			logger.Fatal(err)
		}
		data, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			logger.Fatal(err)
		}
		err = ioutil.WriteFile(keyPath, data, 0400)
		if err != nil {
			logger.Fatal(err)
		}
	} else if err != nil {
		logger.Fatal(err)
	} else {
		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			logger.Fatal(err)
		}
		priv, err = crypto.UnmarshalPrivateKey(key)
		if err != nil {
			logger.Fatal(err)
		}

	}
	pid, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		logger.Fatal(err)
	}

	h, dht, err := ipfslite.SetupLibp2p(
		ctx,
		priv,
		nil,
		[]multiaddr.Multiaddr{listen},
		nil,
		ipfslite.Libp2pOptionsExtra...,
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer h.Close()
	defer dht.Close()

	psub, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		logger.Fatal(err)
	}

	topic, err := psub.Join(netTopic)
	if err != nil {
		logger.Fatal(err)
	}

	//根据topic 进行订阅
	netSubs, err := topic.Subscribe()
	if err != nil {
		logger.Fatal(err)
	}

	// Use a special pubsub topic to avoid disconnecting
	// from globaldb peers.
	// Host 是一个参与 p2p 网络的对象，它实现协议或提供服务。它处理像服务器一样请求，像客户端一样发出请求。
	// 之所以称为 Host，是因为它既是 Server 又是 Client（还有 Peer
	// 可能会引起混淆）。
	//死循环监听订阅
	go func() {
		for {
			msg, err := netSubs.Next(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			//ConnManager 返回这个host连接管理器
			h.ConnManager().TagPeer(msg.ReceivedFrom, "keep", 100)
		}
	}()

	//发布消息

	//select语句和switch语句一样，它不是循环，它只会选择一个case来处理，如果想一直处理channel，你可以在外面加一个无限的for循环：
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				//打印发布消息

				//fmt.Println("触发广播====")
				//广播发布消息
				topic.Publish(ctx, []byte("hi!"))
				time.Sleep(20 * time.Second)
			}
		}
	}()

	ipfs, err := ipfslite.New(ctx, store, h, dht, nil)
	if err != nil {
		logger.Fatal(err)
	}
	//广播
	pubsubBC, err := crdt.NewPubSubBroadcaster(ctx, psub, topicName)
	if err != nil {
		logger.Fatal(err)
	}

	//crdt 广播配置
	opts := crdt.DefaultOptions()
	opts.Logger = logger
	opts.RebroadcastInterval = 5 * time.Second
	//put 时输出值
	opts.PutHook = func(k ds.Key, v []byte) {
		fmt.Printf("Added: [%s] -> %s\n", k, string(v))

	}
	// 删除值
	opts.DeleteHook = func(k ds.Key) {
		fmt.Printf("Removed: [%s]\n", k)
	}

	//使用crdt 进行广播
	crdt, err := crdt.New(store, ds.NewKey("crdt"), ipfs, pubsubBC, opts)
	if err != nil {
		logger.Fatal(err)
	}
	defer crdt.Close()

	fmt.Println("Bootstrapping...")
	//开启本地广播，此处应该调整为配置文件
	//bstr, _ := multiaddr.NewMultiaddr("/ip4/94.130.135.167/tcp/33123/ipfs/12D3KooWFta2AE7oiK1ioqjVAKajUJauZWfeM7R413K7ARtHRDAu")
	bstr, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/33123/ipfs/12D3KooWMVdnQXeh97noZrUavoULs7GA2qQYhHTFRueDAmyprRaH")
	inf, _ := peer.AddrInfoFromP2pAddr(bstr)
	list := append(ipfslite.DefaultBootstrapPeers(), *inf)
	ipfs.Bootstrap(list)
	h.ConnManager().TagPeer(inf.ID, "keep", 100)

	fmt.Printf(`
Peer ID: %s
Listen address: %s
Topic: %s
Data Folder: %s

Ready!

Commands:

> list               -> list items in the store
> get <key>          -> get value for a key
> put <key> <value>  -> store value on a key
> exit               -> quit


`,
		pid, listen, topicName, data,
	)

	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		fmt.Println("Running in daemon mode")
		go func() {
			for {
				fmt.Printf("%s - %d connected peers\n", time.Now().Format(time.Stamp), len(connectedPeers(h)))
				time.Sleep(10 * time.Second)
			}
		}()
		signalChan := make(chan os.Signal, 20)
		signal.Notify(
			signalChan,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGHUP,
		)
		<-signalChan
		return
	}

	fmt.Printf("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		fields := strings.Fields(text)
		if len(fields) == 0 {
			fmt.Printf("> ")
			continue
		}

		cmd := fields[0]

		switch cmd {
		case "exit", "quit":
			return
		case "debug":
			if len(fields) < 2 {
				fmt.Println("debug <on/off/peers>")
			}
			st := fields[1]
			switch st {
			case "on":
				logging.SetLogLevel("globaldb", "debug")
			case "off":
				logging.SetLogLevel("globaldb", "error")
			case "peers": //查看对等节点
				for _, p := range connectedPeers(h) {
					addrs, err := peer.AddrInfoToP2pAddrs(p)
					if err != nil {
						logger.Warn(err)
						continue
					}
					for _, a := range addrs {
						fmt.Println(a)
					}
				}
			}
		case "list":
			q := query.Query{}
			results, err := crdt.Query(q)
			if err != nil {
				printErr(err)
			}
			for r := range results.Next() {
				if r.Error != nil {
					printErr(err)
					continue
				}
				fmt.Printf("[%s] -> %s\n", r.Key, string(r.Value))
			}
		case "get":
			if len(fields) < 2 {
				fmt.Println("get <key>")
				fmt.Println("> ")
				continue
			}
			k := ds.NewKey(fields[1])
			v, err := crdt.Get(k)
			if err != nil {
				printErr(err)
				continue
			}
			fmt.Printf("[%s] -> %s\n", k, string(v))
		case "put":
			if len(fields) < 3 {
				fmt.Println("put <key> <value>")
				fmt.Println("> ")
				continue
			}
			k := ds.NewKey(fields[1])
			v := strings.Join(fields[2:], " ")
			err := crdt.Put(k, []byte(v))
			if err != nil {
				printErr(err)
				continue
			}
		}
		fmt.Printf("> ")
	}
}

func printErr(err error) {
	fmt.Println("error:", err)
	fmt.Println("> ")
}

//对等节点连接，返回对等节点信息
func connectedPeers(h host.Host) []*peer.AddrInfo {
	var pinfos []*peer.AddrInfo
	for _, c := range h.Network().Conns() {
		pinfos = append(pinfos, &peer.AddrInfo{
			ID:    c.RemotePeer(),
			Addrs: []multiaddr.Multiaddr{c.RemoteMultiaddr()},
		})
	}
	return pinfos
}

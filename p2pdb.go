package main

import (
	"context"
	"fmt"
	"io/ioutil"

	idp "berty.tech/go-ipfs-log/identityprovider"

	"berty.tech/go-ipfs-log/keystore"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	config "github.com/ipfs/go-ipfs-config"
	ipfs_core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	ipfs_libp2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	ipfs_repo "github.com/ipfs/go-ipfs/repo"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"

	log "berty.tech/go-ipfs-log"
)

func buildHostOverrideExample(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
	return ipfs_libp2p.DefaultHostOption(ctx, id, ps, options...)
}

func newRepo() (ipfs_repo.Repo, error) {
	// Generating config
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return nil, err
	}

	// Listen on local interface only
	cfg.Addresses.Swarm = []string{
		"/ip4/127.0.0.1/tcp/0",
	}

	// Do not bootstrap on ipfs node
	cfg.Bootstrap = []string{}

	return &ipfs_repo.Mock{
		D: dssync.MutexWrap(datastore.NewMapDatastore()),
		C: *cfg,
	}, nil
}

func newRepo2() (ipfs_repo.Repo, error) {
	// Generating config
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return nil, err
	}

	// Listen on local interface only
	cfg.Addresses.Swarm = []string{
		"/ip4/127.0.0.1/tcp/1",
	}

	// Do not bootstrap on ipfs node
	cfg.Bootstrap = []string{}

	return &ipfs_repo.Mock{
		D: dssync.MutexWrap(datastore.NewMapDatastore()),
		C: *cfg,
	}, nil
}

func buildNode(ctx context.Context) *ipfs_core.IpfsNode {
	r, err := newRepo()
	if err != nil {
		panic(err)
	}

	cfg := &ipfs_core.BuildCfg{
		Online: true,
		Repo:   r,
		Host:   buildHostOverrideExample,
	}

	nodeA, err := ipfs_core.NewNode(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return nodeA
}

func main() {
	fmt.Println("test log merge")
	//初始化上下文
	ctx := context.Background()
	// Build Ipfs Node A
	node := buildNode(ctx)
	// Fill up datastore with identities
	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	ks, err := keystore.NewKeystore(ds)
	if err != nil {
		panic(err)
	}
	// Create identity A
	identityA, err := idp.CreateIdentity(&idp.CreateIdentityOptions{
		Keystore: ks,
		ID:       "userA",
		Type:     "orbitdb",
	})
	if err != nil {
		panic(fmt.Errorf("coreapi error: %s", err))
	}

	identityB, err := idp.CreateIdentity(&idp.CreateIdentityOptions{
		Keystore: ks,
		ID:       "userB",
		Type:     "orbitdb",
	})
	if err != nil {
		panic(fmt.Errorf("coreapi error: %s", err))
	}

	identityC, err := idp.CreateIdentity(&idp.CreateIdentityOptions{
		Keystore: ks,
		ID:       "userB",
		Type:     "orbitdb",
	})
	if err != nil {
		panic(fmt.Errorf("coreapi error: %s", err))
	}

	service, err := coreapi.NewCoreAPI(node)
	if err != nil {
		panic(fmt.Errorf("coreapi error: %s", err))
	}

	// creating log
	log1, err := log.NewLog(service, identityA, &log.LogOptions{ID: "A"})
	if err != nil {
		panic(err)
	}

	// creating log
	log2, err := log.NewLog(service, identityB, &log.LogOptions{ID: "A"})
	if err != nil {
		panic(err)
	}

	// creating log
	log3, err := log.NewLog(service, identityC, &log.LogOptions{ID: "A"})
	if err != nil {
		panic(err)
	}

	_, err = log1.Append(ctx, []byte("one"), nil)
	if err != nil {
		panic(fmt.Errorf("append error: %s", err))
	}

	_, err = log1.Append(ctx, []byte("two"), nil)
	if err != nil {
		panic(fmt.Errorf("append error: %s", err))
	}

	_, err = log2.Append(ctx, []byte("three"), nil)
	if err != nil {
		panic(fmt.Errorf("append error: %s", err))
	}
	// Join the logs
	log3.Join(log1, 1)
	log3.Join(log2, 2)

	_, err = log3.Append(ctx, []byte("four"), nil)
	if err != nil {
		panic(fmt.Errorf("append error: %s", err))
	}

	fmt.Println(log3.Values())

	h, err := log3.ToMultihash(ctx)
	if err != nil {
		panic(fmt.Errorf("ToMultihash error: %s", err))
	}

	res, err := log.NewFromMultihash(ctx, service, identityC, h, &log.LogOptions{}, &log.FetchOptions{})
	if err != nil {
		panic(fmt.Errorf("NewFromMultihash error: %s", err))
	}
	fmt.Println("echo result====")
	// nodeB lookup logA
	fmt.Println(res.ToString(nil))

}

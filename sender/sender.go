package main

import (
	"bufio"
	"context"
	"sync"
	"encoding/binary"
	"P2P_libs/flagsnew"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	multiaddr "github.com/multiformats/go-multiaddr"
	logging "github.com/whyrusleeping/go-logging"

	"github.com/ipfs/go-log"
)

var logger = log.Logger("rendezvous")


func main() {
	log.SetAllLoggers(logging.WARNING)
	log.SetLogLevel("rendezvous", "info")
	config, err := flagsnew.ParseFlags()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// libp2p.New constructs a new libp2p Host. Other options can be added
	// here.
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrs([]multiaddr.Multiaddr(config.ListenAddresses)...),
	)
	if err != nil {
		panic(err)
	}
	logger.Info("Host created. We are:", host.ID())
	logger.Info(host.Addrs())

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		panic(err)
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	logger.Info("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var wg sync.WaitGroup
	for _, peerAddr := range config.BootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
				logger.Warning(err)
			} else {
				logger.Info("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	//logger.Info("Announcing ourselves...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	//discovery.Advertise(ctx, routingDiscovery, config.RendezvousString)
	//logger.Info("Successfully announced!")
	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	logger.Info("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, config.RendezvousString)
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID(){
			continue
		}
		logger.Info("Found peer:", peer)

		logger.Info("Connecting to:", peer)
		stream, err := host.NewStream(ctx, peer.ID, protocol.ID(config.ProtocolID))
		logger.Info("Connected to:", peer)
		
		if err != nil {
			logger.Warning("Connection failed:", err)
			continue
		} else {

			var x uint32 = 1

			w := bufio.NewWriter(stream)
			err := binary.Write(w, binary.LittleEndian, x)
			if err != nil {
				logger.Warning("binary.Write failed:", err)
			}
			w.Flush()
		}

		
	}

	select {}
}

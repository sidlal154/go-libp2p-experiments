package main

import (
	"bufio"
	"context"
	"io"
	"sync"
	"fmt"
	"os"
	"time"
	"encoding/binary"
	"bytes"
	"P2P_libs/flagsnew"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	multiaddr "github.com/multiformats/go-multiaddr"
	logging "github.com/whyrusleeping/go-logging"

	"github.com/ipfs/go-log"
)

var logger = log.Logger("rendezvous")

func handleStream(stream network.Stream) {
	logger.Info("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	r := bufio.NewReader(stream);

	buf := make([]byte, 4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		logger.Warning(err)
	}
	var x uint32
	w := bytes.NewReader(buf)
	err = binary.Read(w, binary.LittleEndian, &x)
	if err != nil {
		fmt.Println("binary.Read failed:", err)
	}
	
	fmt.Println(x)
	
	file, err := os.Create("test.txt") 
      
    if err != nil { 
        logger.Info("failed creating file: %s", err) 
    }

	//logger.Info("Outside loop")
	defer file.Close() 
	
	time.Sleep(3*time.Second)

	file.WriteString(string(x))
	file.Sync()

	// 'stream' will stay open until you close it (or the other side closes it).
}

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

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	host.SetStreamHandler(protocol.ID(config.ProtocolID), handleStream)

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
	logger.Info("Announcing ourselves...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	discovery.Advertise(ctx, routingDiscovery, config.RendezvousString)
	logger.Info("Successfully announced!")
	select {}
}

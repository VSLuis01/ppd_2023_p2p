package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
)

func readData(rw *bufio.ReadWriter) {

	for {
		str, _ := rw.ReadString('\n')

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func handleStream(s network.Stream) {
	log.Println("Um novo super nó foi conectado")

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	go readData(rw)
	//go writeData(rw)
}

func startPeer(ctx context.Context, h host.Host, streamHandler network.StreamHandler) (pontoDeConexao string) {
	// Set a function as stream handler.
	// This function is called when a peer connects, and starts a stream with this protocol.
	// Only applies on the receiving side.
	h.SetStreamHandler("/handshake/master", streamHandler)

	// Let's get the actual TCP port from our listen multiaddr, in case we're using 0 (default; random available port).
	var port string
	for _, la := range h.Network().ListenAddresses() {
		if p, err := la.ValueForProtocol(multiaddr.P_TCP); err == nil {
			port = p
			break
		}
	}

	if port == "" {
		log.Println("was not able to find actual local port")
		return
	}

	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%v/p2p/%s' on another console.\n", port, h.ID())
}

func makeHost(port int, rand io.Reader) (h host.Host, err error) {
	// cria uma chave única privada RSA

	privateKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand)
	if err != nil {
		log.Println("Erro ao criar chave privada: ", err)
		return nil, err
	}

	// criando um multiaddr para ouvir em qualquer ip
	sourcerMulti, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))

	if err != nil {
		log.Println("Falha ao criar endereço multiaddr: ", err)
		return nil, err
	}

	return libp2p.New(
		libp2p.ListenAddrs(sourcerMulti),
		libp2p.Identity(privateKey),
	)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	// definindo a porta do nó mestre
	masterPort := flag.Int("p", 0, "Porta destino")
	flag.Parse()

	// criando um host que irá escutar em qualquer interface "0.0.0.0" e na porta "masterPort"
	h, err := makeHost(*masterPort, rand.Reader)
	if err != nil {
		log.Println("Erro ao criar host: ", err)
		return
	}

	// criando um novo pubsub para os supernós se conectarem ao nó mestre (conexao exclusiva entre eles)
	psSuperMaster, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Println("Erro ao criar pubsub: ", err)
		return
	}

	// criando um peer
	pontoDeConexao := startPeer(ctx, h, handleStream)

	// criando um tópico para os supernós se conectarem ao nó mestre
	topic, err := psSuperMaster.Join("access-key")
	if err != nil {
		log.Println("Erro ao criar tópico: ", err)
		return
	}

	go func() {
		err = topic.Publish(ctx, []byte(pontoDeConexao))
		if err != nil {
			log.Println("Erro ao publicar tópico de acesso aos super nós: ", err)
			panic(err)
		}
	}()

	select {}

}

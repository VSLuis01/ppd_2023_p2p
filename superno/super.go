package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
	"net"
)

var tabelasDeRoteamento []peer.AddrInfo

func subscribeToTopic(ctx context.Context, pubsub *pubsub.PubSub, topic string) (*pubsub.Subscription, error) {
	// Subscribe to the topic
	mytopc, err := pubsub.Join(topic)

	if err != nil {
		return nil, err
	}
	sub, err := mytopc.Subscribe()
	if err != nil {
		return nil, err
	}
	// Start a goroutine to handle incoming messages
	go func() {
		for {
			msg, err := sub.Next(ctx)

			if err != nil {
				log.Println("Error reading message:", err)
				continue
			}

			if topic == "roteamento" {
				err := json.Unmarshal(msg.Data, &tabelasDeRoteamento)
				if err != nil {
					log.Println("Erro ao deserializar roteamento: ", err)
				}
				log.Println("Recebido tabela de roteamento: ", tabelasDeRoteamento)
			} else {
				log.Printf("Received message: %s\n", string(msg.Data))
			}
		}
	}()

	return sub, nil
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

func startPeerAndConnect(ctx context.Context, h host.Host, destination string) (*bufio.ReadWriter, error) {
	/*log.Println("This node's multiaddresses:")
	for _, la := range h.Addrs() {
		log.Printf(" - %v\n", la)
	}*/
	log.Println()

	// Turn the destination into a multiaddr.
	maddr, err := multiaddr.NewMultiaddr(destination)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Extract the peer ID from the multiaddr.
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Add the destination's peer multiaddress in the peerstore.
	// This will be used during connection and stream creation by libp2p.
	h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

	// Start a stream with the destination.
	// Multiaddress of the destination peer is fetched from the peerstore using 'peerId'.
	s, err := h.NewStream(context.Background(), info.ID, "/handshake/master")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("Conexão estabelecida com o nó mestre")

	// Create a buffered stream so that read and writes are non-blocking.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	return rw, nil
}

// parte que vai se comunicar com o nó mestre
func clientSide(h host.Host, ctx context.Context) {
	buffer := make([]byte, 1024)

	pb, err := pubsub.NewGossipSub(context.Background(), h)
	if err != nil {
		log.Println("Erro ao criar pubsub: ", err)
		return
	}

	subscribeToTopic(context.Background(), pb, "broadcast")
	subscribeToTopic(context.Background(), pb, "roteamento")

	fmt.Println("Aguardando conexão de um peer...")
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
		return
	}
	fmt.Println("Conexão estabelecida com sucesso")
	defer conn.Close()

	fmt.Println("Lendo chave de identificação...")
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Erro ao ler a chave de identificação:", err)
		return
	}

	chaveConexao := string(buffer[:n])

	fmt.Println("Chave de acesso recebida: ", chaveConexao)

	fmt.Println("Estabelecendo conexão com o nó mestre...")
	_, err = startPeerAndConnect(ctx, h, chaveConexao)
	if err != nil {
		log.Println("Erro ao conectar ao peer: ", err)
		return
	}

	ack := "ACK"
	fmt.Println("Enviando ACK para o nó mestre...")
	_, err = conn.Write([]byte(ack))
	if err != nil {
		fmt.Println("Erro ao enviar ACK:", err)
		return
	}

	// Create a thread to read and write data.
	//go writeData(rw)
	//go readData(rw)

	select {}
}

// parte que vai se comunicar com os nós servidores
func serverSide(h host.Host, ctx context.Context) {

	select {}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	// definindo a porta do supernó
	superPort := flag.Int("p", 0, "Porta destino")
	flag.Parse()

	hostClientSide, err := makeHost(0, rand.Reader)
	if err != nil {
		log.Println("Erro ao criar host client side: ", err)
		return
	}
	hostServerSide, err := makeHost(*superPort, rand.Reader)
	if err != nil {
		log.Println("Erro ao criar host server side: ", err)
		return
	}

	go clientSide(hostClientSide, ctx)
	go serverSide(hostServerSide, ctx)

	// Wait forever
	select {}
}

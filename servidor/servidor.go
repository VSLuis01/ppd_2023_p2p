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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
	"net"
)

var tabelasDeRoteamento []peer.AddrInfo

func errorHandler(err error, msg string, fatal bool) {
	if fatal {
		if err != nil {
			log.Fatal(msg, err)
		}
	} else {
		if err != nil {
			log.Println(msg, err)
		}
	}
}

func printPeerstore(h host.Host) {
	for _, p := range h.Network().Peers() {
		fmt.Println("Conectado ao peer:", p)
		fmt.Println("\tEndereços do peer:", h.Peerstore().PeerInfo(p))
	}
}

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

func tcpHandleConnection(conn net.Conn, chaveDeConexao string, ackChan chan<- bool, i int) {
	//TODO: implementar tratamento da conexão e conexão p2p
	defer conn.Close()

	_, err := conn.Write([]byte(chaveDeConexao))

	if err != nil {
		fmt.Println("Erro ao enviar a chave de identificação:", err)
		ackChan <- false
		return
	}

	fmt.Println("Chave de identificação enviada para o super nó ", i+1)
	ack := make([]byte, 3)
	_, err = conn.Read(ack)
	if err != nil {
		fmt.Println("Erro ao ler o ACK do cliente: ", i+1, err)
		ackChan <- false
		return
	}

	if string(ack) == "ACK" {
		fmt.Println("ACK recebido do super nó ", i+1, " . Conexão estabelecida.")
		ackChan <- true
		return
	} else {
		fmt.Println("ACK inválido. Ação apropriada aqui (reenviar a chave, encerrar a conexão, etc.).")
		ackChan <- false
	}
}

func listensSubs(ctx context.Context, sub *pubsub.Subscription, topic *pubsub.Topic) {
	// Subscribe to the topic
	// Start a goroutine to handle incoming messages
	for {
		msg, err := sub.Next(ctx)

		if err != nil {
			log.Println("Error reading message:", err)
			continue
		}

		if topic.String() == "roteamento" {
			err := json.Unmarshal(msg.Data, &tabelasDeRoteamento)
			if err != nil {
				log.Println("Erro ao deserializar roteamento: ", err)
			}
			log.Println("Recebido tabela de roteamento: ", tabelasDeRoteamento)
		} else {
			log.Printf("Received message: %s\n", string(msg.Data))
		}
	}
}

func subscribeToTopic(mytopc *pubsub.Topic) (*pubsub.Subscription, error) {
	// Subscribe to the topic
	sub, err := mytopc.Subscribe()
	errorHandler(err, "Erro ao se inscrever no tópico: ", false)

	return sub, err
}

func createAndJoinTopic(pubsub *pubsub.PubSub, topicName string) (*pubsub.Topic, error) {
	return pubsub.Join(topicName)
}

func MakeHost(port int, rand io.Reader) (h host.Host, err error) {
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

func startPeer(h host.Host, streamHandler network.StreamHandler) (pontoDeConexao string) {
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
		log.Println("Erro ao obter a porta TCP")
		return
	}

	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%v/p2p/%s", port, h.ID())
}

func main() {
	ctx := context.Background()

	ip := flag.String("ip", "127.0.0.1:8002", "Ip e porta destino")
	flag.Parse()

	//definir ip e porta do servidor

	h, err := MakeHost(0, rand.Reader)
	errorHandler(err, "Erro ao criar host: ", true)

	pb, err := pubsub.NewGossipSub(context.Background(), h)
	errorHandler(err, "Erro ao criar pubsub: ", true)

	broadcastTopic, err := createAndJoinTopic(pb, "broadcast")
	errorHandler(err, "Erro ao criar tópico broadcast: ", true)
	broadcastSub, err := subscribeToTopic(broadcastTopic)
	errorHandler(err, "Erro ao se inscrever no tópico broadcast: ", true)
	go listensSubs(ctx, broadcastSub, broadcastTopic)

	servidoresTopic, err := createAndJoinTopic(pb, "servidores")
	errorHandler(err, "Erro ao criar tópico servidores: ", true)
	servidoresSub, err := subscribeToTopic(servidoresTopic)
	errorHandler(err, "Erro ao se inscrever no tópico servidores: ", true)
	go listensSubs(ctx, servidoresSub, servidoresTopic)

	fmt.Println("Aguardando conexão de um peer...")

	conn, err := net.Dial("tcp", *ip)
	defer conn.Close()
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")

	chaveDeConexao := startPeer(h, handleStream)

	tcpHandleConnection(conn, chaveDeConexao, nil, 0)
	errorHandler(err, "Erro ao ler a chave de identificação:", true)

	// Create a thread to read and write data.
	//go writeData(rw)
	//go readData(rw)

	// Wait forever
	select {}
}

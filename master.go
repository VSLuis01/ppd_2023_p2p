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
	"time"
)

type TabelaDeRoteamento map[string][]multiaddr.Multiaddr

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

	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%v/p2p/%s", port, h.ID())
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

func broadcastMessage(ctx context.Context, pubsub *pubsub.PubSub, topic string, message []byte) error {
	// Publish the message to the topic
	mytopc, err := pubsub.Join(topic)
	mytopc.Publish(ctx, message)
	if err != nil {
		return err
	}

	// Wait for the message to be propagated to all peers
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	return nil
}

func tcpHandleConnection(conn net.Conn, chaveDeConexao string, ackChan chan<- bool, i int) {
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

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	// definindo a porta do nó mestre
	masterPort := flag.Int("p", 0, "Porta destino")
	flag.Parse()

	// criando um host que irá escutar em qualquer interface "0.0.0.0" e na porta "masterPort"
	fmt.Println("Criando host. Porta:", *masterPort)
	h, err := makeHost(*masterPort, rand.Reader)
	if err != nil {
		log.Println("Erro ao criar host: ", err)
		return
	}

	// criando um novo pubsub para os supernós se conectarem ao nó mestre (conexao exclusiva entre eles)
	psSuperMaster, err := pubsub.NewGossipSub(context.Background(), h)
	if err != nil {
		log.Println("Erro ao criar pubsub: ", err)
		return
	}

	// servidor tcp
	tcpListener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Erro ao criar servidor TCP:", err)
		return
	}
	defer tcpListener.Close()

	chaveDeConexao := startPeer(ctx, h, handleStream)

	// lida com conexoes de outros supernós
	fmt.Println("Aguardando supernós se conectarem...")

	ackChan := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		fmt.Println("Aguardando supernó ", i+1, " se conectar...")
		conn, err := tcpListener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			return
		}
		go tcpHandleConnection(conn, chaveDeConexao, ackChan, i)
	}

	// Aguardar que ambos os nós se conectem
	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os super nós se conectaram com sucesso!")
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	fmt.Println("Enviando mensagem de broadcast")

	broadcastMessage(ctx, psSuperMaster, "broadcast", []byte("concluido"))

	var tabelas []peer.AddrInfo

	fmt.Println()
	// listar todas as informações dos peers conectados
	for _, p := range h.Network().Peers() {
		tabelas = append(tabelas, h.Peerstore().PeerInfo(p))
		fmt.Println("Conectado ao peer:", p)
		fmt.Println("\tEndereços do peer:", h.Peerstore().PeerInfo(p))
	}

	fmt.Println(tabelas)

	byteTeste, err := json.Marshal(tabelas)
	if err != nil {
		fmt.Println("Erro ao converter", err)
		return
	}

	broadcastMessage(ctx, psSuperMaster, "roteamento", byteTeste)

	select {}

}

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

func errorHandler(err error, msg string) {
	if err != nil {
		log.Println(msg, err)
	}
}

func printPeerstore(h host.Host) {
	for _, p := range h.Network().Peers() {
		fmt.Println("Conectado ao peer:", p)
		fmt.Println("\tEndereços do peer:", h.Peerstore().PeerInfo(p))
		fmt.Println()
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

func startPeer(h host.Host, streamHandler network.StreamHandler) (pontoDeConexao string) {
	// Seta uma função como stream handler. Essa função é chamada quando um peer se conecta e inicia uma stream com esse protocolo.
	h.SetStreamHandler("/handshake/master", streamHandler)

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

func createAndJoinTopic(pubsub *pubsub.PubSub, topicName string) (*pubsub.Topic, error) {
	return pubsub.Join(topicName)
}

func broadcastMessage(ctx context.Context, topic *pubsub.Topic, message []byte) {
	// Publish the message to the topic
	err := topic.Publish(ctx, message)
	errorHandler(err, "Erro ao publicar mensagem: ")

	// Wait for the message to be propagated to all peers
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
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
	ctx := context.Background()

	// definindo a porta do nó mestre
	masterPort := flag.Int("p", 0, "Porta destino")
	flag.Parse()

	// criando um host que irá escutar em qualquer interface "0.0.0.0" e na porta "masterPort"
	h, err := makeHost(*masterPort, rand.Reader)
	errorHandler(err, "Erro ao criar host: ")

	// criando um novo pubsub para os supernós se conectarem ao nó mestre (conexao exclusiva entre eles)
	psSuperMaster, err := pubsub.NewGossipSub(context.Background(), h)
	errorHandler(err, "Erro ao criar pubsub: ")

	// servidor tcp
	tcpListener, err := net.Listen("tcp", ":8080")
	errorHandler(err, "Erro ao criar servidor TCP: ")

	defer tcpListener.Close()

	// cria uma chave única privada RSA para os super nós conectarem
	chaveDeConexao := startPeer(h, handleStream)

	// lida com conexoes de outros supernós
	fmt.Println("Aguardando supernós se conectarem...")

	// canais criados para controlar a conexão dos supernós
	ackChan := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão: ")

		go tcpHandleConnection(conn, chaveDeConexao, ackChan, i)
	}

	// Aguardar que ambos os nós se conectem
	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os super nós se conectaram com sucesso!")
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	fmt.Println("Enviando mensagem de broadcast...")

	broadcastTopic, _ := createAndJoinTopic(psSuperMaster, "broadcast")
	broadcastMessage(ctx, broadcastTopic, []byte("concluido"))

	// tabela de roteamento de todos os supernós
	var tabelas []peer.AddrInfo

	fmt.Println()
	// listar todas as informações dos peers conectados
	for _, p := range h.Network().Peers() {
		tabelas = append(tabelas, h.Peerstore().PeerInfo(p))
	}
	printPeerstore(h)

	byteTabela, err := json.Marshal(tabelas)
	errorHandler(err, "Erro ao converter tabela para bytes: ")

	roteamentoTopic, _ := createAndJoinTopic(psSuperMaster, "roteamento")
	broadcastMessage(ctx, roteamentoTopic, byteTabela)

	select {}

}

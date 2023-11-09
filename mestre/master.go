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
	"os"
	"strings"
	"sync"
	"time"
)

type HostAnel struct {
	IDHost string
	IPHost string
}

var tabelasDeRoteamento []HostAnel
var tabelasDeRoteamentoServidores []HostAnel

var mutexTabelasDeServ = sync.Mutex{}
var mutexTabelasDeSupers = sync.Mutex{}
var ipProximoNo string
var ipNoAnterior string
var ipHost string

type mensagem struct {
	IpOrigem  string
	IpDestino string
	Conteudo  string
	IpAtual   string
}

func (m *mensagem) enviarMensagemNext(tipo string) {
	//envia mensagem para o proximo
	sendMessageNext([]byte(tipo + "#" + m.IpOrigem + "#" + m.IpDestino + "#" + m.Conteudo + "#" + ipHost))
}

func (m *mensagem) enviarMensagemAnt(tipo string) {
	//envia mensagem para o proximo
	sendMessageAnt([]byte(tipo + "#" + m.IpOrigem + "#" + m.IpDestino + "#" + m.Conteudo + "#" + ipHost))
}

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
func openFileAndGetIps() []string {
	file, err := os.Open("../ips")
	errorHandler(err, "Erro ao abrir arquivo: ", true)

	defer file.Close()

	scanner := bufio.NewScanner(file)

	var ips []string

	for scanner.Scan() {
		ips = append(ips, strings.TrimSpace(scanner.Text()))
	}

	return ips
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
	errorHandler(err, "Erro ao publicar mensagem: ", true)

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
func receiveMessageAnelListening(adress string) {
	tcpListener, err := net.Listen("tcp", adress)
	defer tcpListener.Close()
	if err != nil {
		errorHandler(err, "Erro ao ler mensagem TCP: ", false)
		return
	}
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			errorHandler(err, "Erro ao ler mensagem TCP: ", false)
			return
		}
		fmt.Println("Conexão TCP estabelecida com sucesso:", conn.RemoteAddr().String())
		go func() { //leitura dos dados recebidos e tratamento deles
			//ipTratamento, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			fmt.Println("Conexão TCP estabelecida com sucesso:", conn.RemoteAddr().String())
			for {
				buffer := make([]byte, 1024)
				n, err := conn.Read(buffer)
				//	decodedMessage, err := base64.StdEncoding.DecodeString(string(buffer[:n]))
				recebimento := string(buffer[:n])
				aux := strings.Split(recebimento, "#")
				if len(aux) < 5 { //evita mensagens com formato invalido
					continue
				}
				fmt.Println("Mensagem recebida: [", aux, "],[", recebimento, "]")
				m := mensagem{IpOrigem: aux[1], IpDestino: aux[2], Conteudo: aux[3], IpAtual: aux[4]}
				fmt.Println("mensagem criada")
				tipo := aux[0]
				if err != nil {
					errorHandler(err, "Erro ao ler mensagem TCP: ", false)
					return
				}
				fmt.Println("mensagem destino: ", m.IpDestino, " ipHost: ", ipHost)
				if m.IpOrigem == ipHost {
					continue
				}
				if m.IpDestino != ipHost {
					if m.IpAtual == ipProximoNo {
						fmt.Println("Mensagem recebida do nó proximo: ", m.Conteudo, " tipo: ", tipo)
						m.enviarMensagemAnt(tipo)
					} else {
						if m.IpAtual == ipNoAnterior {
							fmt.Println("Mensagem recebida do nó anterior: ")
							m.enviarMensagemNext(tipo)
						}
						continue
					}
					//separa de quem veio a mensagem
				} else {

					//TODO: tratar mensagem de novo servidor

					//TODO: ATUALIZAR ip anterior
					fmt.Println("Mensagem recebida de um nó desconhecido: ")
					switch tipo {
					case "NovoServidor":
						//myp := peer.AddrInfo{ID: peer.ID(m.Conteudo), Addrs: []multiaddr.Multiaddr{multiaddr.StringCast(conn.RemoteAddr().String())}}
						myp := HostAnel{IDHost: m.Conteudo, IPHost: conn.RemoteAddr().String()}
						mutexTabelasDeServ.Lock()
						tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores, myp)
						mutexTabelasDeServ.Unlock()
						fmt.Println("Novo servidor registrado: ", m.Conteudo, " ; ", conn.RemoteAddr().String())
						conn.Write([]byte(ipNoAnterior))
						conn.Read(buffer)
						//verificar ack
						if string(buffer[:n]) == "ACK" {
							fmt.Println("ACK recebido")
							ipNoAnterior = conn.RemoteAddr().String()
							byteTabela, _ := json.Marshal(tabelasDeRoteamentoServidores)
							for _, p := range tabelasDeRoteamentoServidores {
								fmt.Println("Enviando tabela para: ", p.IPHost)
								mensagemEnvio := mensagem{IpOrigem: ipHost, IpDestino: p.IPHost, Conteudo: string(byteTabela), IpAtual: ipHost}
								mensagemEnvio.enviarMensagemNext("AtualizarListaServer")

							}
							for _, p := range tabelasDeRoteamento {
								fmt.Println("Enviando tabela para: ", p.IPHost)
								mensagemEnvio := mensagem{IpOrigem: ipHost, IpDestino: p.IPHost, Conteudo: string(byteTabela), IpAtual: ipHost}
								mensagemEnvio.enviarMensagemNext("AtualizarListaServer")
							}

							//mensagemEnvio := mensagem{IpOrigem: ipHost, IpDestino: ipHost, Conteudo: string(byteTabela), IpAtual: ipHost}
							//mensagemEnvio.enviarMensagemNext("AtualizarListaServer")
						}
					case "AtualizarListaServer":

						var tabelaAux []HostAnel  //maquinas recebidas
						var tabelaAux2 []HostAnel //maquinas ainda nao registradas
						err := json.Unmarshal([]byte(m.Conteudo), &tabelaAux)
						if err != nil {
							fmt.Println("Erro ao deserializar roteamento: ", err)
						}

						mutexTabelasDeServ.Lock()
						for _, p := range tabelaAux { //laço para evitar itens duplicados
							existe := false
							for _, p2 := range tabelasDeRoteamentoServidores {

								if p.IDHost == p2.IDHost {
									existe = true

								}

							}
							if !existe {
								tabelaAux2 = append(tabelaAux2, p)
							}
						}
						tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores, tabelaAux2...)
						mutexTabelasDeServ.Unlock()
						fmt.Println("Tabela de servidores atualizada: ", tabelasDeRoteamentoServidores)
					case "AtualizaProximo":
						ipProximoNo = m.Conteudo
						conn.Write([]byte("ACK"))
						fmt.Println("Proximo atualizado: ", ipProximoNo)
					default:

						fmt.Println("mensagem invalida")
					}

				}

			}
		}()
	}
}

func sendMessageNext(mensagem []byte) {
	conn, err := net.Dial("tcp", ipProximoNo)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	//	defer conn.Close()

	_, err = conn.Write(mensagem)
	errorHandler(err, "Erro ao enviar mensagem", true)
}
func sendMessageAnt(mensagem []byte) {
	conn, err := net.Dial("tcp", ipNoAnterior)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	//	defer conn.Close()

	_, err = conn.Write(mensagem)
	errorHandler(err, "Erro ao enviar mensagem", true)
}
func main() {
	ctx := context.Background()

	// definindo a porta do nó mestre
	masterPort := flag.Int("p", 0, "Porta destino")
	ipFile := flag.Int("d", -1, "Porta destino")
	flag.Parse()

	// criando um host que irá escutar em qualquer interface "0.0.0.0" e na porta "masterPort"
	h, err := makeHost(*masterPort, rand.Reader)
	errorHandler(err, "Erro ao criar host: ", true)
	listIp := openFileAndGetIps()
	fmt.Println(listIp)

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	ipHost = h.Addrs()[0].String()
	if *ipFile == -1 {
		for indice, valor := range listIp { //busca o ip da maquina na lista de ips
			ipAtual, _, _ := net.SplitHostPort(valor)
			if strings.Contains(ipHost, ipAtual) {
				if indice == 0 {
					ipNoAnterior = listIp[len(listIp)-1]
					ipProximoNo = listIp[indice+1]
				} else if indice == len(listIp)-1 {
					ipNoAnterior = listIp[indice-1]
					ipProximoNo = listIp[0]
				} else {
					ipNoAnterior = listIp[indice-1]
					ipProximoNo = listIp[indice+1]
				}
				ipHost = valor
				break
			}
		}
	} else {
		indice := *ipFile
		if indice == 0 {
			ipNoAnterior = listIp[len(listIp)-1]
			ipProximoNo = listIp[indice+1]
		} else if indice == len(listIp)-1 {
			ipNoAnterior = listIp[indice-1]
			ipProximoNo = listIp[0]
		} else {
			ipNoAnterior = listIp[indice-1]
			ipProximoNo = listIp[indice+1]
		}
		ipHost = listIp[indice]
	}
	//inicia recepçao de mensagens do anel
	go receiveMessageAnelListening(ipHost)

	// criando um novo pubsub para os supernós se conectarem ao nó mestre (conexao exclusiva entre eles)
	psSuperMaster, err := pubsub.NewGossipSub(context.Background(), h)
	errorHandler(err, "Erro ao criar pubsub: ", true)
	ipHostIp, _, _ := net.SplitHostPort(ipHost)
	// servidor tcp
	tcpListener, err := net.Listen("tcp", ipHostIp+":8080")

	errorHandler(err, "Erro ao criar servidor TCP: ", true)

	defer tcpListener.Close()

	// cria uma chave única privada RSA para os super nós conectarem
	chaveDeConexao := startPeer(h, handleStream)

	// lida com conexoes de outros supernós
	fmt.Println("Aguardando supernós se conectarem...")

	// canais criados para controlar a conexão dos supernós
	ackChan := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão: ", true)

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
	errorHandler(err, "Erro ao converter tabela para bytes: ", true)

	roteamentoTopic, _ := createAndJoinTopic(psSuperMaster, "roteamento")
	broadcastMessage(ctx, roteamentoTopic, byteTabela)

	select {}

}

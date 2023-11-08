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

func listensSubs(ctx context.Context, sub *pubsub.Subscription, topic *pubsub.Topic, ackChan chan<- bool, h host.Host) {
	for {
		msg, err := sub.Next(ctx)

		if err != nil {
			fmt.Println("Error reading message:", err)
			continue
		}

		if topic.String() == "roteamentoSuper" {
			var tabelaAux []peer.AddrInfo
			var tabelaAux2 []peer.AddrInfo
			tabelaRoteamentoAux := tabelasDeRoteamento
			err := json.Unmarshal(msg.Data, &tabelaAux)
			for _, p := range tabelaAux { //laço para evitar itens duplicados
				for _, p2 := range tabelasDeRoteamento {
					existe := false
					if p.ID.String() == p2.IDHost {
						existe = true

					}
					if !existe {
						for _, p3 := range p.Addrs {
							if !strings.Contains(p3.String(), "127.0.0.1") {
								ipOfHost := strings.Split(p3.String(), "/")[2]
								server := HostAnel{IDHost: p.ID.String(), IPHost: ipOfHost}
								tabelaRoteamentoAux = append(tabelaRoteamentoAux, server)
							}
							tabelaAux2 = append(tabelaAux2, p)
						}

					}
				}
			}
			tabelasDeRoteamento = tabelaRoteamentoAux

			if err != nil {
				fmt.Println("Erro ao deserializar roteamento: ", err)
			} else {
				// deu tudo certo, ir para a parte de servidores
				for _, p := range tabelaAux2 {
					if p.ID.String() != h.ID().String() {
						err := h.Connect(ctx, p)
						if err != nil {
							fmt.Println("Erro ao conectar com supernó: ", err)
						}
					}
				}
				ackChan <- true
			}
		} else {
			if topic.String() == "roteamentoServer" {
				var tabelaAux []peer.AddrInfo
				var tabelaAux2 []peer.AddrInfo
				tabelaAuxServers := tabelasDeRoteamentoServidores
				err := json.Unmarshal(msg.Data, &tabelaAux)
				for _, p := range tabelaAux { //laço para evitar itens duplicados
					for _, p2 := range tabelasDeRoteamentoServidores {
						existe := false
						if p.ID.String() == p2.IPHost {
							existe = true

						}
						if !existe {
							for _, p3 := range p.Addrs {
								if !strings.Contains(p3.String(), "127.0.0.1") {
									ipOfHost := strings.Split(p3.String(), "/")[2]
									server := HostAnel{IDHost: p.ID.String(), IPHost: ipOfHost}
									tabelaAuxServers = append(tabelaAuxServers, server)
								}
							}

							tabelaAux2 = append(tabelaAux2, p)
						}
					}
				}
				tabelasDeRoteamentoServidores = tabelaAuxServers

				if err != nil {
					fmt.Println("Erro ao deserializar roteamento: ", err)
				} else {
					// deu tudo certo, ir para a parte de servidores
					for _, p := range tabelaAux2 {
						if p.ID.String() != h.ID().String() {
							err := h.Connect(ctx, p)
							if err != nil {
								fmt.Println("Erro ao conectar com supernó: ", err)
							}
						}
					}
					ackChan <- true
				}
			}
			fmt.Printf("Received message: %s\n", string(msg.Data))
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

func broadcastMessage(ctx context.Context, pubsub *pubsub.Topic, message []byte) error {
	// Publish the message to the topic
	err := pubsub.Publish(ctx, message)

	// Wait for the message to be propagated to all peers
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	return err
}

func MakeHost(port int, rand io.Reader) (h host.Host, err error) {
	// cria uma chave única privada RSA
	privateKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand)
	if err != nil {
		return nil, err
	}

	// criando um multiaddr para ouvir em qualquer ip
	sourcerMulti, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
	if err != nil {
		return nil, err
	}

	return libp2p.New(
		libp2p.ListenAddrs(sourcerMulti),
		libp2p.Identity(privateKey),
	)
}

func tcpHandleConnection(conn net.Conn, ackChan chan<- bool, i int, ctx context.Context, h host.Host) {
	defer conn.Close()
	buffer := make([]byte, 1024)

	n, err := conn.Read(buffer)
	errorHandler(err, "Erro ao ler a chave de identificação:", true)

	/*
		dessa parte em diante deve se analisar as mensagens recebidas, para saber se é uma mensagem de cadastro,
		ou uma mensagem de solicitação de arquivo, ou outra mensagem
	*/

	chaveDeConexao := string(buffer[:n])
	if err != nil {
		fmt.Println("Erro ao enviar a chave de identificação:", err)
		ackChan <- false
		return
	}

	startPeerAndConnect(ctx, h, chaveDeConexao)

	fmt.Println("Chave de identificação enviada para o servidor ", i+1)

	ack := "ACK"
	_, err = conn.Write([]byte(ack))
	if err != nil {
		fmt.Println("Erro ao ler o ACK do cliente: ", i+1, err)
		ackChan <- false
		return
	} else {
		ackChan <- true
	}

}

func startPeerAndConnect(ctx context.Context, h host.Host, destination string) (*bufio.ReadWriter, error) {

	maddr, err := multiaddr.NewMultiaddr(destination)
	errorHandler(err, "Erro ao criar multiaddr: ", true)

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	errorHandler(err, "Erro ao extrair peer ID: ", true)

	h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

	s, err := h.NewStream(context.Background(), info.ID, "/handshake/master")
	errorHandler(err, "Erro ao criar stream: ", true)

	fmt.Println("Conexão estabelecida com o servidor")

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	return rw, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	serversPort := flag.String("p", "0", "Porta para conexão com servidores")
	ipFile := flag.Int("d", -1, "Porta destino")
	//ipHostFlag := flag.String("h", "0", "Porta destino")
	flag.Parse()
	buffer := make([]byte, 1024)

	h, err := MakeHost(0, rand.Reader)
	errorHandler(err, "Erro ao criar host: ", true)
	fmt.Println(h.Addrs())
	listIp := openFileAndGetIps()
	fmt.Println(listIp)

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	ipHost = h.Addrs()[0].String()
	if *ipFile == -1 {

		fmt.Println("informe o indice o arquivo que representa esse host com -d")
		os.Exit(1)

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
	go receiveMessageAnelListening(ipHost)

	// criação do pubsub
	pb, err := pubsub.NewGossipSub(context.Background(), h)
	errorHandler(err, "Erro ao criar pubsub: ", true)

	ackChan := make(chan bool, 1)
	// criação dos topicos
	broadcastTopic, err := createAndJoinTopic(pb, "broadcast")
	errorHandler(err, "Erro ao criar tópico broadcast: ", true)

	roteamentoTopic, err := createAndJoinTopic(pb, "roteamento")
	errorHandler(err, "Erro ao criar tópico roteamento: ", true)

	servidoresTopic, err := createAndJoinTopic(pb, "Servidores")
	errorHandler(err, "Erro ao criar tópico servidores: ", true)

	// inscrição dos topicos
	broadcastSub, err := subscribeToTopic(broadcastTopic)
	errorHandler(err, "Erro ao se inscrever no tópico broadcast: ", true)
	go listensSubs(ctx, broadcastSub, broadcastTopic, nil, h)

	roteamentoSub, err := subscribeToTopic(roteamentoTopic)
	errorHandler(err, "Erro ao se inscrever no tópico roteamento: ", true)
	go listensSubs(ctx, roteamentoSub, roteamentoTopic, ackChan, h)

	//se conecta com o mestre

	ipMestre, _, _ := net.SplitHostPort(listIp[0])
	ipMestre = ipMestre + ":8080"
	conn, err := net.Dial("tcp", ipMestre)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()

	n, err := conn.Read(buffer)
	errorHandler(err, "Erro ao ler a chave de identificação:", true)

	chaveConexaoMestre := string(buffer[:n])

	fmt.Println("Chave de acesso recebida")

	_, err = startPeerAndConnect(ctx, h, chaveConexaoMestre)
	errorHandler(err, "(libp2p)Erro ao conectar ao mestre: ", true)

	ack := "ACK"
	fmt.Println("Enviando ACK para o nó mestre...")
	_, err = conn.Write([]byte(ack))
	errorHandler(err, "Erro ao enviar ACK:", true)

	if <-ackChan {
		go handleServers(serversPort, ctx, h, servidoresTopic, roteamentoTopic)
	}
	// Create a thread to read and write data.
	//go writeData(rw)
	//go readData(rw)

	// Wait forever
	select {}
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
				if m.IpOrigem == ipHost {
					continue
				}
				if m.IpDestino != ipHost {
					if m.IpAtual == ipProximoNo {
						fmt.Println("Mensagem recebida do nó proximo: ", m.Conteudo, " tipo: ", tipo)
						sendMessageAnt(buffer[:n])
					} else {
						if m.IpAtual == ipNoAnterior {
							fmt.Println("Mensagem recebida do nó anterior: ")
							sendMessageNext(buffer[:n])
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
						tabelaAux := tabelasDeRoteamentoServidores
						existe := false
						for _, p := range tabelaAux {
							if p.IDHost == m.Conteudo {
								existe = true
							}
						}
						if !existe {
							tabelasDeRoteamentoServidores = append(tabelasDeRoteamentoServidores, myp)
						} else {
							fmt.Println("Servidor já registrado")
						}

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
					case "AtualizaProximo":
						ipProximoNo = m.Conteudo
						conn.Write([]byte("ACK"))
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
	defer conn.Close()

	_, err = conn.Write([]byte(mensagem))
	errorHandler(err, "Erro ao enviar mensagem", true)
}
func sendMessageAnt(mensagem []byte) {
	conn, err := net.Dial("tcp", ipNoAnterior)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()
	fmt.Println("Mensagem enviada para o nó anterior: [", string(mensagem), "]")

	_, err = conn.Write([]byte(mensagem))

	errorHandler(err, "Erro ao enviar mensagem", true)
}
func handleServers(serversPort *string, ctx context.Context, h host.Host, topicServ *pubsub.Topic, topicRot *pubsub.Topic) bool {
	//super nós podem se conectar a rede p2p
	ackChan := make(chan bool, 2)
	//concatena : com o servers port com

	address := ":" + *serversPort

	tcpListener, err := net.Listen("tcp", address)
	errorHandler(err, "Erro ao criar servidor TCP: ", false)

	defer tcpListener.Close()

	for i := 0; i < 2; i++ {
		fmt.Println("Aguardando servidor ", i+1, " se conectar...")
		conn, err := tcpListener.Accept()
		errorHandler(err, "Erro ao aceitar conexão:", true)

		go tcpHandleConnection(conn, ackChan, i, ctx, h)
	}

	if <-ackChan && <-ackChan {
		fmt.Println("Ambos os servidores se conectaram com sucesso!")
	} else {
		fmt.Println("Erro ao conectar um ou mais nós.")
	}

	fmt.Println("Enviando mensagem de broadcast aos servidores")

	err = broadcastMessage(ctx, topicServ, []byte("ACK"))
	errorHandler(err, "Erro ao enviar mensagem de broadcast: ", true)

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

	err = broadcastMessage(ctx, topicRot, byteTabela)
	errorHandler(err, "Erro ao enviar mensagem de broadcast: ", true)
	return false
}

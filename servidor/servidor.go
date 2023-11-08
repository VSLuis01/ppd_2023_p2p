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
)

type HostAnel struct {
	IDHost string
	IPHost string
}

var tabelasDeRoteamento []HostAnel
var tabelasDeRoteamentoServidores []HostAnel

var tabelasDeRoteamentoSuper []peer.AddrInfo
var tabelasDeRoteamentoClientes []peer.AddrInfo
var mutexTabelasDeSupers = sync.Mutex{}
var mutexTabelasDeServ = sync.Mutex{}
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

				//formata mensagem recebida
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

					case "AtualizaProximo":
						ipProximoNo = m.Conteudo
						conn.Write([]byte("ACK"))
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

	_, err = conn.Write(mensagem)
	errorHandler(err, "Erro ao enviar mensagem", true)
}
func sendMessageAnt(mensagem []byte) {
	conn, err := net.Dial("tcp", ipNoAnterior)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()

	_, err = conn.Write(mensagem)
	errorHandler(err, "Erro ao enviar mensagem", true)
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
			mutexTabelasDeSupers.Lock()
			err := json.Unmarshal(msg.Data, &tabelasDeRoteamentoSuper)
			if err != nil {
				log.Println("Erro ao deserializar roteamento: ", err)
			}
			log.Println("Recebido tabela de roteamento: ", tabelasDeRoteamentoSuper)
			mutexTabelasDeSupers.Unlock()
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
func joinRing(host host.Host, ipSuper string) {
	//conecta solicita conexão super nó
	buffer := make([]byte, 1024)

	conn, err := net.Dial("tcp", ipSuper)
	conn.LocalAddr().String()
	ipHost = conn.LocalAddr().String()
	//utiliza o ip e porta da primeira conexão para a conexão em anel

	defer conn.Close()
	ipProximoNo = ipSuper
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	//envia mensagem de solicitação de conexão
	m := mensagem{IpOrigem: ipHost, IpDestino: ipSuper, Conteudo: host.ID().String(), IpAtual: ipHost}
	//m.enviarMensagemNext("NovoServidor")
	conn.Write(([]byte("NovoServidor" + "#" + m.IpOrigem + "#" + m.IpDestino + "#" + m.Conteudo + "#" + ipHost)))
	conn.Read(buffer)
	fmt.Println("Mensagem recebida, o ip do anterior é: ", string(buffer))

	ipNoAnterior = string(buffer)
	ipNoAnterior = strings.TrimSpace(ipNoAnterior)
	fmt.Println("guardo anterior [", ipNoAnterior, "]")
	//enviar  ack
	conn.Write([]byte("ACK"))
	conn.Close()
	fmt.Println("passou do ack")
	//enviar mensagem para o anterior atualizar o proximo ip
	conn2, err := net.Dial("tcp", ipNoAnterior)

	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
	}
	errorHandler(err, "Erro ao conectar ao servidor:", true)
	m = mensagem{IpOrigem: ipHost, IpDestino: ipNoAnterior, Conteudo: ipHost, IpAtual: ipHost}
	conn2.Write(([]byte("AtualizaProximo" + "#" + m.IpOrigem + "#" + m.IpDestino + "#" + m.Conteudo + "#" + ipHost)))
	conn2.Read(buffer)
	if string(buffer) != "ACK" {
		fmt.Println("Erro ao atualizar anterior")
	} else {
		fmt.Println("anterior atualizado com sucesso")
	}
}
func main() {
	ctx := context.Background()
	ipFile := flag.Int("d", -1, "Porta destino")
	ipConect := flag.String("c", "", "<ip>:<porta> de um superno. ")

	flag.Parse()
	//definir ip e porta do servidor

	h, err := MakeHost(0, rand.Reader)
	errorHandler(err, "Erro ao criar host: ", true)
	listIp := openFileAndGetIps()
	fmt.Println(listIp)

	///baseado no arquivo, encontra o ipAtual e define proximo e anterior
	ipHost := h.Addrs()[0].String()
	if *ipConect == "" { //nesse caso o servidor é da configuração inicial
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
			fmt.Println("Indice: ", indice)
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
		//por padrao o endereço do superno de configuração inicial é o segundo elemento da lista de ips
		ipHostIp, _, _ := net.SplitHostPort(listIp[1])
		conn, err := net.Dial("tcp", ipHostIp+":8080")
		defer conn.Close()
		errorHandler(err, "Erro ao conectar ao servidor:", true)

		fmt.Println("Conexão TCP estabelecida com sucesso")

		chaveDeConexao := startPeer(h, handleStream)

		tcpHandleConnection(conn, chaveDeConexao, nil, 0)
		errorHandler(err, "Erro ao ler a chave de identificação:", true)
	} else {
		//nesse caso o servidor se conecta a rede em anel ja configurada

		joinRing(h, *ipConect)
	}

	go receiveMessageAnelListening(ipHost)

	// Create a thread to read and write data.
	//go writeData(rw)
	//go readData(rw)

	// Wait forever
	select {}
}

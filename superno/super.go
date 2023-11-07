package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

var tabelasDeRoteamento []net.Conn
var ipNextNode string
var ipPrevNode string
var ipHost string

/*type mensagem struct {
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
}*/

func openFileAndGetIps(filename string) []string {
	file, err := os.Open(filename)
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

func tcpHandleIncomingMessages(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		errorHandler(err, "Erro ao fechar conexão TCP: ", true)
	}(conn)

	buffer := make([]byte, 1024)

	for {
		msg, err := conn.Read(buffer)
		errorHandler(err, "Erro ao ler mensagem TCP: ", false)

		fmt.Println(msg)
		// verifica o tipo de mengagem recebida
	}

}

var characterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characterRunes[rand.Intn(len(characterRunes))]
	}
	return string(b)
}

func newHashSha1(n ...int) string {
	noRandomCharacters := 32

	if len(n) > 0 {
		noRandomCharacters = n[0]
	}

	randString := RandomString(noRandomCharacters)

	hash := sha1.New()
	hash.Write([]byte(randString))
	bs := hash.Sum(nil)

	return fmt.Sprintf("%x", bs)
}

func getNextAndPrevAndHostManual(ipList []string, ipIndexFile int) (next string, prev string, host string) {
	if ipIndexFile == 0 {
		prev = ipList[len(ipList)-1]
		next = ipList[ipIndexFile+1]
	} else if ipIndexFile == len(ipList)-1 {
		prev = ipList[ipIndexFile-1]
		next = ipList[0]
	} else {
		prev = ipList[ipIndexFile-1]
		next = ipList[ipIndexFile+1]
	}
	host = ipList[ipIndexFile]

	return
}

func getNextAndPrevAuto(ipList []string, host string) (string, string, string) {
	var next string
	var prev string
	for i, valor := range ipList { //busca o ip da maquina na lista de ips
		ipAtual, _, _ := net.SplitHostPort(valor)
		if strings.Contains(host, ipAtual) {
			if i == 0 {
				prev = ipList[len(ipList)-1]
				next = ipList[i+1]
			} else if i == len(ipList)-1 {
				prev = ipList[i-1]
				next = ipList[0]
			} else {
				prev = ipList[i-1]
				next = ipList[i+1]
			}
			host = valor
			break
		}
	}
	return next, prev, host
}

func getIpHost() string {
	addrs, err := net.InterfaceAddrs()
	errorHandler(err, "Erro ao obter endereços da interface: ", true)

	for _, address := range addrs {
		ipNet, ok := address.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
}

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {
	_, cancel := context.WithCancel(context.Background())

	defer cancel()

	ipIndexFile := flag.Int("fi", -1, "Porta destino")
	flag.Parse()

	//buffer := make([]byte, 1024)

	listIp := openFileAndGetIps("../ips")

	///baseado no arquivo, encontra o ipatual e define proximo e anterior
	// pega o ip da máquina sem a porta
	ipHost = getIpHost()

	fmt.Println(ipHost)

	if *ipIndexFile == -1 {
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAuto(listIp, ipHost)
	} else {
		// atribui a porta
		ipNextNode, ipPrevNode, ipHost = getNextAndPrevAndHostManual(listIp, *ipIndexFile)
	}

	fmt.Println("ipNextNo: ", ipNextNode)
	fmt.Println("ipPrevNo: ", ipPrevNode)
	fmt.Println("ipHost: ", ipHost)

	/*go receiveMessageAnelListening(ipHost)
	var m mensagem
	m.IpDestino = ipProximoNo
	m.IpOrigem = ipHost
	m.Conteudo = "teste"
	m.enviarMensagemAnt("teste")*/

	ackChan := make(chan bool, 1)

	//se conecta com o mestre

	ipMestre := listIp[0]

	tcpAddrMestre, _ := net.ResolveTCPAddr("tcp", ipMestre)
	tcpAddrHost, _ := net.ResolveTCPAddr("tcp", ipHost)

	// conecta ao mestre
	mestreConn, err := net.DialTCP("tcp", tcpAddrHost, tcpAddrMestre)
	errorHandler(err, "Erro ao conectar ao mestre:", true)
	fmt.Println("Conexão TCP estabelecida com sucesso")

	// ver o que fazer com isso
	chaveMestre := make([]byte, 1024)

	// recebe a chave de identificação do no mestre
	lenMsg, err := mestreConn.Read(chaveMestre)
	errorHandler(err, "Erro ao receber chave do mestre:", false)

	chaveMestre = chaveMestre[:lenMsg]
	fmt.Println("Chave recebida do mestre: ", string(chaveMestre))

	//go tcpHandleIncomingMessages(mestreConn)

	ack := "ACK"
	fmt.Println("Enviando ACK para o nó mestre...")
	_, err = mestreConn.Write([]byte(ack))
	errorHandler(err, "Erro ao enviar ACK:", true)

	if <-ackChan {
		//go handleServers(serversPort, ctx, h, servidoresTopic, roteamentoTopic)
	}
	// Create a thread to read and write data.
	//go writeData(rw)
	//go readData(rw)

	// Wait forever
	select {}
}

/*func receiveMessageAnelListening(adress string) {
	tcpListener, err := net.Listen("tcp", adress)
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
				//separa de quem veio a mensagem
				if m.IpAtual == ipProximoNo {
					fmt.Println("Mensagem recebida do nó proximo: ", m.Conteudo, " tipo: ", tipo)
					sendMessageNext(buffer[:n])
				} else {
					fmt.Println("Mensagem recebida do nó anterior: ")
				}

			}
		}()
	}
}*/

func sendMessageNext(mensagem []byte) {
	conn, err := net.Dial("tcp", ipNextNode)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()

	_, err = conn.Write([]byte(mensagem))
	errorHandler(err, "Erro ao enviar mensagem", true)
}

func sendMessageAnt(mensagem []byte) {
	conn, err := net.Dial("tcp", ipPrevNode)
	errorHandler(err, "Erro ao conectar ao servidor:", true)

	fmt.Println("Conexão TCP estabelecida com sucesso")
	defer conn.Close()
	fmt.Println("Mensagem enviada para o nó anterior: [", string(mensagem), "]")

	_, err = conn.Write([]byte(mensagem))

	errorHandler(err, "Erro ao enviar mensagem", true)
}

/*func handleServers(serversPort *string, ctx context.Context, h host.Host, topicServ *pubsub.Topic, topicRot *pubsub.Topic) bool {
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

		go tcpHandleIncomingMessages(conn, ackChan, i, ctx, h)
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
}*/

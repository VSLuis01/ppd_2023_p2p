package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type HostAnel struct {
	IDHost string
	IPHost string
}

type Arquivos struct {
	NomeArquivo string
	Cliente     HostAnel
}

var tabelaArquivos []Arquivos

var tabelaArquivosOutraRede []Arquivos

var privateKey string

var ipHost string

var ipSuperNo string

var connSuperNo net.Conn

type Mensagem struct {
	Tipo       string
	IpOrigem   string
	IpDestino  string
	Conteudo   []byte
	IpAtual    string
	JumpsCount int
}

func splitMensagem(mensagem string) (*Mensagem, error) {
	mensagemSplit := strings.Split(mensagem, "#")

	if len(mensagemSplit) == 6 {
		jumps, _ := strconv.Atoi(mensagemSplit[5])
		return &Mensagem{mensagemSplit[0], mensagemSplit[1], mensagemSplit[2], []byte(mensagemSplit[3]), mensagemSplit[4], jumps}, nil
	} else {
		return nil, fmt.Errorf("mensagem inválida")
	}

}

func newMensagem(tipo string, IpOrigem string, IpDestino string, conteudo []byte, IpHost string, jumpsCount int) *Mensagem {
	return &Mensagem{tipo, IpOrigem, IpDestino, conteudo, IpHost, jumpsCount}
}

func newAck(ipDestino string) *Mensagem {
	return &Mensagem{"ack", ipHost, ipDestino, []byte(""), ipHost, 0}
}

func (m *Mensagem) toString() string {
	return fmt.Sprintf("%s#%s#%s#%s#%s#%d", m.Tipo, m.IpOrigem, m.IpDestino, m.Conteudo, m.IpAtual, m.JumpsCount)
}

func (m *Mensagem) toBytes() []byte {
	return []byte(m.toString())
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

func ordenarArquivosPorIP() {
	sort.SliceStable(tabelaArquivos, func(i, j int) bool {
		return tabelaArquivos[i].Cliente.IDHost < tabelaArquivos[j].Cliente.IDHost
	})
}

func removeElemento(nomeArquivo, idHost string) bool {
	for i, arquivo := range tabelaArquivos {
		if arquivo.NomeArquivo == nomeArquivo && arquivo.Cliente.IDHost == idHost {
			tabelaArquivos = append(tabelaArquivos[:i], tabelaArquivos[i+1:]...)
			return true
		}
	}
	return false
}

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

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	cmd := exec.Command("clear") //Linux example, its tested
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func findSuperNo(someIp string) net.Conn {
	conn, err := net.Dial("tcp", someIp)
	errorHandler(err, "Erro ao conectar com algum nó do anel: ", true)

	conn.Write(newMensagem("FindSuper", ipHost, "", []byte(""), ipHost, 0).toBytes())

	listener, err := net.Listen("tcp", ipHost)
	defer listener.Close()
	errorHandler(err, "Erro ao abrir porta para receber resposta do supernó: ", true)

	connSuper, err := listener.Accept()
	errorHandler(err, "Erro ao receber resposta do supernó: ", true)

	return connSuper
}

func getIpHost() (string, error) {
	addrs, err := net.InterfaceAddrs()
	errorHandler(err, "Erro ao obter endereços da interface: ", true)

	for _, address := range addrs {
		ipNet, ok := address.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("não foi possível obter o endereço IP da máquina")
}

func main() {
	portaServidorArquivos := flag.String("p", "7070", "Porta do servidor de arquivos")
	ipFile := flag.String("f", "ips", "Arquivo de ips")
	flag.Parse()

	listIp := openFileAndGetIps("../" + *ipFile)

	ipHost, _ = getIpHost()

	ipHost = ipHost + ":" + *portaServidorArquivos

	connSuperNo = findSuperNo(listIp[0])

	connSuperNo.Write(newAck("").toBytes())

	privateKey = newHashSha1()

	// configuração inicial com o supernó
	handleTcpMessages(connSuperNo)
}

func handleTcpMessages(conn net.Conn) {
	defer conn.Close()

	for {
		buffer := make([]byte, 4000)

		msgLen, err := conn.Read(buffer)

		if err == io.EOF {
			fmt.Printf("[%s] A conexão foi fechada pelo nó.\n", conn.RemoteAddr().String())
			return
		}

		mensagem := make([]byte, msgLen)
		copy(mensagem, buffer[:msgLen]) // jeito mais seguro de copiar o buffer

		msg, err := splitMensagem(string(mensagem))
		if err != nil {
			errorHandler(err, "Erro ao converter mensagem: ", false)
			continue
		}

		switch msg.Tipo {
		case "ack":
			ipSuperNo = string(msg.Conteudo)
			fmt.Println("Conectado com o supernó: ", ipSuperNo)
		case "uploadFile":
			arquivo := strings.Split(string(msg.Conteudo), "/")

			hostAnel := HostAnel{arquivo[0], msg.IpOrigem}

			tabelaArquivos = append(tabelaArquivos, Arquivos{arquivo[1], hostAnel})

			ordenarArquivosPorIP()

			conn.Write(newAck(msg.IpOrigem).toBytes())

			for _, tabelaArquivo := range tabelaArquivos {
				fmt.Printf("%s - %s: %s\n", tabelaArquivo.Cliente.IDHost, tabelaArquivo.Cliente.IPHost, tabelaArquivo.NomeArquivo)
			}
		case "downloadFile":
			// envia o ip de quem possui o arquivo
			var peersFiles []string
			var peersFilesOutraRede []string

			for _, tabelaArquivo := range tabelaArquivos {
				if tabelaArquivo.NomeArquivo == string(msg.Conteudo) {
					peersFiles = append(peersFiles, tabelaArquivo.Cliente.IDHost+"-"+tabelaArquivo.Cliente.IPHost)
				}
			}

			var nMsg *Mensagem

			if len(peersFiles) == 0 {
				// procura em outra rede
				for _, tabelaArquivo := range tabelaArquivosOutraRede {
					if tabelaArquivo.NomeArquivo == string(msg.Conteudo) {
						peersFilesOutraRede = append(peersFilesOutraRede, tabelaArquivo.Cliente.IDHost+"-"+tabelaArquivo.Cliente.IPHost)
					}
				}

				if len(peersFilesOutraRede) == 0 {
					nMsg = newMensagem("findFileAnotherRing", ipHost, connSuperNo.RemoteAddr().String(), msg.Conteudo, ipHost, 0)

					conn.Write(nMsg.toBytes())

					time.Sleep(500 * time.Millisecond)

					buf := make([]byte, 1024)

					msgLen, _ = conn.Read(buf)

					buf = buf[:msgLen]

					rcvMsg, err := splitMensagem(string(buf))
					errorHandler(err, "", false)

					if rcvMsg.Tipo == "UniquePeer" {
						idPeerAndIp := strings.Split(string(rcvMsg.Conteudo), "-")

						tabelaArquivosOutraRede = append(tabelaArquivosOutraRede, Arquivos{string(msg.Conteudo), HostAnel{idPeerAndIp[0], idPeerAndIp[1]}})

						m := newMensagem("AnotherNetworkPeer", ipHost, rcvMsg.IpOrigem, rcvMsg.Conteudo, ipHost, rcvMsg.JumpsCount+1)

						fmt.Println(m.toString())

						conn.Write(m.toBytes())
					} else if rcvMsg.Tipo == "MultiplePeers" {
						idPeersAndIps := strings.Split(string(rcvMsg.Conteudo), "/")

						for _, idPeerAndIp := range idPeersAndIps {
							idPeerAndIpSplit := strings.Split(idPeerAndIp, "-")
							tabelaArquivosOutraRede = append(tabelaArquivosOutraRede, Arquivos{string(msg.Conteudo), HostAnel{idPeerAndIpSplit[0], idPeerAndIpSplit[1]}})
						}

						m := newMensagem("AnotherNetworkPeer", ipHost, rcvMsg.IpOrigem, rcvMsg.Conteudo, ipHost, rcvMsg.JumpsCount+1)

						conn.Write(m.toBytes())
					} else {
						nMsg = newMensagem("NoSuchFile", ipHost, msg.IpOrigem, []byte("Arquivo não encontrado em outra rede anel"), ipHost, 0)

						conn.Write(nMsg.toBytes())
					}
				} else {
					fmt.Println("Arquivo encontrado nos arquivos locais que pertence a outra rede: ")

					nMsg = newMensagem("AnotherNetworkPeer", ipHost, msg.IpOrigem, []byte(strings.Join(peersFilesOutraRede, "/")), ipHost, 0)

					conn.Write(nMsg.toBytes())
				}

			} else if len(peersFiles) == 1 {
				nMsg = newMensagem("UniquePeer", ipHost, msg.IpOrigem, []byte(peersFiles[0]), ipHost, 0)

				conn.Write(nMsg.toBytes())

			} else {
				bytesPeers := []byte(strings.Join(peersFiles, "/"))

				nMsg = newMensagem("MultiplePeers", ipHost, msg.IpOrigem, bytesPeers, ipHost, 0)

				conn.Write(nMsg.toBytes())
			}

		case "listFiles":
			bytesTabelaArquivos, _ := json.Marshal(tabelaArquivos)

			var nMsg *Mensagem

			if len(tabelaArquivos) == 0 {
				nMsg = newMensagem("NoFiles", ipHost, msg.IpOrigem, []byte("Nenhum arquivo encontrado"), ipHost, 0)
			} else {
				nMsg = newMensagem("ack", ipHost, msg.IpOrigem, bytesTabelaArquivos, ipHost, 0)
			}

			conn.Write(nMsg.toBytes())

		case "removeFile":
			fmt.Println("Buscando arquivo: ", string(msg.Conteudo))

			idAndFileName := strings.Split(string(msg.Conteudo), "/")

			isRemoved := removeElemento(idAndFileName[1], idAndFileName[0])

			var nMsg *Mensagem

			if isRemoved {
				nMsg = newMensagem("ack", ipHost, msg.IpOrigem, []byte("Arquivo removido com sucesso"), ipHost, 0)
			} else {
				nMsg = newMensagem("NoFiles", ipHost, msg.IpOrigem, []byte("Nenhum arquivo encontrado"), ipHost, 0)
			}

			conn.Write(nMsg.toBytes())

		}

	}

}

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
)

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

func main() {
	_, cancel := context.WithCancel(context.Background())

	defer cancel()

	_, err := MakeHost(0, rand.Reader)
	if err != nil {
		log.Println("Erro ao criar host client side: ", err)
		return
	}

}

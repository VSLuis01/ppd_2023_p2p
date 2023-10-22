package main

import (
	"context"
	"crypto/rand"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"log"
)

func subscribeToTopic(ctx context.Context, pubsub *pubsub.PubSub, topic string) (*pubsub.Subscription, error) {
	// Subscribe to the topic
	mytopc, err := pubsub.Join(topic)

	//sub, err := pubsub.Subscribe(topic)
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
			log.Printf("Received message: %s\n", string(msg.Data))
		}
	}()

	return sub, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()
	//buffer := make([]byte, 1024)

	h, err := makeHost(0, rand.Reader)

	if err != nil {
		log.Println("Erro ao criar host: ", err)
		return
	}

	pb, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		log.Println("Erro ao criar pubsub: ", err)
		return
	}

	accessTopic, err := pb.Join("access-key")
	if err != nil {
		log.Println("Erro ao criar tópico: ", err)
		return
	}

	subAccess, err := accessTopic.Subscribe()
	if err != nil {
		log.Println("Erro ao criar subscricao: ", err)
		return
	}

	msg, err := subAccess.Next(ctx)
	if err != nil {
		log.Println("Erro ao ler chave de acesso para o nó mestre: ", err)
		return
	}

	fmt.Println("Chave de acesso recebida: ", string(msg.Data))
}

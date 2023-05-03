package main

import (
	"context"
	"flag"
	"log"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"

	R "github.com/BishiNET/ss-server/rpcinterface"
	redis "github.com/redis/go-redis/v9"
)

func getDial(nameFunc string, args interface{}, reply interface{}) error {
	client, err := rpc.DialHTTP("tcp", "127.0.0.1:9990")
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Call(nameFunc, args, reply); err != nil {
		return err
	}
	return nil
}

func run(r1 *redis.Client, listen string) {
	wg := NewWGServer(r1)
	NewRPC(listen, wg)
	wg.StartAll()
	defer wg.StopAll()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func newall(r1 *redis.Client) {
	ctx := context.Background()
	var err error
	userSlices, err := r1.Keys(ctx, "*").Result()
	if err != nil {
		log.Println(err)
		return
	}
	var creply R.CallReply
	args := &R.NewUserArgs{}
	for _, name := range userSlices {
		args.Name = name
		args.Port, _ = r1.HGet(ctx, name, "port").Result()
		if err := getDial("UserRpc.AddUser", args, &creply); err != nil {
			log.Println(err)
		}
		log.Println(creply.ErrReason)
	}
}

func gettraffic(r1 *redis.Client) {
	var treply R.TrafficReply
	if err := getDial("UserRpc.GetUser", &R.CommonArgs{}, &treply); err != nil {
		return
	}
	for name, info := range treply {
		log.Println(name, info.Traffic)
	}
}

func resetall(r1 *redis.Client) {
	var creply R.CallReply
	args := &R.CommonArgs{}
	if err := getDial("UserRpc.ResetAll", args, &creply); err != nil {
		log.Println(err)
	}
}

func main() {
	var mode string
	var listen string
	flag.StringVar(&mode, "mode", "", "mode")
	flag.StringVar(&listen, "listen", "127.0.0.1:9990", "rpc listen")
	flag.Parse()
	r1 := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	})
	defer r1.Close()
	switch mode {
	case "run":
		run(r1, listen)
	case "resetall":
		resetall(r1)
	case "newall":
		newall(r1)
	case "gettraffic":
		gettraffic(r1)
	default:
		log.Fatal("unknown mode")
	}
}

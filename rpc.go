package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"

	R "github.com/BishiNET/ss-server/rpcinterface"
)

type UserRpc struct {
	wg *WGServer
}

func NewRPC(listen string, wg *WGServer) *UserRpc {
	r := &UserRpc{wg}
	rpc.Register(r)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", listen)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
	return r
}

func (r *UserRpc) AddUser(args *R.NewUserArgs, reply *R.CallReply) error {
	port, _ := strconv.Atoi(args.Port)
	uid := port % 100
	ok, key := r.wg.NewUser(uid, args.Name)
	if !ok {
		reply = &R.CallReply{
			ErrCode:   1,
			ErrReason: "cannot add wireguard user",
		}
		return fmt.Errorf("cannot add wireguard user")
	}

	reply = &R.CallReply{
		ErrCode:   0,
		ErrReason: key.String(),
	}
	return nil
}

func (r *UserRpc) StartUser(args *R.CommonArgs, reply *R.CallReply) error {
	r.wg.StartUser(args.Name)
	reply = &R.CallReply{}
	return nil
}
func (r *UserRpc) StopUser(args *R.CommonArgs, reply *R.CallReply) error {
	r.wg.StopUser(args.Name)
	reply = &R.CallReply{}
	return nil
}
func (r *UserRpc) DeleteUser(args *R.CommonArgs, reply *R.CallReply) error {
	r.wg.DeleteUser(args.Name)
	reply = &R.CallReply{}
	return nil
}

func (r *UserRpc) GetUser(args *R.CommonArgs, reply *R.TrafficReply) error {
	if args.Name != "" {
		ut := R.TrafficReply{}
		ut[args.Name] = R.SingleTrafficReply{
			Traffic: GetUser(args.Name),
		}
		*reply = ut
		return nil
	}
	ut := r.wg.GetAllUser()
	*reply = ut
	return nil
}

func (r *UserRpc) ResetAll(args *R.CommonArgs, reply *R.CallReply) error {
	r.wg.ResetAll()
	reply = &R.CallReply{}
	return nil
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	R "github.com/BishiNET/ss-server/rpcinterface"
	redis "github.com/redis/go-redis/v9"
)

type WGServer struct {
	db *redis.Client
}

type KeyPair struct {
	Name    string `json:"name"`
	Public  string `json:"public"`
	Private string `json:"private"`
}

var (
	bg = context.Background()
)

const (
	NEW_SERVER = `
	[Interface]
	Address = 172.16.0.%d/32
	PrivateKey = %s
	ListenPort = %d
	MTU = 1420
	`
	URL    = ""
	apiURL = ""
)

func getUserConfigPath(name string) string {
	return fmt.Sprintf("/etc/wireguard/%s.conf", name)
}

func getKey(name string) *KeyPair {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+name, nil)
	if err != nil {
		log.Println("Get Forwarder: ", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Get Forwarder: ", err)
		return nil
	}
	defer resp.Body.Close()
	var ret KeyPair
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Get Forwarder: ", err)
		return nil
	}
	json.Unmarshal(body, &ret)
	if ret.Public == "" {
		return nil
	}
	return &ret
}

func NewKeyPair(name, pub, pri string) *KeyPair {
	return &KeyPair{
		Name:    name,
		Public:  strings.ReplaceAll(pub, "\n", ""),
		Private: strings.ReplaceAll(pri, "\n", ""),
	}
}

func (kp *KeyPair) String() string {
	return kp.Public + "," + kp.Private
}

func (kp *KeyPair) JSON() []byte {
	b, _ := json.Marshal(kp)
	return b
}
func (kp *KeyPair) Upload() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", URL, bytes.NewBuffer(kp.JSON()))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		log.Println("Upload Stats: ", err)
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Upload Stats: ", err)
		return err
	}
	resp.Body.Close()
	return nil
}

func shell(cmd string) {
	exec.Command("bash", "-c", cmd).Run()
}

func generateKeyPair(pub, pri string) {
	cmd := fmt.Sprintf(`wg genkey |  tee %s | wg pubkey > %s`, pri, pub)
	shell(cmd)
}

func NewWGServer(r1 *redis.Client) *WGServer {
	w := &WGServer{
		db: r1,
	}
	w.serverUP()
	return w
}

func (wg *WGServer) checkUserKeyPair(name string) bool {
	return wg.db.HExists(bg, name, "publickey").Val()
}

func (wg *WGServer) serverUP() {
	cmd := `wg-quick up server`
	shell(cmd)
}
func (wg *WGServer) NewUser(userid int, name string) (ok bool, p *KeyPair) {
	if wg.checkUserKeyPair(name) {
		return
	}
	kp := getKey(name)
	if kp == nil {
		pub, _ := os.CreateTemp("/tmp", "*.key")
		pri, _ := os.CreateTemp("/tmp", "*.key")
		defer os.Remove(pub.Name())
		defer os.Remove(pri.Name())
		defer pub.Close()
		defer pri.Close()
		generateKeyPair(pub.Name(), pri.Name())
		public, err := io.ReadAll(pub)
		if err != nil || len(public) == 0 {
			log.Println("cannot generate publickey: ", err)
			return
		}
		private, err := io.ReadAll(pri)
		if err != nil || len(private) == 0 {
			log.Println("cannot generate privatekey: ", err)
			return
		}

		kp = NewKeyPair(name, string(public), string(private))
		if err := kp.Upload(); err != nil {
			return
		}
	}
	ip := fmt.Sprintf("172.16.0.%d", userid)
	wg.db.HSet(bg, name, "publickey", kp.Public)
	wg.db.HSet(bg, name, "privatekey", kp.Private)
	wg.db.HSet(bg, name, "ip", ip)
	params := map[string]string{
		"ip":         ip,
		"publickey":  kp.Public,
		"privatekey": kp.Private,
	}
	wg.startUser(params)
	ok = true
	p = kp
	return
}

func (wg *WGServer) startUser(params map[string]string) {
	cmd := fmt.Sprintf(`wg set server peer %s allowed-ips %s persistent-keepalive 30`,
		params["publickey"],
		params["ip"])
	shell(cmd)
	cmd = fmt.Sprintf(`ip route add %s/32 dev server`, params["ip"])
	shell(cmd)
}

func (wg *WGServer) StartUser(name string) {
	user, err := wg.db.HGetAll(bg, name).Result()
	if err != nil {
		return
	}
	wg.startUser(user)
}

func (wg *WGServer) StopUser(name string) {
	user, err := wg.db.HGetAll(bg, name).Result()
	if err != nil {
		return
	}
	wg.stopUser(name, user)
}

func (wg *WGServer) DeleteUser(name string) {
	user, err := wg.db.HGetAll(bg, name).Result()
	if err != nil {
		return
	}
	wg.deleteUser(name, user)
}

func (wg *WGServer) stopUser(name string, params map[string]string) {
	cmd := fmt.Sprintf(`wg set server peer %s remove`,
		params["publickey"])
	shell(cmd)
	cmd = fmt.Sprintf(`ip route del %s/32 dev server`, params["ip"])
	shell(cmd)
}

func (wg *WGServer) deleteUser(name string, params map[string]string) {
	wg.stopUser(name, params)
}

func (wg *WGServer) forEachUser(each func(string, map[string]string)) {
	userSlices, err := wg.db.Keys(bg, "*").Result()
	if err != nil {
		log.Println(err)
		return
	}
	for _, name := range userSlices {
		user, _ := wg.db.HGetAll(bg, name).Result()
		each(name, user)
	}
}

func (wg *WGServer) StartAll() {
	wg.forEachUser(func(name string, user map[string]string) {
		if _, ok := user["ip"]; ok {
			wg.startUser(user)
			log.Println("Start " + name)
		}
	})
}

func (wg *WGServer) StopAll() {
	cmd := `wg-quick down server`
	shell(cmd)
}

func (wg *WGServer) GetAllUser() R.TrafficReply {
	m := R.TrafficReply{}
	all := GetAll()
	wg.forEachUser(func(name string, user map[string]string) {
		m[name] = R.SingleTrafficReply{
			Traffic: all[user["ip"]],
		}
	})
	return m
}

func (wg *WGServer) ResetAll() {
	wg.forEachUser(func(name string, user map[string]string) {
		wg.stopUser(name, user)
		wg.db.HSet(bg, name, "wg-traffic", 0)
		wg.startUser(user)
	})
}

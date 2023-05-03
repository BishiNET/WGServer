package main

/*
#cgo LDFLAGS: -Wl,--allow-multiple-definition
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "containers.h"
#include "ipc.h"

extern void callback(char *allow_ip, uint64_t used);
uint64_t get_one(char *username)
{
	uint64_t used = 0;
	struct wgdevice *device = NULL;
	if (ipc_get_device(&device, (const char*) username) < 0) {
		goto exit;
	}
	used = device->first_peer->rx_bytes + device->first_peer->tx_bytes;
exit:
	free_wgdevice(device);

	return used;
}
static char *get_ip(const struct wgallowedip *ip)
{
	static char buf[INET6_ADDRSTRLEN + 1];

	memset(buf, 0, INET6_ADDRSTRLEN + 1);
	if (ip->family == AF_INET)
		inet_ntop(AF_INET, &ip->ip4, buf, INET6_ADDRSTRLEN);
	else if (ip->family == AF_INET6)
		inet_ntop(AF_INET6, &ip->ip6, buf, INET6_ADDRSTRLEN);
	return buf;
}

int get_all()
{
	struct wgdevice *device = NULL;
	struct wgpeer *peer;
	if (ipc_get_device(&device, "server") < 0) {
		return -1;
	}
	uint64_t used = 0;
	for_each_wgpeer(device, peer) {
		if (peer->first_allowedip){
			used = peer->rx_bytes + peer->tx_bytes;
			callback(get_ip(peer->first_allowedip), used);
		}
	}
	free_wgdevice(device);
	return 0;
}


*/
import "C"
import (
	"sync"
	"unsafe"
)

var (
	m     map[string]uint64
	mapMu sync.Mutex
)

//export callback
func callback(allow_ip *C.char, used C.uint64_t) {
	ip := C.GoString(allow_ip)
	m[ip] = uint64(used)
}

func GetUser(name string) uint64 {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return uint64(C.get_one(cname))
}

func GetAll() map[string]uint64 {
	mapMu.Lock()
	defer mapMu.Unlock()
	m = map[string]uint64{}
	C.get_all()
	new := make(map[string]uint64, len(m))
	for k, v := range m {
		new[k] = v
	}
	return new
}
func IsUP(name string) bool {
	return GetUser(name) > 0
}

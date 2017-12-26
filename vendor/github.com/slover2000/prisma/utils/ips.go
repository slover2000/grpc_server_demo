package utils

import (
	"net"
	"errors"
	"strings"
	"strconv"
)

// LocalIPs return all non-loopback IP addresses
func LocalIPs() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {			
			ips = append(ips, ipnet.IP.String())
		}
	}

	return ips, nil
}

// IPToInt32 convert a ip like "128.1.1.1" to int32
func IPToInt32(ip string) (int32, error) {
	bits := strings.Split(ip, ".")
	if len(bits) != 4 {
		return 0, errors.New("not a ip address")
	}

	var intIP int
	for i, v := range bits {
		k, err := strconv.Atoi(v)
		if err != nil || k > 255 {
			return 0, errors.New("invalid ip address")
		}

		intIP = intIP | k<<uint(8*(3-i))
	}

	return int32(intIP), nil
}